# anatomy.md

> Auto-maintained by OpenWolf. Last scanned: 2026-08-05T01:58:59.442Z
> Files: 13 tracked | Anatomy hits: 0 | Misses: 0

## ../../tmp/


## ./


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


## backend/


## backend/cmd/cleanup/


## backend/cmd/fixdata/


## backend/cmd/seed/


## backend/cmd/server/


## backend/internal/api/

- `projects.go` — Struct: ProjectHandler，含项目回收站端点 ListProjects(?deleted=1)/RestoreProject (~3840 tok)
- `tasks.go` — Struct: TaskHandler (~4843 tok)

## backend/internal/auth/


## backend/internal/db/


## backend/internal/models/


## backend/internal/scheduler/


## backend/internal/server/


## backend/internal/util/


## backend/internal/ws/


## docs/


## docs/superpowers/plans/

- `2026-08-05-recycle-bin.md` — 回收站（已删除任务/项目恢复）实施计划 (~4520 tok)

## docs/superpowers/specs/

- `2026-08-05-recycle-bin-design.md` — 回收站（已删除任务/项目恢复）设计 (~836 tok)

## frontend/


## frontend/src/


## frontend/src/api/


## frontend/src/components/

- `RecycleBinModal.tsx` — RecycleBinModal (~955 tok)

## frontend/src/pages/

- `Dashboard.tsx` — Dashboard (~5056 tok)
- `ProjectGantt.tsx` — ref 存储最新 allTasks，避免 useEffect 闭包捕获过期值 (~8017 tok)

## frontend/src/stores/


## frontend/src/styles/

- `components.css` — Styles: 100 rules (~7076 tok)

## frontend/src/utils/

