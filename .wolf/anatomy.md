# anatomy.md

> Auto-maintained by OpenWolf. Last scanned: 2026-07-30T09:29:57.120Z
> Files: 68 tracked | Anatomy hits: 0 | Misses: 0

## ../../tmp/

- `create-project.json` (~34 tok)

## ./

- `.gitignore` — Git ignore rules (~69 tok)
- `build.bat` (~166 tok)
- `CLAUDE.md` — CLAUDE.md (~930 tok)
- `config.yaml.example` (~98 tok)
- `README.html` — FollowITup — SmartSheet-like Project Management (~2529 tok)
- `README.md` — Project documentation (~1488 tok)
- `start.bat` (~34 tok)

## .claude/

- `settings.json` — Declares f (~532 tok)

## .claude/agents/

- `gantt-tester.md` — 测试场景 (~259 tok)
- `security-reviewer.md` — 审查清单 (~239 tok)

## .claude/rules/

- `openwolf.md` (~313 tok)

## .claude/skills/ldap-sync/

- `SKILL.md` — LDAP/AD 用户同步 (~259 tok)

## .claude/skills/project-status/

- `SKILL.md` — 项目周报生成 (~161 tok)

## C:/Users/jingl/.claude/plans/

- `review-groovy-octopus.md` — 日期格式 + 工作日历方案 (~1191 tok)
- `smartsheet-windows-cryptic-hennessy.md` — SmartSheet-Like 项目管理系统 — 实施计划 (~2629 tok)

## backend/

- `config.yaml` (~125 tok)

## backend/cmd/cleanup/

- `main.go` (~139 tok)

## backend/cmd/fixdata/

- `main.go` — Struct: item (~531 tok)

## backend/cmd/seed/

- `enddate.go` (~228 tok)
- `finalize.go` (~223 tok)
- `fix.go` (~168 tok)
- `main.go` — Struct: taskDef (~858 tok)
- `query.go` (~202 tok)
- `schedule.go` (~248 tok)
- `tree.go` — Struct: change (~403 tok)
- `verify.go` — Struct: taskRec (~392 tok)

## backend/cmd/server/

- `config.go` — Struct: Config (~382 tok)
- `main.go` (~140 tok)

## backend/internal/api/

- `auth.go` — Struct: AuthHandler (~1083 tok)
- `calendar.go` — Struct: CalendarHandler (~660 tok)
- `helpers.go` — HTTP handlers: writeJSON, writeError (~213 tok)
- `projects.go` — Struct: ProjectHandler (~2470 tok)
- `tasks.go` — Struct: TaskHandler (~2988 tok)

## backend/internal/auth/

- `auth.go` — Struct: Service (~1612 tok)
- `middleware.go` — Struct: Middleware (~820 tok)

## backend/internal/db/

- `sqlite.go` — Struct: DB (~1587 tok)

## backend/internal/models/

- `models.go` — Struct: User (~1027 tok)

## backend/internal/scheduler/

- `calendar.go` — Struct: CalendarEntry (~739 tok)
- `scheduler_test.go` — TestShiftDate, TestCalcDuration, TestDetectCycle, TestCalcDatesFS, TestCalcDatesSS (~2315 tok)
- `scheduler.go` — Struct: Dep (~3148 tok)

## backend/internal/server/

- `config.go` — Struct: Config (~427 tok)
- `server.go` — Struct: Options (~1155 tok)

## backend/internal/util/

- `fiscal_test.go` — TestFiscalYearRange, TestFiscalYearFromDate, TestAvailableFiscalYears, TestFiscalYearLabel, TestCale (~699 tok)
- `fiscal.go` — FiscalYearRange, FiscalYearFromDate, CurrentFiscalYear, AvailableFiscalYears, FiscalYearLabel (~744 tok)

## backend/internal/ws/

- `hub.go` — Struct: Message (~1468 tok)

## docs/

- `design-requirements.md` — 综合报告看板 — 设计需求文档 (~2407 tok)

## frontend/

- `vite.config.ts` — https://vite.dev/config/ (~106 tok)

## frontend/src/

- `App.tsx` — App (~313 tok)
- `index.css` — Styles: 3 rules, 10 vars (~426 tok)
- `main.tsx` (~93 tok)

## frontend/src/api/

- `client.ts` — Declares api (~170 tok)
- `gantt-adapter.ts` — dhtmlx-gantt 数据格式适配层 (~1052 tok)
- `ws-client.ts` — WebSocket 客户端 — 实时协作 (~814 tok)

## frontend/src/components/

- `Navbar.tsx` — Navbar (~318 tok)
- `TaskDetailModal.tsx` — 快速添加前置任务：解析逗号/分号分隔的 ID 并逐个创建依赖 (~5372 tok)

## frontend/src/pages/

- `Dashboard.tsx` — Dashboard — renders form (~3654 tok)
- `Login.tsx` — Login — renders form (~582 tok)
- `ProjectDetail.tsx` — ProjectDetail (~282 tok)
- `ProjectGantt.tsx` — ref 存储最新 allTasks，避免 useEffect 闭包捕获过期值 (~4224 tok)
- `TaskListView.tsx` — 🗑️ 已废弃（甘特图统一工作台取代） (~3323 tok)
- `UserManagement.tsx` — UserManagement — renders form, table (~1313 tok)

## frontend/src/stores/

- `authStore.ts` — API routes: POST (1 endpoints) (~388 tok)
- `dashboardStore.ts` — API routes: GET (2 endpoints) (~533 tok)
- `ganttStore.ts` — 聚焦信息：某用户正在查看/编辑某任务 (~1160 tok)
- `settingsStore.ts` — 根据日期和财年起始月计算财年编号 (~799 tok)

## frontend/src/styles/

- `components.css` — Styles: 111 rules (~4992 tok)

## frontend/src/utils/

- `date.ts` — 统一日期格式化 — 数据层始终 YYYY-MM-DD，仅展示层转换 (~361 tok)
