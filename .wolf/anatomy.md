# anatomy.md

> Auto-maintained by OpenWolf. Last scanned: 2026-08-21T08:23:36.252Z
> Files: 25 tracked | Anatomy hits: 0 | Misses: 0

## ../../../../.claude/skills/work-wrap-up/


## ../../tmp/


## ../ndm/.claude/skills/work-wrap-up/

- `SKILL.md` — 收工 (~700 tok)

## ./

- `build.bat` (~211 tok)

## .claude/


## .claude/agents/


## .claude/rules/


## .claude/skills/ldap-sync/


## .claude/skills/project-status/


## .superpowers/sdd/2026-08-03-baseline-comparison/


## .superpowers/sdd/2026-08-04-schedule-direction/


## .superpowers/sdd/2026-08-05-recycle-bin/


## .superpowers/sdd/2026-08-10-multi-owner/


## C:/Users/jingl/.claude/plans/

- `dazzling-plotting-conway.md` — 任务计划/实际日期联动 + 工期体系重构 (~1462 tok)

## C:/Users/jingl/.claude/skills/work-wrap-up/

- `SKILL.md` — 收工 (~700 tok)

## backend/


## backend/cmd/cleanup/


## backend/cmd/fixdata/


## backend/cmd/seed/


## backend/cmd/server/


## backend/internal/api/

- `baseline_test.go` — TestCreateBaselineSnapshot, TestBaselineAggregates, TestClearBaseline (~1221 tok)
- `import_test.go` — TestImportTasksStatusWordsAndGuards, TestUpdateTaskPartialUpdateKeepsActual, TestUpdateTaskInvalidAc (~2484 tok)
- `projects.go` — Struct: ProjectHandler (~7263 tok)
- `tasks.go` — Struct: TaskHandler (~10403 tok)

## backend/internal/auth/


## backend/internal/db/

- `sqlite.go` — Struct: DB (~2521 tok)

## backend/internal/mail/


## backend/internal/models/

- `models.go` — Struct: User (~1237 tok)

## backend/internal/scheduler/

- `scheduler_test.go` — TestShiftDate, TestCalcDuration, TestDetectCycle, TestCalcDatesFS, TestCalcDatesSS (~6593 tok)
- `scheduler.go` — Struct: Dep (~7750 tok)

## backend/internal/server/


## backend/internal/settings/


## backend/internal/util/


## backend/internal/ws/


## docs/


## docs/superpowers/plans/


## docs/superpowers/specs/


## frontend/

- `tsconfig.app.json` (~196 tok)

## frontend/src/

- `index.css` — Styles: 8 rules, 21 vars (~715 tok)

## frontend/src/api/

- `gantt-adapter.ts` — dhtmlx-gantt 数据格式适配层 (~1232 tok)

## frontend/src/components/

- `Navbar.tsx` — 切换语言后整页刷新：gantt 的 scale/tooltip/缩放标签在初始化时固化，reload 是最稳妥的二次初始化 (~663 tok)
- `TaskDetailModal.tsx` — 正排：编辑计划开始 = 写入 start_no_earlier_than 约束（引擎取 max(前置, 约束)，天然满足"不能早于"校验） (~9578 tok)

## frontend/src/i18n/


## frontend/src/i18n/locales/

- `en.ts` — English translation table (~5295 tok)
- `zh.ts` — 中文翻译表(简体中文,默认语言) (~3785 tok)

## frontend/src/pages/

- `Dashboard.tsx` — Dashboard (~10266 tok)
- `ProjectGantt.tsx` — ref 存储最新 allTasks，避免 useEffect 闭包捕获过期值 (~17014 tok)
- `TaskListView.tsx` — 为任务列表计算每行的可视化深度（递归查找 parent chain） (~3417 tok)

## frontend/src/stores/

- `dashboardStore.ts` — API routes: GET (2 endpoints) (~565 tok)

## frontend/src/styles/

- `components.css` — Styles: 85 rules, 1 vars (~11235 tok)

## frontend/src/utils/

- `date.ts` — 统一日期格式化 — 数据层始终 YYYY-MM-DD，仅展示层转换 (~893 tok)
