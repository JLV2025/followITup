# FollowITup

> SmartSheet-like project management system · Single-file deployment · Gantt-driven · v0.7.28

## What is FollowITup?

FollowITup is a self-hosted, Gantt-chart-driven project management tool designed for small teams (~10 users). It helps you plan timelines, track progress, auto-schedule dependencies, and share project status — all from a single `.exe` running on Windows Server.

**Core features:**

- **Gantt chart** — drag-and-drop editing, dependency links (FS/SS/FF/SF), WBS hierarchy, critical path highlighting, today marker
- **Auto-scheduling** — when a task date changes or slips, dependent tasks cascade automatically; manually-locked tasks stay put with conflict warnings
- **Dashboard** — cross-project overview with stat cards, project health (red/yellow/green), mini Gantt timeline, upcoming milestones, personal to-do list
- **Task list view** — spreadsheet-style inline editing with status badges, progress bars, priority labels
- **Real-time collaboration** — WebSocket-powered, edits broadcast to all online users in the same project
- **Public read-only mode** — anyone with the URL can view the dashboard and Gantt charts; login to edit
- **Soft delete with recycle bin** — deleted projects and tasks can be recovered
- **Fiscal year support** — projects grouped and statistics calculated by fiscal year (FY)

## Quick Start

### Prerequisites

- Windows Server 2019+ or Windows 10/11
- No runtime dependencies (no Java, no Node.js, no IIS)

### Deployment

```bash
# 1. Copy files to server
copy followitup.exe C:\followitup\
copy config.yaml C:\followitup\

# 2. Edit config.yaml — set jwt_secret to a random string

# 3. Run
C:\followitup\followitup.exe -config C:\followitup\config.yaml

# 4. Open browser
# http://<server>:8080
```

**First-run admin account:** `admin@followitup.local` / `admin123`

### Run as Windows Service

```bash
sc create FollowITup binPath= "C:\followitup\followitup.exe -config C:\followitup\config.yaml" start= auto
sc start FollowITup
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.22+, chi router, modernc.org/sqlite (pure Go, no CGO) |
| Frontend | React 18, TypeScript, Zustand, dhtmlx-gantt (GPL v2) |
| Auth | bcrypt + JWT (local accounts); LDAP/AD ready (config toggle) |
| Real-time | gorilla/websocket (per-project rooms) |
| Build | go:embed frontend into single `.exe` |

## Project Structure

```
followITup/
├── backend/
│   ├── cmd/server/          # Entry point with embedded frontend
│   ├── internal/
│   │   ├── api/             # REST endpoints
│   │   ├── auth/            # Authentication & middleware
│   │   ├── db/              # SQLite connection + migrations
│   │   ├── models/          # Data models
│   │   ├── scheduler/       # Auto-scheduling engine
│   │   ├── server/          # Server bootstrap & config
│   │   └── ws/              # WebSocket hub
│   └── config.yaml
├── frontend/
│   ├── src/
│   │   ├── api/             # HTTP client + WebSocket + Gantt adapter
│   │   ├── components/      # Shared UI components
│   │   ├── pages/           # Dashboard, Login, ProjectGantt, TaskListView
│   │   └── stores/          # Zustand state stores
│   └── vite.config.ts
├── docs/
│   └── design-requirements.md
├── build.bat                # One-click build script
└── config.yaml.example
```

## Documentation

- **Design requirements**: `docs/design-requirements.md`
- **Implementation plan**: `.wolf/memory.md` and plan file
- **API**: See `backend/internal/api/` for endpoint definitions

## License

dhtmlx-gantt is GPL v2. This project is for internal use.

---

# FollowITup（中文）

> 类 SmartSheet 项目管理工具 · 单文件部署 · 甘特图驱动 · v0.7.28

## 这是什么？

FollowITup 是一个自托管的甘特图驱动项目管理工具，面向 10 人左右的小型团队。它帮助你规划时间线、跟踪进度、自动排程依赖关系，并通过浏览器分享项目状态。只需一个 `.exe` 文件即可在 Windows Server 上运行。

**核心功能：**

- **甘特图** — 拖拽编辑任务、创建依赖连线（FS/SS/FF/SF）、WBS 层级展开折叠、关键路径高亮、今日线
- **自动排程** — 任务日期变更或超期后，后续任务级联自动延期；手动锁定的任务不受影响但显示冲突标记
- **综合看板** — 跨项目总览：统计卡片、项目健康度（红/黄/绿灯）、迷你甘特图、近期里程碑、个人待办
- **任务列表** — 电子表格式行内编辑，状态标签、进度条、优先级一目了然
- **实时协作** — WebSocket 驱动，同一项目内编辑实时广播给所有在线用户
- **公开只读** — 任何人打开浏览器就能看到看板和甘特图，登录后可以编辑
- **软删除 + 回收站** — 删除的项目和任务可以恢复
- **财年支持** — 按财年（FY）分组展示和统计

## 快速上手

### 环境要求

- Windows Server 2019+ 或 Windows 10/11
- 无需运行时依赖（无需 Java、Node.js、IIS）

### 部署

```bash
# 1. 复制文件到服务器
copy followitup.exe C:\followitup\
copy config.yaml C:\followitup\

# 2. 修改 config.yaml — 将 jwt_secret 改为随机字符串

# 3. 运行
C:\followitup\followitup.exe -config C:\followitup\config.yaml

# 4. 打开浏览器
# http://<服务器地址>:8080
```

**首次运行管理员账号：** `admin@followitup.local` / `admin123`

### 注册为 Windows 服务

```bash
sc create FollowITup binPath= "C:\followitup\followitup.exe -config C:\followitup\config.yaml" start= auto
sc start FollowITup
```

## 技术栈

| 层 | 技术 |
|---|------|
| 后端 | Go 1.22+, chi 路由, modernc.org/sqlite（纯 Go，无需 CGO） |
| 前端 | React 18, TypeScript, Zustand 状态管理, dhtmlx-gantt（GPL v2） |
| 认证 | bcrypt + JWT（本地账号）；LDAP/AD 已预留开关 |
| 实时 | gorilla/websocket（按项目分房间广播） |
| 构建 | go:embed 将前端打包进单个 `.exe` |

## 项目结构

```
followITup/
├── backend/
│   ├── cmd/server/          # 入口（内嵌前端静态文件）
│   ├── internal/
│   │   ├── api/             # REST 端点
│   │   ├── auth/            # 认证与中间件
│   │   ├── db/              # SQLite 连接 + 迁移
│   │   ├── models/          # 数据模型
│   │   ├── scheduler/       # 自动排程引擎
│   │   ├── server/          # 服务器启动与配置
│   │   └── ws/              # WebSocket Hub
│   └── config.yaml
├── frontend/
│   ├── src/
│   │   ├── api/             # HTTP 客户端 + WebSocket + 甘特图适配层
│   │   ├── components/      # 共享 UI 组件
│   │   ├── pages/           # 看板/登录/甘特图/任务列表
│   │   └── stores/          # Zustand 状态
│   └── vite.config.ts
├── docs/
│   └── design-requirements.md  # 看板设计需求文档
├── build.bat                # 一键构建脚本
└── config.yaml.example      # 配置模板
```

## 相关文档

- **设计需求**：`docs/design-requirements.md`
- **实施计划与进展**：`.wolf/memory.md` 及计划文件
- **API 端点**：详见 `backend/internal/api/` 目录

## 许可证

dhtmlx-gantt 使用 GPL v2 许可证。本项目仅供内部使用。
