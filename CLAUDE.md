# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# OpenWolf

@.wolf/OPENWOLF.md

This project uses OpenWolf for context management. Read and follow .wolf/OPENWOLF.md every session. Check .wolf/cerebrum.md before generating code. Check .wolf/anatomy.md before reading files.

## Project Overview

FollowITup 是一个类似 SmartSheet 的网页版项目管理系统。面向 10 人左右小团队，在 Windows Server 上单 .exe 部署。

**核心功能**：甘特图驱动的项目规划（拖拽编辑、依赖连线）、前置任务自动排程（FS/SS/FF/SF + lag）、延迟级联传播、综合报告看板、基线对比、多人实时协作、LDAP/AD 认证。

**技术栈**：Go 1.22+ 后端 + React 18/TypeScript 前端 + SQLite（modernc.org/sqlite，纯 Go 无 CGO）+ WebSocket 实时同步。前端产物通过 `go:embed` 嵌入二进制，编译为单文件 `followitup.exe`。

## Architecture

```
Browser (React 18 + TypeScript + dhtmlx-gantt)
        │ HTTP REST + WebSocket
        ▼
Go Backend (chi router + embed static)
  ├── Auth (LDAP/AD + JWT)
  ├── REST API (projects / tasks / dependencies / members)
  ├── WebSocket Hub (per-project rooms, broadcast edits)
  ├── Auto-Scheduling Engine (CPM forward/backward pass + delay cascade)
  └── SQLite (modernc.org/sqlite, file at data/followitup.db)
```

**数据流**：用户编辑任务日期/依赖 → API 保存 → 触发自动排程引擎（BFS 遍历后继链，前向传播重算日期）→ 批量更新受影响任务 → WebSocket 广播变更给同一项目内所有在线用户。

## Key Dependencies

| 层 | 库 | 用途 |
|---|-----|------|
| 后端路由 | `go-chi/chi` | HTTP 路由 + 中间件 |
| 后端数据库 | `modernc.org/sqlite` | 纯 Go SQLite，无需 CGO |
| 后端认证 | `go-ldap/ldap/v3` | LDAP/AD 认证 |
| 后端认证 | `golang-jwt/jwt/v5` | JWT 签发与验证 |
| 后端实时 | `gorilla/websocket` | WebSocket 连接管理 |
| 前端甘特图 | `dhtmlx-gantt` (GPL v2) | 甘特图渲染与交互 |
| 前端构建 | `vite` | 打包为 `go:embed` 可嵌入的静态文件 |

## Database Schema（核心表）

- `projects` — 项目基本信息（名称、日期范围、状态）
- `tasks` — 任务/里程碑，支持 WBS 层级（`parent_id`），含 `manual_scheduled` 标志控制是否参与自动排程，`version` 字段用于乐观锁
- `dependencies` — 任务依赖关系（`predecessor_id → successor_id`），支持 FS/SS/FF/SF 四种类型 + `lag_days`
- `users` — LDAP 缓存 + 本地管理员后备
- `project_members` — 项目成员与角色
- `activity_log` — 操作审计日志

## Core Algorithm: Auto-Scheduling Engine

**前向传播**（Forward Pass）：
1. 从无前置的任务开始，ES = max(项目开始日期, 任务约束日期)
2. 对四种依赖类型分别计算后继的 ES/EF（FS: 后继.ES = 前置.EF + lag，SS: 后继.ES = 前置.ES + lag，FF/ SF 同理）
3. 递归直到全部任务计算完毕

**延迟传播**：任务日期变更或超期时，BFS 遍历所有后继链，重新执行前向传播；`manual_scheduled=1` 的任务跳过但标记冲突

**关键路径**：TF（总浮动时间）= LS - ES，TF = 0 的任务即为关键路径，在甘特图上红色高亮

## Frontend Routes

| 路由 | 页面 | 说明 |
|------|------|------|
| `/` | 综合看板（Dashboard） | 默认首页，设计详见 `docs/design-requirements.md` |
| `/project/:id` | 项目甘特图页 | 核心编辑页，全屏甘特图 + 任务表格联动 |
| `/project/:id/list` | 任务列表视图 | 表格式查看/编辑 |
| `/project/:id/members` | 成员管理 | 添加/移除项目成员 |
| `/project/:id/settings` | 项目设置 | 基本信息、模块开关 |
| `/admin/users` | 用户管理 | 仅管理员可见 |

## Visual & UX Standards

详见 `docs/design-requirements.md`。核心约束：
- 暖白底色（`#FAFAF9`），白卡片（`#FFFFFF`），极浅阴影
- 色彩克制：95% 灰调 + 5% 功能色（状态灯、今日线）
- 字体：Inter（英文/数字）+ Microsoft YaHei UI（中文）
- 仅桌面端（1920px / 1366px），不做移动端适配

## Common Commands

```bash
# 完整构建（前端 + 后端 → 单 .exe）
build.bat

# 或分步操作：
# 前端开发（热重载，端口 3000，API 代理到 8080）
cd frontend && npm run dev

# 前端构建
cd frontend && npm run build

# 后端编译（含内嵌前端）
cd backend && go build -o followitup.exe ./cmd/server/

# 运行（首次启动自动创建数据库 + admin 账号）
cd backend && ./followitup.exe config.yaml

# 后端测试
cd backend && go test ./...

# 前端测试
cd frontend && npm test
```
## Deployment

```bash
# 1. 复制单文件到服务器
copy followitup.exe C:\app\ + config.yaml

# 2. 运行
followitup.exe -config config.yaml

# 3. 注册为 Windows 服务
sc create FollowITup binPath= "C:\app\followitup.exe -config C:\app\config.yaml" start= auto
```

## Language Rules

- 所有对话、注释、文档使用**简体中文**
- 代码注释使用中文
- 提交信息使用中文
- 专业术语和缩写保留英文原文

## Workflow

- 架构决策前先询问
- 做最小改动，不重构无关代码
- 每次变更后跑测试，失败先修复再继续
- 每个逻辑变更单独提交
- 两种方案之间拿不准时，两个都解释，让我来选

## Out of Scope

- 移动端适配（仅桌面端）
- 第三方登录/OAuth（仅 LDAP/AD + 本地账号）
- 多语言国际化（仅简体中文）

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **followITup** (2483 symbols, 5905 relationships, 217 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze` in terminal first.

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `gitnexus_impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `gitnexus_detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `gitnexus_query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `gitnexus_context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `gitnexus_impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename` which understands the call graph.
- NEVER commit changes without running `gitnexus_detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/followITup/context` | Codebase overview, check index freshness |
| `gitnexus://repo/followITup/clusters` | All functional areas |
| `gitnexus://repo/followITup/processes` | All execution flows |
| `gitnexus://repo/followITup/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
