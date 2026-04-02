from datetime import datetime
from pathlib import Path
from fastapi import FastAPI, Request, HTTPException, UploadFile, File, Form
from fastapi.responses import HTMLResponse, FileResponse, RedirectResponse
from fastapi.staticfiles import StaticFiles
from fastapi.templating import Jinja2Templates
from storage import (
    list_categories, list_files, read_file,
    create_category, save_upload,
    CategoryExistsError, InvalidNameError,
)

BASE_DIR = Path(__file__).parent
DATA_DIR = BASE_DIR / "data"
DATA_DIR.mkdir(exist_ok=True)

app = FastAPI(title="MagicBox")
app.mount("/static", StaticFiles(directory=BASE_DIR / "static"), name="static")
templates = Jinja2Templates(directory=BASE_DIR / "templates")

# Jinja2 内置 urlencode 只支持 dict，注册字符串版本支持中文路径
import urllib.parse
templates.env.filters["urlencode"] = urllib.parse.quote


@app.get("/", response_class=HTMLResponse)
async def index(request: Request):
    categories = list_categories(DATA_DIR)
    return templates.TemplateResponse(request, "index.html", {
        "categories": categories,
    })


@app.post("/api/category")
async def api_create_category(name: str = Form(...)):
    try:
        create_category(DATA_DIR, name)
    except CategoryExistsError:
        raise HTTPException(status_code=400, detail=f"分类 '{name}' 已存在")
    except InvalidNameError:
        raise HTTPException(status_code=400, detail="分类名称包含非法字符")
    return RedirectResponse(url="/", status_code=303)


@app.get("/category/{category_name}", response_class=HTMLResponse)
async def category_page(request: Request, category_name: str):
    files = list_files(DATA_DIR, category_name)
    for f in files:
        f["mtime_str"] = datetime.fromtimestamp(f["mtime"]).strftime("%Y-%m-%d")
    return templates.TemplateResponse(request, "category.html", {
        "category": category_name,
        "files": files,
    })


@app.post("/api/upload/{category_name}")
async def api_upload(category_name: str, file: UploadFile = File(...)):
    cat_path = DATA_DIR / category_name
    if not cat_path.is_dir():
        raise HTTPException(status_code=404, detail=f"Category '{category_name}' not found")
    content = await file.read()
    save_upload(DATA_DIR, category_name, file.filename, content)
    return RedirectResponse(url=f"/category/{category_name}", status_code=303)


@app.get("/view/{category_name}/{filename}", response_class=HTMLResponse)
async def view_file(request: Request, category_name: str, filename: str):
    content, filetype = read_file(DATA_DIR, category_name, filename)
    return templates.TemplateResponse(request, "file.html", {
        "category": category_name,
        "filename": filename,
        "content": content,
        "filetype": filetype,
    })


@app.get("/raw/{category_name}/{filename}")
async def raw_file(category_name: str, filename: str):
    from storage import safe_resolve, PathTraversalError, VALID_CATEGORY_RE
    if not VALID_CATEGORY_RE.match(category_name):
        raise HTTPException(status_code=400, detail="Invalid category name")
    try:
        path = safe_resolve(DATA_DIR, category_name, filename)
    except PathTraversalError:
        raise HTTPException(status_code=400, detail="Invalid path")
    if not path.exists():
        raise HTTPException(status_code=404)
    return FileResponse(str(path))
