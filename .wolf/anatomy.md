# anatomy.md

> Auto-maintained by OpenWolf. Last scanned: 2026-08-20T09:13:36.784Z
> Files: 95 tracked | Anatomy hits: 0 | Misses: 0

## ../../tmp/


## ./

- `README.md` — Project documentation (~1918 tok)
- `tmp_fix.json` (~110 tok)
- `tmp_import_body.json` (~45 tok)

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

## .superpowers/sdd/2026-08-10-multi-owner/

- `final-fix-report.md` — Final Fix Report: 多负责人功能最终审查修复 (~607 tok)
- `task-1-report.md` — Task 1 报告:迁移 v9(多负责人关联表)+ 迁移测试 (~771 tok)
- `task-10-report.md` — Task 10 Report: 看板创建弹窗多选 + 卡片三行布局 + 待办双视角切换 (~616 tok)
- `task-11-report.md` — ## Task 11 Report: 项目详情页 owner 多选 + 资源视图多负责人分组 (~791 tok)
- `task-12-report.md` — Task 12 Report: 任务列表行内只读 + 甘特图列/过滤/批量条 + CSV 模板提示 (~552 tok)
- `task-13-report.md` — Task 13 Report: 全量构建 + 后端测试 + 浏览器实测 (~899 tok)
- `task-2-report.md` — Task 2 报告:模型字段 + 负责人解析/保存辅助函数 (~980 tok)
- `task-3-report.md` — Task 3 报告: 任务写路径接入多负责人(CreateTask/UpdateTask) (~946 tok)
- `task-4-report.md` — Task 4 报告:任务读路径返回 assignee_ids(ListTasks/GetTask/ListDeletedTasks) (~634 tok)
- `task-4-report.md` — Task 4 报告:任务读路径返回 assignee_ids(ListTasks/GetTask/ListDeletedTasks) (~543 tok)
- `task-5-report.md` — Task 5 报告: GetMyTasks 双视角(view=task|project) (~746 tok)
- `task-6-report.md` — Task 6 报告:CSV 导入负责人列分号多值 (~649 tok)
- `task-7-report.md` — Task 7 Report: 项目多负责人(CreateProject/UpdateProject/ProjectList/GetProject/CopyProject) (~906 tok)
- `task-8-report.md` — Task 8 报告:到期提醒改 JOIN 关联表 (~565 tok)
- `task-9-report.md` — Task 9 Report: MultiUserSelect 组件 + 任务详情弹窗接入 (~514 tok)

## C:/Users/jingl/.claude/plans/

- `inherited-humming-sunset.md` — 英文版 i18n(标题栏语言切换按钮) (~1144 tok)

## C:/Users/jingl/.claude/skills/work-wrap-up/

- `SKILL.md` — 收工 (~587 tok)

## backend/


## backend/cmd/cleanup/


## backend/cmd/fixdata/


## backend/cmd/seed/


## backend/cmd/server/

- `main.go` (~199 tok)

## backend/internal/api/

- `auth.go` — Struct: AuthHandler (~2466 tok)
- `baseline_test.go` — TestFillActualDates, TestCreateBaselineSnapshot, TestBaselineAggregates, TestClearBaseline (~1509 tok)
- `calendar.go` — Struct: CalendarHandler (~1106 tok)
- `helpers.go` — HTTP handlers: writeJSON, writeError (~303 tok)
- `import_test.go` — TestImportTasksStatusWordsAndGuards, TestUpdateTaskPartialUpdateKeepsActual, TestUpdateTaskInvalidAc (~2491 tok)
- `import_test.go` — TestImportTasksStatusWordsAndGuards, TestUpdateTaskPartialUpdateKeepsActual, TestUpdateTaskInvalidAc (~1637 tok)
- `multi_owner_test.go` — TestSplitOwnerNames, TestResolveAndSaveAssignees, TestResolveAndSaveProjectOwners, TestResolveUserID (~9479 tok)
- `projects.go` — Struct: ProjectHandler (~6745 tok)
- `settings.go` — Struct: SettingsHandler (~1026 tok)
- `tasks.go` — Struct: TaskHandler (~10324 tok)

## backend/internal/auth/

- `auth.go` — Struct: Service (~2335 tok)
- `middleware.go` — Struct: Middleware (~956 tok)
- `password_test.go` — TestDeriveDisplayName, TestGenerateRandomPassword (~241 tok)
- `password.go` — GenerateRandomPassword, DeriveDisplayName (~391 tok)

## backend/internal/db/

- `migration_test.go` — TestMigrateV9MultiOwner (~819 tok)
- `sqlite.go` — Struct: DB (~2484 tok)

## backend/internal/mail/

- `mail.go` — Send, SendPasswordReset, SendTemporaryPassword (~537 tok)
- `reminder.go` — Struct: DueTask (~728 tok)

## backend/internal/models/

- `models.go` — Struct: User (~1222 tok)

## backend/internal/scheduler/

- `calendar.go` — Struct: CalendarEntry (~935 tok)
- `scheduler_test.go` — TestShiftDate, TestCalcDuration, TestDetectCycle, TestCalcDatesFS, TestCalcDatesSS (~6259 tok)
- `scheduler.go` — Struct: Dep (~7572 tok)

## backend/internal/server/

- `server.go` — Struct: Options (~1528 tok)

## backend/internal/settings/

- `settings.go` — Get, GetInt, GetAll, Set (~692 tok)

## backend/internal/util/


## backend/internal/ws/


## docs/

- `deployment-windows-server-2022.md` — FollowITup 部署指南(Windows Server 2022 离线环境) (~1198 tok)
- `deployment-windows-server-2022.md` — 部署指南:Win2022离线/NSSM/8081端口/内网SMTP/备份 (~320 tok)

## docs/superpowers/plans/

- `2026-08-05-holiday-range-password-reset.md` — 节假日范围管理 + 管理员密码重置实施计划 (~4534 tok)
- `2026-08-05-recycle-bin.md` — 回收站（已删除任务/项目恢复）实施计划 (~4520 tok)
- `2026-08-05-user-management.md` — 用户管理升级实施计划 (~12021 tok)
- `2026-08-10-multi-owner.md` — 任务与项目多负责人(multi-owner)实施计划 (~17024 tok)

## docs/superpowers/specs/

- `2026-08-05-holiday-range-password-reset-design.md` — 节假日范围管理 + 管理员密码重置设计 (~1118 tok)
- `2026-08-05-recycle-bin-design.md` — 回收站（已删除任务/项目恢复）设计 (~856 tok)
- `2026-08-05-user-management-design.md` — 用户管理升级设计（邮件通知 / 首登改密 / 权限模型 / 系统配置页） (~1643 tok)
- `2026-08-10-multi-owner-design.md` — 任务与项目多负责人(multi-owner)设计 (~1197 tok)
- `2026-08-10-multi-owner-design.md` — 任务与项目多负责人(multi-owner)设计:关联表/双视角待办/三行卡片/CSV分号多值 (~1450 tok)

## frontend/

- `index.html` — FollowITup 项目管理 (~98 tok)

## frontend/src/

- `App.tsx` — App (~485 tok)
- `index.css` — Styles: 7 rules, 21 vars (~654 tok)
- `main.tsx` — 首行 import "./i18n" 确保语言先初始化 (~110 tok)

## frontend/src/api/

- `client.ts` — Declares api (~244 tok)
- `gantt-adapter.ts` — dhtmlx-gantt 数据格式适配层 (~1267 tok)

## frontend/src/components/

- `ImportModal.tsx` — CSV 任务批量导入弹窗 (~1982 tok)
- `MultiUserSelect.tsx` — 多选负责人:已选用户标签(可点 x 移除)+ 下拉勾选列表(点击 toggle,去重) (~868 tok)
- `Navbar.tsx` — 切换语言后整页刷新：gantt 的 scale/tooltip/缩放标签在初始化时固化，reload 是最稳妥的二次初始化 (~616 tok)
- `RecycleBinModal.tsx` — RecycleBinModal (~1112 tok)
- `TaskDetailModal.tsx` — 快速添加前置任务：解析逗号/分号分隔的行号并逐个创建依赖 (~7266 tok)

## frontend/src/i18n/

- `index.ts` — i18n 初始化(LANGS/LANG_KEY/setLanguage/html lang+title 同步/整页 reload 策略) (~500 tok)
- `index.ts` — i18n 初始化：i18next + react-i18next，双语（zh/en），localStorage 持久化。 (~417 tok)
- `locales/en.ts` — 英文翻译表 (~2000 tok)
- `locales/zh.ts` — 中文翻译表全量(common/nav/status/errors 21码/dashboard/gantt/各页面 ~600 键) (~2000 tok)

## frontend/src/i18n/locales/

- `en.ts` — English translation table (~4948 tok)
- `zh.ts` — 中文翻译表(简体中文,默认语言) (~3576 tok)

## frontend/src/pages/

- `ChangePassword.tsx` — ChangePassword — renders form (~826 tok)
- `Dashboard.tsx` — Dashboard (~9999 tok)
- `Login.tsx` — Login — renders form (~601 tok)
- `ProjectDetail.tsx` — ProjectDetail (~1541 tok)
- `ProjectGantt.tsx` — ref 存储最新 allTasks，避免 useEffect 闭包捕获过期值 (~16720 tok)
- `Resources.tsx` — 资源视图：按负责人分组汇总任务（叶子任务计入，父任务由子任务汇总不重复） (~1368 tok)
- `SystemSettings.tsx` — SystemSettings — renders table (~3979 tok)
- `TaskListView.tsx` — 为任务列表计算每行的可视化深度（递归查找 parent chain） (~3425 tok)
- `UserManagement.tsx` — UserManagement — renders form, table (~2810 tok)

## frontend/src/stores/

- `authStore.ts` — API routes: POST (1 endpoints) (~460 tok)
- `dashboardStore.ts` — API routes: GET (2 endpoints) (~552 tok)
- `ganttStore.ts` — 聚焦信息：某用户正在查看/编辑某任务 (~1661 tok)
- `settingsStore.ts` — 财年起始月从系统配置读取（管理员在系统设置页修改），不再本地存储 (~716 tok)

## frontend/src/styles/

- `components.css` — Styles: 93 rules, 1 vars (~10666 tok)

## frontend/src/utils/

- `date.ts` — 统一日期格式化 — 数据层始终 YYYY-MM-DD，仅展示层转换 (~347 tok)
- `errorMsg.ts` — 错误消息统一处理：前端按后端 error.code 映射翻译，查不到回退后端中文消息。 (~279 tok)
- `labels.ts` — 状态/优先级统一标签（各组件共用，消除 5 处重复映射；措辞统一：待开始/进行中/已完成/已延期） (~100 tok)
