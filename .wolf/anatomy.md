# anatomy.md

> Auto-maintained by OpenWolf. Last scanned: 2026-08-07T01:55:05.709Z
> Files: 53 tracked | Anatomy hits: 0 | Misses: 0

## ../../tmp/


## ./

- `README.md` — Project documentation (~1673 tok)

## .claude/


## .claude/agents/


## .claude/rules/


## .claude/skills/ldap-sync/


## .claude/skills/project-status/


## .superpowers/sdd/2026-08-03-baseline-comparison/


## .superpowers/sdd/2026-08-04-schedule-direction/


## .superpowers/sdd/2026-08-05-recycle-bin/

- `task-1-report.md` — Task 1 报告：后端任务回收站端点（列表 + 恢复） (~583 tok)
- `task-2-report.md` — Task 2 报告：后端项目回收站端点（列表 + 恢复） (~518 tok)
- `task-3-report.md` — Task 3 Report: 前端项目页回收站 (~319 tok)
- `task-4-report.md` — Task 4 Report: 前端首页回收站（Dashboard 入口 + 弹窗） (~446 tok)
- `task-5-report.md` — Step 1: 后端测试 + 前端类型检查 (~611 tok)

## C:/Users/jingl/.claude/plans/

- `inherited-humming-sunset.md` — 用户测试就绪增强包(批量) (~836 tok)

## backend/


## backend/cmd/cleanup/


## backend/cmd/fixdata/


## backend/cmd/seed/


## backend/cmd/server/


## backend/internal/api/

- `auth.go` — Struct: AuthHandler (~2466 tok)
- `calendar.go` — Struct: CalendarHandler (~1106 tok)
- `helpers.go` — HTTP handlers: writeJSON, writeError (~303 tok)
- `projects.go` — Struct: ProjectHandler (~4481 tok)
- `settings.go` — Struct: SettingsHandler (~917 tok)
- `tasks.go` — Struct: TaskHandler (~6741 tok)

## backend/internal/auth/

- `auth.go` — Struct: Service (~2335 tok)
- `middleware.go` — Struct: Middleware (~956 tok)
- `password_test.go` — TestDeriveDisplayName, TestGenerateRandomPassword (~241 tok)
- `password.go` — GenerateRandomPassword, DeriveDisplayName (~391 tok)

## backend/internal/db/

- `sqlite.go` — Struct: DB (~1947 tok)

## backend/internal/mail/

- `mail.go` — Send, SendPasswordReset, SendTemporaryPassword (~474 tok)

## backend/internal/models/

- `models.go` — Struct: User (~1182 tok)

## backend/internal/scheduler/

- `calendar.go` — Struct: CalendarEntry (~935 tok)
- `scheduler_test.go` — TestShiftDate, TestCalcDuration, TestDetectCycle, TestCalcDatesFS, TestCalcDatesSS (~5291 tok)
- `scheduler.go` — Struct: Dep (~7082 tok)

## backend/internal/server/

- `server.go` — Struct: Options (~1460 tok)

## backend/internal/settings/

- `settings.go` — Get, GetInt, GetAll, Set (~592 tok)

## backend/internal/util/


## backend/internal/ws/


## docs/


## docs/superpowers/plans/

- `2026-08-05-holiday-range-password-reset.md` — 节假日范围管理 + 管理员密码重置实施计划 (~4534 tok)
- `2026-08-05-recycle-bin.md` — 回收站（已删除任务/项目恢复）实施计划 (~4520 tok)
- `2026-08-05-user-management.md` — 用户管理升级实施计划 (~12021 tok)

## docs/superpowers/specs/

- `2026-08-05-holiday-range-password-reset-design.md` — 节假日范围管理 + 管理员密码重置设计 (~1118 tok)
- `2026-08-05-recycle-bin-design.md` — 回收站（已删除任务/项目恢复）设计 (~856 tok)
- `2026-08-05-user-management-design.md` — 用户管理升级设计（邮件通知 / 首登改密 / 权限模型 / 系统配置页） (~1643 tok)

## frontend/


## frontend/src/

- `App.tsx` — App (~454 tok)
- `index.css` — Styles: 4 rules, 20 vars (~552 tok)

## frontend/src/api/

- `client.ts` — Declares api (~244 tok)
- `gantt-adapter.ts` — dhtmlx-gantt 数据格式适配层 (~1240 tok)

## frontend/src/components/

- `ImportModal.tsx` — CSV 任务批量导入弹窗 (~1608 tok)
- `Navbar.tsx` — Navbar (~374 tok)
- `RecycleBinModal.tsx` — RecycleBinModal (~955 tok)
- `TaskDetailModal.tsx` — 快速添加前置任务：解析逗号/分号分隔的行号并逐个创建依赖 (~6707 tok)

## frontend/src/pages/

- `ChangePassword.tsx` — ChangePassword — renders form (~826 tok)
- `Dashboard.tsx` — Dashboard (~7755 tok)
- `Login.tsx` — Login — renders form (~609 tok)
- `ProjectDetail.tsx` — ProjectDetail (~1317 tok)
- `ProjectGantt.tsx` — ref 存储最新 allTasks，避免 useEffect 闭包捕获过期值 (~13242 tok)
- `SystemSettings.tsx` — SystemSettings — renders table (~2939 tok)
- `TaskListView.tsx` — 为任务列表计算每行的可视化深度（递归查找 parent chain） (~3528 tok)
- `UserManagement.tsx` — UserManagement — renders form, table (~2451 tok)

## frontend/src/stores/

- `authStore.ts` — API routes: POST (1 endpoints) (~460 tok)
- `dashboardStore.ts` — API routes: GET (2 endpoints) (~543 tok)
- `ganttStore.ts` — 聚焦信息：某用户正在查看/编辑某任务 (~1653 tok)
- `settingsStore.ts` — 财年起始月从系统配置读取（管理员在系统设置页修改），不再本地存储 (~716 tok)

## frontend/src/styles/

- `components.css` — Styles: 94 rules, 1 vars (~8511 tok)

## frontend/src/utils/

- `date.ts` — 统一日期格式化 — 数据层始终 YYYY-MM-DD，仅展示层转换 (~336 tok)
