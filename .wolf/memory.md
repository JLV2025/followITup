# Memory

> Chronological action log. Hooks and AI append to this file automatically.
> Old sessions are consolidated by the daemon weekly.

## 待办事项

1. **财年定义**：FY27 = 2026-04-01 ~ 2027-03-31，年度切换和统计按财年计算
2. **协作感知**：某人正在编辑某条目时，其他在线用户可见该条目被选中并看到编辑者用户名，避免多人同时编辑冲突

---

## 2026-07-28 工作进展

### 需求调研
- 研究了 OpenProject 功能体系作为参考
- 以资深项目经理 + IT 工程师双视角梳理了核心需求
- 确认了"公开只读 + 登录编辑"的权限模型

### 设计决策（共 7 项，已写入实施计划）
| # | 决策 | 结论 |
|---|------|------|
| D1 | 甘特图数据适配 | 前端转换（gantt-adapter.ts） |
| D2 | 甘特图许可证 | dhtmlx-gantt GPL v2（免费） |
| D3 | API 规范 | 统一信封 + offset/limit + 软删除 |
| D4 | 排程引擎 | 环检测、多前置取 max、手动锁定仍是约束 |
| D5 | 前端状态管理 | Zustand |
| D6 | 数据库迁移 | Go 代码内置迁移 |
| D7 | 文件结构 | backend/ + frontend/ 分离 |

### 已完成的功能（截至 14:20）

| 阶段 | 交付内容 | 状态 |
|------|---------|------|
| 1 | Go 后端骨架 + React 前端骨架 | ✅ |
| 1 | SQLite 数据库迁移系统（6 张表） | ✅ |
| 1 | 本地认证系统（bcrypt + JWT + 登录页面） | ✅ |
| 1 | 前后端构建流水线（build.bat） | ✅ |
| 2 | 项目 API + 看板统计 API | ✅ |
| 2 | 前端 Dashboard（统计卡片、项目列表、年度切换、迷你甘特图） | ✅ |
| 3 | 任务 CRUD API（含乐观锁、软删除） | ✅ |
| 3 | 前端任务列表视图（行内编辑、状态标记） | ✅ |
| 4 | dhtmlx-gantt 集成（拖拽编辑、依赖连线、关键路径、今日线） | ✅ |
| 5 | 自动排程引擎（FS/SS/FF/SF + 延迟级联 + 环检测） | ✅ |
| 5 | 排程单元测试 7 项全部通过 | ✅ |
| 6 | WebSocket Hub（per-project rooms + 编辑广播） | ✅ |

### 待办事项

| 优先级 | 内容 | 预计工时 |
|--------|------|---------|
| 🔴 | **财年定义**：FY27 替换现有自然年逻辑（统计/看板/年度切换按 4月-3月 计算） | 1天 |
| 🔴 | **协作感知**：编辑者在线可见 + 条目锁定提示 + 前端 ws-client 接入甘特图 | 1天 |
| 🟡 | 基线对比功能（快照 + 计划 vs 实际） | 2天 |
| 🟡 | 通知提醒（浏览器通知 + 邮件） | 1-2天 |
| 🟡 | 项目列表页的"创建项目"表单（当前缺少 UI 入口） | 0.5天 |
| 🟢 | LDAP/AD 认证接入 | 1天 |
| 🟢 | Excel/CSV 导出 | 0.5天 |
| 🟢 | Windows 服务注册脚本 | 0.5天 |
| 🟢 | 代码清理：dhtmlx-gantt 的 `any` 类型替代为完整 TS 类型 | 0.5天 |

### 项目文件统计

```
后端：~2400 行 Go（20 个源文件）
前端：~1500 行 TSX/CSS（14 个源文件）
测试：7 项排程算法测试 ✓
构建产物：followitup.exe（~30MB 单文件）
```

### 运行方式

```bash
build.bat                                  # 完整构建
cd backend && followitup.exe config.yaml   # 启动
# http://localhost:8080
# admin@followitup.local / admin123
```

---

## Session: 2026-07-28 12:35
| 12:49 | Session end: 1 writes across 1 files (smartsheet-windows-cryptic-hennessy.md) | 0 reads | ~1736 tok |
| 12:56 | Session end: 1 writes across 1 files (smartsheet-windows-cryptic-hennessy.md) | 0 reads | ~1736 tok |
| 13:04 | Session end: 1 writes across 1 files (smartsheet-windows-cryptic-hennessy.md) | 0 reads | ~1736 tok |
| 13:06 | Created docs/design-requirements.md | — | ~2568 |
| 13:07 | Session end: 2 writes across 2 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md) | 0 reads | ~4487 tok |
| 13:09 | Edited CLAUDE.md | expanded (+60 lines) | ~324 |
| 13:09 | Session end: 3 writes across 3 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md) | 1 reads | ~4892 tok |
| 13:11 | Created CLAUDE.md | — | ~965 |
| 13:11 | Session end: 4 writes across 3 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md) | 3 reads | ~10099 tok |
| 13:12 | Session end: 4 writes across 3 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md) | 6 reads | ~10099 tok |
| 13:13 | Created .claude/skills/project-status/SKILL.md | — | ~172 |
| 13:14 | Created .claude/skills/ldap-sync/SKILL.md | — | ~276 |
| 13:14 | Created .claude/agents/security-reviewer.md | — | ~255 |
| 13:14 | Created .claude/agents/gantt-tester.md | — | ~276 |
| 13:14 | Edited .claude/settings.json | added 1 condition(s) | ~235 |
| 13:15 | Session end: 9 writes across 7 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 7 reads | ~11824 tok |
| 13:18 | Session end: 9 writes across 7 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 7 reads | ~11824 tok |
| 13:20 | Edited C:/Users/jingl/.claude/plans/smartsheet-windows-cryptic-hennessy.md | 5→7 lines | ~50 |
| 13:21 | Edited C:/Users/jingl/.claude/plans/smartsheet-windows-cryptic-hennessy.md | 3→7 lines | ~50 |
| 13:21 | Edited C:/Users/jingl/.claude/plans/smartsheet-windows-cryptic-hennessy.md | expanded (+7 lines) | ~416 |
| 13:21 | Edited C:/Users/jingl/.claude/plans/smartsheet-windows-cryptic-hennessy.md | expanded (+66 lines) | ~510 |
| 13:22 | Edited C:/Users/jingl/.claude/plans/smartsheet-windows-cryptic-hennessy.md | expanded (+15 lines) | ~309 |
| 13:22 | Edited C:/Users/jingl/.claude/plans/smartsheet-windows-cryptic-hennessy.md | 26→29 lines | ~178 |
| 13:22 | Session end: 15 writes across 7 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 7 reads | ~14045 tok |
| 13:25 | Session end: 15 writes across 7 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 7 reads | ~14045 tok |
| 13:26 | Session end: 15 writes across 7 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 7 reads | ~14045 tok |
| 13:27 | Session end: 15 writes across 7 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 7 reads | ~14045 tok |
| 13:27 | Session end: 15 writes across 7 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 7 reads | ~14045 tok |
| 13:30 | Session end: 15 writes across 7 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 7 reads | ~14045 tok |
| 13:31 | Session end: 15 writes across 7 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 7 reads | ~14045 tok |
| 13:34 | Session end: 15 writes across 7 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 7 reads | ~14045 tok |
| 13:36 | Session end: 15 writes across 7 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 7 reads | ~14045 tok |
| 13:42 | Session end: 15 writes across 7 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 7 reads | ~14045 tok |
| 13:44 | Edited C:/Users/jingl/.claude/plans/smartsheet-windows-cryptic-hennessy.md | expanded (+80 lines) | ~531 |
| 13:48 | Created backend/config.yaml | — | ~116 |
| 13:48 | Created backend/internal/models/models.go | — | ~989 |
| 13:48 | Created backend/cmd/server/config.go | — | ~382 |
| 13:49 | Created backend/internal/db/sqlite.go | — | ~1535 |
| 13:49 | Created backend/internal/auth/auth.go | — | ~1267 |
| 13:49 | Created backend/internal/auth/middleware.go | — | ~778 |
| 13:49 | Created backend/internal/api/auth.go | — | ~689 |
| 13:50 | Edited backend/internal/auth/auth.go | expanded (+14 lines) | ~117 |
| 13:50 | Created backend/internal/api/helpers.go | — | ~288 |
| 13:50 | Created backend/internal/api/auth.go | — | ~635 |
| 13:50 | Created backend/cmd/server/main.go | — | ~910 |
| 13:51 | Created backend/internal/server/server.go | — | ~594 |
| 13:51 | Created backend/cmd/server/main.go | — | ~67 |
| 13:51 | Created backend/internal/server/server.go | — | ~922 |
| 13:52 | Created backend/cmd/server/main.go | — | ~140 |
| 13:53 | Edited backend/internal/api/helpers.go | modified writeJSON() | ~213 |
| 13:53 | Edited backend/internal/api/auth.go | modified RegisterRoutes() | ~63 |
| 13:53 | Edited backend/internal/api/auth.go | 7→9 lines | ~34 |
| 13:54 | Created frontend/src/App.tsx | — | ~192 |
| 13:54 | Created frontend/src/index.css | — | ~416 |
| 13:54 | Created frontend/src/main.tsx | — | ~93 |
| 13:54 | Created frontend/src/stores/authStore.ts | — | ~388 |
| 13:54 | Created frontend/src/api/client.ts | — | ~170 |
| 13:54 | Created frontend/src/components/Navbar.tsx | — | ~234 |
| 13:54 | Created frontend/src/pages/Login.tsx | — | ~584 |
| 13:55 | Created frontend/src/pages/Dashboard.tsx | — | ~497 |
| 13:55 | Created frontend/vite.config.ts | — | ~106 |
| 13:55 | Created frontend/src/styles/components.css | — | ~1032 |
| 13:55 | Edited frontend/src/index.css | 3→4 lines | ~22 |
| 13:56 | Edited frontend/src/pages/Login.tsx | inline fix | ~14 |
| 13:57 | Edited backend/internal/auth/auth.go | 11→12 lines | ~123 |
| 13:57 | Edited backend/internal/auth/auth.go | modified Before() | ~63 |
| 13:57 | Edited backend/internal/auth/auth.go | 12→16 lines | ~146 |
| 13:58 | Edited backend/internal/auth/auth.go | 13→16 lines | ~131 |
| 13:59 | Created build.bat | — | ~166 |
| 13:59 | Created config.yaml.example | — | ~90 |
| 13:59 | Edited CLAUDE.md | 25→25 lines | ~103 |
| 14:00 | 完成阶段1: Go后端 + React前端骨架 + 本地认证 + 数据库迁移 | 55 files created | ✅ 编译通过, API测试通过 | ~15K tok |
| 14:00 | Session end: 53 writes across 28 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 11 reads | ~31626 tok |
| 14:03 | Created backend/internal/api/projects.go | — | ~2351 |
| 14:03 | Created backend/internal/api/tasks.go | — | ~1967 |
| 14:03 | Edited backend/internal/api/projects.go | 11→12 lines | ~44 |
| 14:03 | Edited backend/internal/server/server.go | expanded (+8 lines) | ~85 |
| 14:04 | Edited backend/internal/api/tasks.go | modified NewTaskHandler() | ~215 |
| 14:04 | Edited backend/internal/api/tasks.go | 10→11 lines | ~42 |
| 14:04 | Created frontend/src/stores/dashboardStore.ts | — | ~425 |
| 14:05 | Created frontend/src/pages/Dashboard.tsx | — | ~2132 |
| 14:05 | Edited frontend/src/styles/components.css | expanded (+239 lines) | ~1157 |
| 14:05 | Edited frontend/src/pages/Dashboard.tsx | 9→6 lines | ~78 |
| 14:06 | Edited frontend/src/pages/Dashboard.tsx | inline fix | ~25 |
| 14:07 | Edited backend/internal/api/projects.go | 14→12 lines | ~134 |
| 14:07 | Edited backend/internal/api/projects.go | modified Next() | ~85 |
| 14:07 | Edited backend/internal/api/projects.go | 5→7 lines | ~83 |
| 14:08 | Created frontend/src/pages/ProjectDetail.tsx | — | ~467 |
| 14:08 | Created frontend/src/pages/TaskListView.tsx | — | ~2717 |
| 14:09 | Created frontend/src/App.tsx | — | ~298 |
| 14:09 | Edited frontend/src/styles/components.css | expanded (+173 lines) | ~795 |
| 14:09 | Edited frontend/src/pages/TaskListView.tsx | 3→2 lines | ~28 |
| 14:10 | Edited frontend/src/pages/TaskListView.tsx | 4→3 lines | ~33 |
| 14:10 | Edited frontend/src/pages/TaskListView.tsx | reduced (-25 lines) | ~59 |
| 14:10 | Created frontend/src/pages/TaskListView.tsx | — | ~2549 |
| 14:10 | Edited frontend/src/styles/components.css | expanded (+14 lines) | ~65 |
| 14:12 | Edited frontend/src/stores/dashboardStore.ts | 10→12 lines | ~114 |
| 14:13 | Edited backend/internal/api/projects.go | 4→3 lines | ~56 |
| 14:13 | Edited backend/internal/api/projects.go | modified Next() | ~61 |
| 14:15 | Created frontend/src/api/gantt-adapter.ts | — | ~837 |
| 14:15 | Created frontend/src/stores/ganttStore.ts | — | ~744 |
| 14:16 | Created frontend/src/pages/ProjectGantt.tsx | — | ~946 |
| 14:16 | Created frontend/src/pages/ProjectGantt.tsx | — | ~1098 |
| 14:16 | Created frontend/src/App.tsx | — | ~310 |
| 14:17 | Edited frontend/src/styles/components.css | expanded (+53 lines) | ~280 |
| 14:17 | Created frontend/src/pages/ProjectGantt.tsx | — | ~1158 |
| 14:17 | Edited frontend/src/stores/ganttStore.ts | 2→2 lines | ~24 |
| 14:18 | Edited frontend/src/pages/ProjectGantt.tsx | modified function() | ~56 |
| 14:18 | Edited frontend/src/pages/ProjectGantt.tsx | modified if() | ~27 |
| 14:18 | Edited frontend/src/pages/ProjectGantt.tsx | modified function() | ~54 |
| 14:19 | Created backend/internal/scheduler/scheduler.go | — | ~1897 |
| 14:20 | Edited backend/internal/api/tasks.go | 11→13 lines | ~52 |
| 14:20 | Edited backend/internal/api/tasks.go | modified func() | ~110 |
| 14:20 | Edited backend/internal/api/tasks.go | modified triggersReschedule() | ~144 |
| 14:20 | Edited backend/internal/api/tasks.go | modified DeleteDependency() | ~167 |
| 14:21 | Edited backend/internal/api/tasks.go | modified AddDependency() | ~180 |
| 14:21 | Created backend/internal/scheduler/scheduler_test.go | — | ~1178 |
| 14:22 | Edited backend/internal/scheduler/scheduler_test.go | 4→3 lines | ~32 |
| 14:23 | Created backend/internal/ws/hub.go | — | ~1196 |
| 14:23 | Edited backend/internal/server/server.go | 3→7 lines | ~53 |
| 14:23 | Edited backend/internal/server/server.go | 3→4 lines | ~29 |
| 14:23 | Created frontend/src/api/ws-client.ts | — | ~637 |
| 14:24 | 阶段2-6完成: 看板/任务CRUD/甘特图/排程引擎/WebSocket | ~20 files | ✅ 编译通过, 7项排程测试全过 | ~12K tok |
| 14:24 | Session end: 102 writes across 40 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 17 reads | ~67771 tok |
| 14:26 | Session end: 102 writes across 40 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 17 reads | ~67771 tok |
| 14:28 | Created README.md | — | ~1587 |
| 14:29 | Created README.html | — | ~2529 |
| 14:29 | Session end: 104 writes across 42 files (smartsheet-windows-cryptic-hennessy.md, design-requirements.md, CLAUDE.md, SKILL.md, security-reviewer.md) | 17 reads | ~72181 tok |

## Session: 2026-07-29 08:01

| Time | Action | File(s) | Outcome | ~Tokens |
|------|--------|---------|---------|--------|
| 08:12 | Edited backend/config.yaml | 3→6 lines | ~21 |
| 08:12 | Edited backend/internal/server/config.go | expanded (+6 lines) | ~76 |
| 08:12 | Edited backend/internal/server/config.go | 4→5 lines | ~36 |
| 08:13 | Edited backend/internal/api/projects.go | modified NewProjectHandler() | ~93 |
| 08:13 | Edited backend/internal/server/server.go | 2→2 lines | ~29 |
| 08:13 | Created backend/internal/util/fiscal.go | — | ~664 |
| 08:14 | Created backend/internal/util/fiscal_test.go | — | ~551 |
| 08:14 | Created backend/internal/util/fiscal.go | — | ~744 |
| 08:15 | Created backend/internal/util/fiscal_test.go | — | ~699 |
| 08:15 | Edited backend/internal/api/projects.go | 12→13 lines | ~52 |
| 08:16 | Edited backend/internal/api/projects.go | modified DashboardStats() | ~1138 |
| 08:16 | Created frontend/src/stores/settingsStore.ts | — | ~799 |
| 08:17 | Created frontend/src/stores/dashboardStore.ts | — | ~533 |
| 08:17 | Created frontend/src/pages/Dashboard.tsx | — | ~2507 |
| 08:17 | Edited frontend/src/styles/components.css | expanded (+25 lines) | ~244 |
| 08:18 | Edited config.yaml.example | 3→6 lines | ~20 |
| 08:19 | 财年定义改造完成: config添加fiscal配置 + util/fiscal.go工具模块 + projects.go支持?fy=参数 + settingsStore + Dashboard动态年度选择器 | backend/ (6 files), frontend/ (3 files) | ✅ 编译+测试全过 | ~3K tok |
| 08:19 | Session end: 16 writes across 11 files (config.yaml, config.go, projects.go, server.go, fiscal.go) | 11 reads | ~19119 tok |
| 08:30 | Session end: 16 writes across 11 files (config.yaml, config.go, projects.go, server.go, fiscal.go) | 16 reads | ~25194 tok |
| 08:32 | Edited backend/internal/ws/hub.go | expanded (+7 lines) | ~158 |
| 08:32 | Edited backend/internal/ws/hub.go | 4→9 lines | ~68 |
| 08:32 | Edited backend/internal/ws/hub.go | modified BroadcastTaskUpdate() | ~312 |
| 08:33 | Edited backend/internal/api/tasks.go | 13→14 lines | ~59 |
| 08:33 | Edited backend/internal/api/tasks.go | modified NewTaskHandler() | ~70 |
| 08:33 | Edited backend/internal/auth/middleware.go | modified GetUserID() | ~118 |
| 08:35 | Created backend/internal/api/tasks.go | — | ~2542 |
| 08:35 | Edited backend/internal/server/server.go | 7→7 lines | ~59 |
| 08:35 | Created frontend/src/api/ws-client.ts | — | ~812 |
| 08:36 | Created frontend/src/stores/ganttStore.ts | — | ~1192 |
| 08:36 | Created frontend/src/pages/ProjectGantt.tsx | — | ~2118 |
| 08:38 | Edited frontend/src/pages/ProjectGantt.tsx | inline fix | ~16 |
| 08:38 | Edited frontend/src/api/ws-client.ts | modified subscribe() | ~31 |
| 08:39 | Edited frontend/src/pages/ProjectGantt.tsx | modified drawFocus() | ~170 |
| 08:40 | 协作感知完成: WS Hub增加broadcastExcept/GetOnlineUsers + TaskHandler注入Hub广播变更 + ProjectGantt接入WS实现聚焦指示器 | ws/hub.go, api/tasks.go, auth/middleware.go, server.go, ws-client.ts, ganttStore.ts, ProjectGantt.tsx, components.css | ✅ 编译+测试全过 | ~5K tok |
| 08:40 | Session end: 30 writes across 17 files (config.yaml, config.go, projects.go, server.go, fiscal.go) | 17 reads | ~34150 tok |
| 08:41 | Session end: 30 writes across 17 files (config.yaml, config.go, projects.go, server.go, fiscal.go) | 17 reads | ~34150 tok |
| 08:43 | Session end: 30 writes across 17 files (config.yaml, config.go, projects.go, server.go, fiscal.go) | 18 reads | ~36047 tok |
| 08:48 | Edited backend/internal/scheduler/scheduler_test.go | 6→6 lines | ~64 |
| 08:52 | 修复排程引擎多前置取max bug + 前向传播只推后不拉前 | scheduler.go, scheduler_test.go | ✅ 8项测试全过 | ~2K tok |
| 08:54 | Session end: 31 writes across 18 files (config.yaml, config.go, projects.go, server.go, fiscal.go) | 19 reads | ~37635 tok |
| 09:02 | Session end: 31 writes across 18 files (config.yaml, config.go, projects.go, server.go, fiscal.go) | 20 reads | ~38472 tok |
| 09:06 | Session end: 31 writes across 18 files (config.yaml, config.go, projects.go, server.go, fiscal.go) | 20 reads | ~38472 tok |
| 09:12 | Edited backend/internal/db/sqlite.go | 2→7 lines | ~47 |
| 09:13 | Edited backend/internal/models/models.go | 2→4 lines | ~81 |
| 09:13 | Created backend/internal/scheduler/scheduler.go | — | ~3056 |
| 09:14 | Created backend/internal/scheduler/scheduler_test.go | — | ~2138 |
| 09:14 | Edited backend/internal/scheduler/scheduler_test.go | 2→2 lines | ~20 |
| 09:16 | Edited backend/internal/scheduler/scheduler.go | 3→3 lines | ~20 |
| 09:16 | Edited backend/internal/scheduler/scheduler.go | 29→26 lines | ~217 |
| 09:16 | Edited backend/internal/scheduler/scheduler_test.go | 7→7 lines | ~67 |
| 09:16 | Edited backend/internal/scheduler/scheduler_test.go | 3→3 lines | ~29 |
| 09:17 | Edited backend/internal/scheduler/scheduler_test.go | modified TestStartNoEarlierThanConstraint() | ~270 |
| 09:17 | Edited backend/internal/scheduler/scheduler_test.go | 4→4 lines | ~41 |
| 09:17 | Edited backend/internal/api/tasks.go | 7→8 lines | ~126 |
| 09:18 | Edited backend/internal/api/tasks.go | 7→8 lines | ~102 |
| 09:18 | Edited backend/internal/api/tasks.go | 11→13 lines | ~203 |
| 09:18 | Edited backend/internal/api/tasks.go | 8→9 lines | ~146 |
| 09:18 | Edited backend/internal/api/tasks.go | 13→13 lines | ~182 |
| 09:18 | Edited frontend/src/api/gantt-adapter.ts | 7→9 lines | ~60 |
| 09:19 | Created frontend/src/api/gantt-adapter.ts | — | ~906 |
| 09:20 | 倒推排程完成: 约束字段migration + backwardPass + start_no_earlier_than/finish_no_later_than + 关键路径浮动 | sqlite.go, models.go, scheduler.go(+400行), tasks.go, gantt-adapter.ts, scheduler_test.go(+3用例) | ✅ 11项测试全过 | ~6K tok |
| 09:20 | Session end: 49 writes across 22 files (config.yaml, config.go, projects.go, server.go, fiscal.go) | 22 reads | ~50342 tok |
| 09:23 | Session end: 49 writes across 22 files (config.yaml, config.go, projects.go, server.go, fiscal.go) | 22 reads | ~50342 tok |

## 2026-07-29 上午会话总结

### 完成 3 项功能 + 1 个 bug 修复

1. **财年定义** — config.yaml 新增 fiscal.year_start_month，后端 API 支持 ?fy= 参数，前端 Dashboard 自然年/财年一键切换，localStorage 持久化偏好
2. **协作感知** — WS Hub 新增 broadcastExcept，TaskHandler 注入 Hub 写操作后广播，甘特图 addTaskLayer 渲染彩色聚焦标签，15秒自动过期
3. **倒推排程** — tasks 表新增 constraint_type/constraint_date，backwardPass() 后向传播，SNET/FNLT 两种约束，关键路径浮动计算
4. **修复多前置取max** — forwardPass 改用 candidateStart > succ.StartDate，新增加测试用例

### 质量
- ✅ 后端 30 项测试全过（排程 11 + 财年 6 + 内置 13）
- ✅ 前端 TypeScript 编译通过
- ✅ 完整 .exe 构建成功

### 剩余待办
🟡 基线对比、通知提醒、创建项目表单
🟢 LDAP认证、导出、服务脚本、类型清理、约束UI、关键路径高亮
| 09:26 | Session end: 49 writes across 22 files (config.yaml, config.go, projects.go, server.go, fiscal.go) | 22 reads | ~50342 tok |

## Session: 2026-07-30 08:26

| Time | Action | File(s) | Outcome | ~Tokens |
|------|--------|---------|---------|--------|
| 08:30 | 创建项目功能: Dashboard标题栏添加"+ 创建项目"按钮 + 模态表单（名称/日期/描述） | Dashboard.tsx, components.css | ✅ 通过浏览器验证：创建2个项目，列表和迷你甘特图正常 | ~3K tok |
| 08:40 | 修复 SQLite WAL 模式连接池：SetMaxOpenConns(1)→(4)，解决 POST 后 GET 永久挂起 | sqlite.go | ✅ 登录+创建项目全流程通过 | ~500 tok |
| 12:27 | Created frontend/src/pages/Dashboard.tsx | — | ~3654 |
| 12:27 | Edited frontend/src/styles/components.css | CSS: display, align-items, justify-content | ~68 |
| 12:27 | Edited frontend/src/styles/components.css | expanded (+19 lines) | ~131 |
| 12:27 | Edited frontend/src/styles/components.css | expanded (+46 lines) | ~273 |
| 12:29 | Session end: 4 writes across 2 files (Dashboard.tsx, components.css) | 7 reads | ~16627 tok |
| 12:41 | Edited backend/internal/db/sqlite.go | 4→5 lines | ~36 |
| 12:45 | Session end: 5 writes across 3 files (Dashboard.tsx, components.css, sqlite.go) | 14 reads | ~21780 tok |
| 13:00 | 修复 CreateTask SQL 占位符多一个 ? (17→16) | tasks.go | ✅ 任务创建成功 | ~100 tok |
| 13:05 | 修复 SPA 路由：直接访问 /project/6 回退到 index.html | server.go | ✅ SPA 路由正常 | ~200 tok |
| 13:10 | 修复 models.go：CreatedAt/UpdatedAt time.Time→string (modernc 不支持 scan time) | models.go | ✅ 13条任务正常返回 | ~200 tok |
| 13:22 | 修复 ProjectGantt：containerRef 竞态 + dhtmlx-gantt v10 API (addTaskLayer/addMarker) | ProjectGantt.tsx | ✅ 甘特图正常渲染13行任务 | ~500 tok |
| 13:25 | 创建"新房装修"示例项目：13个任务+15条依赖链，含自动排程日期 | seed/main.go | ✅ Dashboard看板显示 + 甘特图渲染 | ~2K tok |
| 13:58 | WBS 层级折叠功能：gantt-adapter($open+type) + ProjectGantt(4项配置+assignee列+react按钮) + TaskListView(缩进←→按钮+深度缩进) + scheduler(ParentID+跳过汇总+rollupParentDates) + tasks.go(parent_id校验) | 5 files | ✅ 13行甘特图含26个树形折叠图标 + 14行列表含深度缩进 + "+ 添加任务" 按钮 POST 201 持久化 + 负责人列正常 | ~4K tok |
| — | **今日共修复 6 bug + 实现 1 新功能体系** | 6+5 files | ✅ 全栈功能验证通过 | — |
| 12:48 | Edited backend/internal/api/tasks.go | inline fix | ~16 |
| 12:50 | Edited backend/internal/server/server.go | modified Get() | ~195 |
| 12:50 | Edited backend/internal/server/server.go | modified mountFrontend() | ~320 |
| 12:52 | Created ../../tmp/create-project.json | — | ~34 |
| 12:54 | Created backend/cmd/seed/main.go | — | ~858 |
| 12:57 | Created backend/cmd/seed/query.go | — | ~202 |
| 12:59 | Created backend/cmd/seed/fix.go | — | ~168 |
| 12:59 | Created backend/cmd/seed/schedule.go | — | ~248 |
| 13:00 | Created backend/cmd/seed/enddate.go | — | ~228 |
| 13:03 | Edited backend/internal/api/tasks.go | modified ListTasks() | ~110 |
| 13:04 | Edited backend/internal/api/tasks.go | modified Next() | ~24 |
| 13:04 | Edited backend/internal/api/tasks.go | 3→4 lines | ~38 |
| 13:04 | Edited backend/internal/api/tasks.go | 4→6 lines | ~35 |
| 13:06 | Edited backend/internal/models/models.go | 2→2 lines | ~23 |
| 13:06 | Edited backend/internal/api/tasks.go | removed 6 lines | ~7 |
| 13:06 | Edited backend/internal/api/tasks.go | modified Next() | ~17 |
| 13:06 | Edited backend/internal/api/tasks.go | 4→3 lines | ~19 |
| 13:07 | Edited backend/internal/api/tasks.go | 6→4 lines | ~12 |
| 13:10 | Edited backend/internal/models/models.go | 5→3 lines | ~8 |
| 13:12 | Created backend/cmd/seed/finalize.go | — | ~232 |
| 13:13 | Edited backend/cmd/seed/finalize.go | modified main() | ~31 |
| 13:14 | Created backend/cmd/seed/verify.go | — | ~173 |
| 13:15 | Created backend/cmd/seed/verify.go | — | ~244 |
| 13:16 | Created backend/cmd/seed/verify.go | — | ~354 |
| 13:17 | Created backend/cmd/seed/verify.go | — | ~392 |
| 13:20 | Edited frontend/src/pages/ProjectGantt.tsx | 3→4 lines | ~53 |
| 13:21 | Edited frontend/src/pages/ProjectGantt.tsx | inline fix | ~19 |
| 13:21 | Edited frontend/src/pages/ProjectGantt.tsx | 2→3 lines | ~21 |
| 13:21 | Edited frontend/src/pages/ProjectGantt.tsx | 3→3 lines | ~22 |
| 13:21 | Edited frontend/src/pages/ProjectGantt.tsx | 3→3 lines | ~19 |
| 13:23 | Edited frontend/src/pages/ProjectGantt.tsx | CSS: display | ~100 |
| 13:24 | Edited frontend/src/pages/ProjectGantt.tsx | modified drawFocus() | ~32 |
| 13:24 | Edited frontend/src/pages/ProjectGantt.tsx | added 1 condition(s) | ~201 |
| 13:26 | Edited frontend/src/pages/ProjectGantt.tsx | added error handling | ~70 |
| 13:29 | Session end: 39 writes across 15 files (Dashboard.tsx, components.css, sqlite.go, tasks.go, server.go) | 22 reads | ~37711 tok |
| 13:36 | Session end: 39 writes across 15 files (Dashboard.tsx, components.css, sqlite.go, tasks.go, server.go) | 23 reads | ~40222 tok |
| 13:37 | Session end: 39 writes across 15 files (Dashboard.tsx, components.css, sqlite.go, tasks.go, server.go) | 23 reads | ~40222 tok |
| 13:45 | Created C:/Users/jingl/.claude/plans/review-groovy-octopus.md | — | ~1032 |
| 13:48 | Created C:/Users/jingl/.claude/plans/review-groovy-octopus.md | — | ~1143 |
| 13:48 | Session end: 41 writes across 16 files (Dashboard.tsx, components.css, sqlite.go, tasks.go, server.go) | 28 reads | ~47211 tok |
| 13:50 | Edited frontend/src/api/gantt-adapter.ts | 11→12 lines | ~98 |
| 13:50 | Edited frontend/src/api/gantt-adapter.ts | 2→3 lines | ~31 |
| 13:50 | Edited frontend/src/pages/ProjectGantt.tsx | added 1 import(s) | ~53 |
| 13:51 | Edited frontend/src/pages/ProjectGantt.tsx | CSS: type | ~268 |
| 13:51 | Edited frontend/src/pages/ProjectGantt.tsx | added error handling | ~214 |
| 13:52 | Created frontend/src/pages/TaskListView.tsx | — | ~3323 |
| 13:52 | Edited frontend/src/styles/components.css | modified not() | ~122 |
| 13:52 | Edited backend/internal/scheduler/scheduler.go | 14→15 lines | ~127 |
| 13:52 | Edited backend/internal/scheduler/scheduler.go | 4→4 lines | ~58 |
| 13:52 | Edited backend/internal/scheduler/scheduler.go | 2→2 lines | ~40 |
| 13:53 | Edited backend/internal/scheduler/scheduler.go | expanded (+11 lines) | ~96 |
| 13:53 | Edited backend/internal/scheduler/scheduler.go | inline fix | ~33 |
| 13:53 | Edited backend/internal/scheduler/scheduler.go | 5→5 lines | ~25 |
| 13:54 | Edited backend/internal/scheduler/scheduler.go | inline fix | ~10 |
| 13:54 | Edited backend/internal/scheduler/scheduler.go | inline fix | ~20 |
| 13:54 | Edited backend/internal/scheduler/scheduler.go | 6→6 lines | ~29 |
| 13:55 | Edited backend/internal/scheduler/scheduler.go | modified rollupParentDates() | ~358 |
| 13:55 | Edited backend/internal/api/tasks.go | modified CreateTask() | ~139 |
| 13:55 | Edited backend/internal/api/tasks.go | modified UpdateTask() | ~166 |
| 13:55 | Edited backend/internal/api/tasks.go | modified boolToInt2() | ~155 |
| 13:56 | Edited backend/internal/api/tasks.go | 6→7 lines | ~21 |
| 13:58 | Created backend/cmd/seed/tree.go | — | ~403 |
| 14:01 | Edited frontend/src/pages/ProjectGantt.tsx | modified attachEvent() | ~218 |
| 14:03 | Edited frontend/src/pages/ProjectGantt.tsx | CSS: task | ~213 |
| 14:04 | Edited frontend/src/pages/ProjectGantt.tsx | 2→2 lines | ~24 |
| 14:06 | Created backend/cmd/cleanup/main.go | — | ~139 |
| 14:07 | Edited frontend/src/pages/ProjectGantt.tsx | inline fix | ~65 |
| 14:08 | Edited frontend/src/pages/ProjectGantt.tsx | modified attachEvent() | ~35 |
| 14:09 | Edited frontend/src/pages/ProjectGantt.tsx | inline fix | ~6 |
| 14:11 | Created frontend/src/pages/ProjectGantt.tsx | — | ~2405 |
| 14:15 | Session end: 71 writes across 20 files (Dashboard.tsx, components.css, sqlite.go, tasks.go, server.go) | 28 reads | ~57363 tok |
| 14:43 | Edited frontend/src/pages/ProjectGantt.tsx | inline fix | ~40 |
| 14:43 | Edited frontend/src/pages/ProjectGantt.tsx | CSS: assignee | ~163 |
| 14:44 | Edited frontend/src/pages/ProjectGantt.tsx | inline fix | ~44 |
| 14:44 | Edited frontend/src/pages/ProjectGantt.tsx | inline fix | ~35 |
| 14:46 | Edited frontend/src/pages/ProjectGantt.tsx | added 2 condition(s) | ~172 |
| 14:47 | Edited frontend/src/pages/ProjectGantt.tsx | CSS: assignee | ~93 |
| 14:48 | Edited frontend/src/pages/ProjectGantt.tsx | modified attachEvent() | ~56 |
| 14:48 | Edited frontend/src/pages/ProjectGantt.tsx | added 1 condition(s) | ~152 |
| 14:50 | Edited frontend/src/pages/ProjectGantt.tsx | 4→1 lines | ~13 |
| 14:50 | Edited frontend/src/pages/ProjectGantt.tsx | 1→3 lines | ~36 |
| 14:52 | Edited frontend/src/api/gantt-adapter.ts | added 3 condition(s) | ~168 |
| 14:53 | Created backend/cmd/fixdata/main.go | — | ~83 |
| 14:55 | Created backend/cmd/fixdata/main.go | — | ~555 |
| 14:56 | Created backend/cmd/fixdata/main.go | — | ~531 |
| 14:58 | Edited frontend/src/pages/ProjectGantt.tsx | inline fix | ~74 |
| 15:00 | Edited frontend/src/pages/ProjectGantt.tsx | inline fix | ~12 |
| 15:00 | Edited frontend/src/api/gantt-adapter.ts | added 1 condition(s) | ~151 |
| 15:01 | Session end: 88 writes across 20 files (Dashboard.tsx, components.css, sqlite.go, tasks.go, server.go) | 28 reads | ~59903 tok |
| 15:14 | Created C:/Users/jingl/.claude/plans/review-groovy-octopus.md | — | ~1270 |
| 15:19 | Created backend/internal/scheduler/calendar.go | — | ~712 |
| 15:21 | Created backend/internal/scheduler/scheduler.go | — | ~3358 |
| 15:23 | Edited backend/internal/scheduler/scheduler_test.go | inline fix | ~19 |
| 15:23 | Edited backend/internal/scheduler/scheduler_test.go | inline fix | ~17 |
| 15:25 | Created backend/internal/scheduler/scheduler.go | — | ~3148 |
| 15:26 | Edited backend/internal/scheduler/calendar.go | modified AddWorkDays() | ~175 |
| 15:28 | Created backend/internal/api/calendar.go | — | ~660 |
| 15:28 | Edited backend/internal/server/server.go | 5→9 lines | ~66 |
| 15:28 | Created frontend/src/utils/date.ts | — | ~361 |
| 15:29 | Edited frontend/src/pages/ProjectGantt.tsx | modified getUserColor() | ~143 |
| 15:29 | Edited frontend/src/pages/ProjectGantt.tsx | CSS: date | ~154 |
| 15:30 | Edited frontend/src/styles/components.css | CSS: background | ~34 |
| 15:32 | Edited frontend/src/pages/ProjectGantt.tsx | inline fix | ~21 |
| 15:34 | Edited backend/internal/api/tasks.go | 5→10 lines | ~97 |
| 15:35 | Session end: 103 writes across 23 files (Dashboard.tsx, components.css, sqlite.go, tasks.go, server.go) | 37 reads | ~75554 tok |
| 15:39 | Edited backend/internal/server/server.go | "[Server] FollowITup v0.7." → "[Server] FollowITup v0.8." | ~19 |
| 15:39 | Edited frontend/src/styles/components.css | inline fix | ~9 |

## Session: 2026-07-30 下午 — v0.8.0

### WBS 层级折叠
- gantt-adapter.ts：`$open: true`
- ProjectGantt.tsx：`open_tree_initially`/`order_branch`/`auto_types` 配置 + `assignee` 列（行内编辑 + `+ 添加任务` 按钮 prompt 输入名称+负责人）
- TaskListView.tsx：`→`/`←` 缩进/升级按钮 + `computeDepths()` 多级缩进
- scheduler.go：TaskInfo 加 ParentID，forwardPass/backwardPass 跳过汇总任务，rollupParentDates() 汇总子任务日期
- tasks.go：parent_id 校验

### 日期格式 v0.8.0
- 数据层保持 YYYY-MM-DD，展示层 Aug 02（甘特图）+ MM/DD/YYYY（列表/看板）
- frontend/src/utils/date.ts：统一格式化工具
- gantt.date_grid="%M %d"，scale_cell_class 周末灰色

### 工作日历 v0.8.0
- migration v3 → calendar 表 (holiday/workday)
- scheduler/calendar.go：IsWorkDay/AddWorkDays/CountWorkDays
- scheduler.go：calcDates 工作日感知（lag=自然日，duration→end_date=工作日）
- api/calendar.go：GET/POST/DELETE /api/calendar
- tasks.go：CreateTask 自动计算工作日 end_date
- 7/31(周五)+3工作日 → 8/4(周二) ✅

### 修复的 bug（今日累计 7+ 个）
- SQLite WAL MaxOpenConns(1)→(4)
- CreateTask SQL 占位符 17→16
- SPA 路由 /project/:id 回退到 index.html
- models time.Time→string（modernc 兼容）
- Gantt 容器 DOM 竞态
- dhtmlx-gantt v10 addTaskLayer/addMarker 兼容
- gantt 事件闭包陈旧 readonly 变量
| 15:40 | Session end: 105 writes across 23 files (Dashboard.tsx, components.css, sqlite.go, tasks.go, server.go) | 37 reads | ~75583 tok |
| 15:44 | Session end: 105 writes across 23 files (Dashboard.tsx, components.css, sqlite.go, tasks.go, server.go) | 37 reads | ~75583 tok |
| 15:46 | Created .gitignore | — | ~65 |
| 15:46 | Created .gitignore | — | ~60 |
| 15:47 | Edited .gitignore | 5→9 lines | ~18 |
| 15:49 | Session end: 108 writes across 24 files (Dashboard.tsx, components.css, sqlite.go, tasks.go, server.go) | 37 reads | ~75736 tok |
| 15:50 | Session end: 108 writes across 24 files (Dashboard.tsx, components.css, sqlite.go, tasks.go, server.go) | 37 reads | ~75736 tok |
| 15:55 | Session end: 108 writes across 24 files (Dashboard.tsx, components.css, sqlite.go, tasks.go, server.go) | 37 reads | ~75736 tok |
| 15:56 | Created start.bat | — | ~34 |
| 15:56 | Session end: 109 writes across 25 files (Dashboard.tsx, components.css, sqlite.go, tasks.go, server.go) | 37 reads | ~75773 tok |

## Session: 2026-07-30 15:59

| Time | Action | File(s) | Outcome | ~Tokens |
|------|--------|---------|---------|--------|
| 16:42 | Created frontend/src/components/TaskDetailModal.tsx | — | ~4914 |
| 16:44 | Created frontend/src/pages/ProjectGantt.tsx | — | ~4340 |
| 16:44 | Created frontend/src/App.tsx | — | ~278 |
| 16:44 | Created frontend/src/pages/ProjectDetail.tsx | — | ~282 |
| 16:44 | Created frontend/src/stores/ganttStore.ts | — | ~1160 |
| 16:45 | Edited frontend/src/styles/components.css | expanded (+189 lines) | ~952 |
| 16:47 | Created backend/internal/api/tasks.go | — | ~2988 |
| 16:48 | Edited frontend/src/styles/components.css | CSS: display, flex-direction, flex | ~83 |
| 16:49 | Edited frontend/src/pages/ProjectGantt.tsx | CSS: actual_start, actual_end | ~259 |
| 16:49 | Edited frontend/src/pages/ProjectGantt.tsx | inline fix | ~33 |
| 16:50 | Created frontend/src/pages/ProjectGantt.tsx | — | ~4320 |
| 16:51 | 甘特图大幅重构：禁用拖拽日期/进度（改为双击弹窗编辑），集成 TaskDetailModal（支持多前置任务批量输入、日期/进度/状态/负责人/约束一站式编辑），添加缩放控件（日月季年），父任务深色粗体+禁止连线，双击连线删除，删除任务自动清理依赖，调度器改为同步执行 | App.tsx ProjectGantt.tsx TaskDetailModal.tsx ProjectDetail.tsx ganttStore.ts tasks.go components.css | 前端+后端完整编译通过，测试全过 | ~15k |
| 16:52 | Session end: 11 writes across 7 files (TaskDetailModal.tsx, ProjectGantt.tsx, App.tsx, ProjectDetail.tsx, ganttStore.ts) | 11 reads | ~41019 tok |
| 17:03 | Edited frontend/src/api/gantt-adapter.ts | inline fix | ~19 |
| 17:04 | Edited frontend/src/components/TaskDetailModal.tsx | added 3 condition(s) | ~170 |
| 17:05 | Edited frontend/src/pages/ProjectGantt.tsx | 12→13 lines | ~157 |
| 17:05 | Edited frontend/src/pages/ProjectGantt.tsx | 6→8 lines | ~78 |
| 17:06 | Created frontend/src/pages/ProjectGantt.tsx | — | ~4204 |
| 17:07 | Edited frontend/src/components/TaskDetailModal.tsx | CSS: display, gap, flex | ~403 |
| 17:07 | Edited frontend/src/styles/components.css | modified not() | ~198 |
| 17:08 | Edited frontend/src/pages/ProjectGantt.tsx | modified function() | ~271 |
| 17:08 | Edited frontend/src/pages/ProjectGantt.tsx | modified function() | ~133 |
| 17:08 | Edited frontend/src/pages/ProjectGantt.tsx | getTask() → hasChild() | ~100 |
| 17:10 | Session end: 21 writes across 8 files (TaskDetailModal.tsx, ProjectGantt.tsx, App.tsx, ProjectDetail.tsx, ganttStore.ts) | 14 reads | ~51958 tok |
| 17:25 | Edited frontend/src/pages/ProjectGantt.tsx | modified function() | ~98 |
| 17:26 | Edited frontend/src/components/TaskDetailModal.tsx | CSS: 0 | ~116 |
| 17:26 | Edited backend/internal/auth/auth.go | modified boolToInt() | ~194 |
| 17:27 | Edited backend/internal/api/auth.go | modified RegisterRoutes() | ~103 |
| 17:27 | Edited backend/internal/api/auth.go | modified ListUsers() | ~415 |
| 17:27 | Created frontend/src/pages/UserManagement.tsx | — | ~1313 |
| 17:28 | Edited frontend/src/App.tsx | added 1 import(s) | ~110 |
| 17:28 | Edited frontend/src/App.tsx | 7→8 lines | ~83 |
| 17:28 | Edited frontend/src/components/Navbar.tsx | expanded (+7 lines) | ~238 |
| 17:28 | Edited frontend/src/components/TaskDetailModal.tsx | CSS: name | ~148 |
| 17:29 | Edited frontend/src/components/TaskDetailModal.tsx | modified if() | ~89 |
| 17:29 | Edited frontend/src/components/TaskDetailModal.tsx | expanded (+7 lines) | ~131 |
| 17:29 | Edited backend/internal/api/auth.go | 2→1 lines | ~10 |
| 17:32 | Session end: 34 writes across 11 files (TaskDetailModal.tsx, ProjectGantt.tsx, App.tsx, ProjectDetail.tsx, ganttStore.ts) | 18 reads | ~58195 tok |

## 2026-07-30 下午（重构）

| 时间 | 描述 | 文件 | 结果 |
|------|------|------|------|
| 16:51 | 甘特图重构：双击弹窗编辑、缩放控件、父任务禁连线、调度器同步 | ProjectGantt.tsx TaskDetailModal.tsx tasks.go 等 | OK |
| 17:03 | allTasksRef+readonlyRef 修复闭包过期 | ProjectGantt.tsx gantt-adapter.ts | OK |
| 17:06 | 弹窗→←缩进升级按钮 | TaskDetailModal.tsx | OK |
| 17:30 | #列、进度数字输入、用户管理页面 | ProjectGantt.tsx UserManagement.tsx auth.go | OK |
| 17:34 | Session end: 34 writes across 11 files (TaskDetailModal.tsx, ProjectGantt.tsx, App.tsx, ProjectDetail.tsx, ganttStore.ts) | 18 reads | ~58195 tok |
