# MagicBox 知识库

本地知识库，用于存储和浏览 Markdown 与 PDF 文件。文件按分类组织，支持网页上传和手动放入文件夹两种方式。

## 目录结构

```
magicbox/
├── main.py              # FastAPI 应用入口，所有路由定义
├── storage.py           # 文件系统操作层（读写分类和文件）
├── requirements.txt     # Python 依赖
├── data/                # 知识库存储目录（每个子目录 = 一个分类）
│   ├── 技术笔记/
│   │   ├── Python入门.md
│   │   └── 深度学习.pdf
│   └── 论文收藏/
├── static/
│   └── style.css        # 全局样式
├── templates/
│   ├── base.html        # 公共布局（导航栏、Modal JS）
│   ├── index.html       # 首页：分类卡片网格
│   ├── category.html    # 分类页：文件列表
│   └── file.html        # 文件预览页（Markdown 渲染 / PDF 内嵌）
└── tests/
    ├── test_storage.py  # 存储层单元测试（18 个）
    └── test_routes.py   # HTTP 路由集成测试（15 个）
```

## 安装依赖

```bash
pip install -r requirements.txt
```

## 启动服务

```bash
uvicorn main:app --reload
```

服务启动后访问 [http://localhost:8000](http://localhost:8000)

生产环境（局域网访问）：

```bash
uvicorn main:app --host 0.0.0.0 --port 8000
```

## 使用方式

### 浏览分类

打开首页，所有分类以卡片形式展示，卡片上显示文件数量。点击卡片进入分类页，可以看到该分类下所有 `.md` 和 `.pdf` 文件。

### 新建分类

- **网页**：点击右上角「+ 新建分类」，输入分类名称，点击创建
- **手动**：在 `data/` 目录下新建文件夹，刷新页面即可看到

分类名称支持：中文、字母、数字、连字符 `-`、下划线 `_`

### 上传文件

进入某个分类页，点击右上角「⬆ 上传文件」，选择 `.md` 或 `.pdf` 文件上传。

也可以直接把文件复制到 `data/<分类名>/` 文件夹，刷新页面即可看到。

### 查看文件

- **Markdown**：点击文件名，内容以渲染后的 HTML 展示（支持标题、代码块、表格、引用等）
- **PDF**：点击文件名，PDF 内嵌在浏览器中展示

## 路由说明

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/` | 首页，展示所有分类 |
| GET | `/category/{name}` | 分类页，展示文件列表 |
| GET | `/view/{category}/{filename}` | 文件预览（MD 渲染 / PDF 内嵌） |
| GET | `/raw/{category}/{filename}` | 原始文件（供 PDF embed 使用） |
| POST | `/api/category` | 新建分类 |
| POST | `/api/upload/{category}` | 上传文件到指定分类 |

## 运行测试

```bash
python -m pytest tests/ -v
```

## 技术栈

- **Python 3.10+**
- **FastAPI** — Web 框架
- **Jinja2** — 服务端模板渲染
- **markdown** — Markdown 转 HTML
- **uvicorn** — ASGI 服务器
