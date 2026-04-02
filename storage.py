import re
import markdown as md_lib
from pathlib import Path
from fastapi import HTTPException

ALLOWED_EXTENSIONS = {".md", ".pdf"}
VALID_CATEGORY_RE = re.compile(r"^[\w\u4e00-\u9fff\-]+$")


class CategoryExistsError(Exception):
    pass


class InvalidNameError(Exception):
    pass


class PathTraversalError(Exception):
    pass


def safe_resolve(data_dir: Path, category: str, filename: str) -> Path:
    """解析路径并验证不逃逸 data_dir / category。"""
    cat_dir = (data_dir / category).resolve()
    target = (data_dir / category / filename).resolve()
    try:
        target.relative_to(cat_dir)
    except ValueError:
        raise PathTraversalError(f"Path traversal detected: {filename}")
    return target


def _sanitize_filename(filename: str) -> str:
    """去掉目录分隔符，只保留最终文件名部分。"""
    return Path(filename).name


def list_categories(data_dir: Path) -> list[dict]:
    """返回 data_dir 下所有子目录，附带 .md/.pdf 文件数量。"""
    result = []
    for item in sorted(data_dir.iterdir()):
        if item.is_dir():
            count = sum(
                1 for f in item.iterdir()
                if f.suffix in ALLOWED_EXTENSIONS
            )
            result.append({"name": item.name, "count": count})
    return result


def list_files(data_dir: Path, category: str) -> list[dict]:
    """返回指定分类下的 .md/.pdf 文件列表。"""
    if not VALID_CATEGORY_RE.match(category):
        raise HTTPException(status_code=400, detail=f"Invalid category name: {category!r}")
    cat_path = data_dir / category
    if not cat_path.is_dir():
        raise HTTPException(status_code=404, detail=f"Category '{category}' not found")
    result = []
    for f in sorted(cat_path.iterdir()):
        if f.suffix in ALLOWED_EXTENSIONS:
            stat = f.stat()
            result.append({
                "name": f.name,
                "suffix": f.suffix,
                "mtime": stat.st_mtime,
            })
    return result


def read_file(data_dir: Path, category: str, filename: str):
    """读取文件内容。MD 返回渲染后 HTML，PDF 返回 (None, 'pdf')。"""
    path = safe_resolve(data_dir, category, filename)
    if not path.exists():
        raise HTTPException(status_code=404, detail=f"File '{filename}' not found")
    if path.suffix == ".md":
        text = path.read_text(encoding="utf-8")
        html = md_lib.markdown(text, extensions=["fenced_code", "tables"])
        return html, "markdown"
    elif path.suffix == ".pdf":
        return None, "pdf"
    else:
        raise HTTPException(status_code=400, detail="Unsupported file type")


def create_category(data_dir: Path, name: str) -> None:
    """新建分类目录。"""
    if not name or not VALID_CATEGORY_RE.match(name):
        raise InvalidNameError(f"Invalid category name: {name!r}")
    cat_path = data_dir / name
    if cat_path.exists():
        raise CategoryExistsError(f"Category '{name}' already exists")
    cat_path.mkdir(parents=False)


def save_upload(data_dir: Path, category: str, filename: str, content: bytes) -> None:
    """保存上传文件到指定分类目录。"""
    filename = _sanitize_filename(filename)
    suffix = Path(filename).suffix.lower()
    if suffix not in ALLOWED_EXTENSIONS:
        raise HTTPException(status_code=400, detail=f"File type '{suffix}' not allowed")
    cat_dir = data_dir / category
    if not cat_dir.is_dir():
        raise HTTPException(status_code=404, detail=f"Category '{category}' not found")
    # Normalize suffix to lowercase so list_files always finds the file
    stem = Path(filename).stem
    filename = stem + suffix
    target = safe_resolve(data_dir, category, filename)
    target.write_bytes(content)
