# Cerebrum

> OpenWolf's learning memory. Updated automatically as the AI learns from interactions.
> Do not edit manually unless correcting an error.
> Last updated: 2026-07-28

## User Preferences

<!-- How the user likes things done. Code style, tools, patterns, communication. -->

## Key Learnings

- **Project:** followITup

## Do-Not-Repeat

<!-- Mistakes made and corrected. Each entry prevents the same mistake recurring. -->
<!-- Format: [YYYY-MM-DD] Description of what went wrong and what to do instead. -->
- **[2026-08-20] Go nil slice JSON 序列化为 null，前端 .length/.map 直读必崩（bug-239）**：`var mine []MyTaskItem` 空数据时返回 `"mine": null`。新部署后 admin 无待办 → Dashboard 渲染 `myTodo.mine.length` → TypeError → React 18 无错误边界卸载整树 → 全白板。凡 writeJSON 直接返回的 slice 一律 `make([]T, 0)` 保证输出 `[]`（GetMyTasks 的 mine/starting、ProjectList 已修；tasks.go:78 ListTasks、baseline.go:149、calendar.go:55、projects.go:688 等同类风险点待排查）。前端对 `res.data.data` 必须逐字段 `?? []` 兜底，不能只兜 data 层。**另注意**：时间敏感测试不能用固定日期——starting 窗口 [今天, 今天+7] 的测试数据必须用 time.Now() 相对日期（multi_owner_test/import_test 的 2026-08-12/14 已过期修复）。
- **[2026-08-10] fieldCheck 键存在判定过宽导致静默数据覆盖**：`fieldCheck["xxx"]` 仅凭键存在就执行副作用(改派/写关联表),不比较新旧值是否一致——前端展开对象时总是携带所有键,导致每次无关字段变更都触发副作用(bug-238)。修改集合性字段(owner_ids/assignee_ids)时**必须先取旧值比较(集合语义),仅真正变化时才执行副作用**。
- **[2026-08-10] INSERT...SELECT 空表静默 0 行**：`INSERT INTO t (...) SELECT ... FROM src WHERE ...` 在 src 无匹配行时插入 0 行且 Exec 不报错（err=nil），计数假增数据全丢（bug-173，CSV 导入空项目）。凡插入依赖源表行数的写法，源表可能为空时改用 VALUES 直插 + 单独查基数（如 MAX(sort_order)）。
- **[2026-07-29] 财年范围计算**：`FiscalYearRange()` 中，自然年（startMonth=1）的起始日历年 = `2000+fiscalYear`（FY27→2027），跨年财年（startMonth>1）的起始日历年 = `2000+fiscalYear-1`（FY27→2026）。自然年的结束日期是同年 12-31 而非次年。`FiscalYearFromDate()` 中，自然年直接 `year-2000`，不因月份做 +1 调整。
- **[2026-07-29] 排程多前置取max**：`forwardPass()` 中遍历后继依赖时，原先用 `candidateStart == succ.StartDate` 判断是否跳过，多前置场景下后处理的早期前置会覆盖先处理的晚期前置的结果。修复为 `candidateStart > succ.StartDate` 才更新（只推后不拉前），且队列入队移到 if 外确保始终传播。
- **[2026-07-29] 约束倒推排程**：tasks 表新增 `constraint_type`（''/start_no_earlier_than/finish_no_later_than）和 `constraint_date` 列。前向传播整合 SNET 约束（候选日期取 max(前置候选, 约束日期)）。后向传播从叶子任务和 deadline 约束出发逆推 LS/LF。TF = LS - ES，TF < 0 表示约束冲突。calcPredecessorLF 根据 4 种依赖类型反推前驱必须满足的完成/开始日期。
- **[2026-07-30] SQLite WAL 连接池**：`modernc.org/sqlite` 使用 WAL 模式时 `SetMaxOpenConns(1)` 会导致写操作后读请求永久挂起（curl exit 28）。WAL 需要至少 2 个连接（1 读 + 1 写 checkpoint）。改为 `SetMaxOpenConns(4)` + `SetMaxIdleConns(2)`。
- **[2026-07-30] modernc.org/sqlite 不能 scan time.Time**：`database/sql` 的 `rows.Scan` 不支持将 TEXT 列扫描到 `time.Time`。所有模型的 CreatedAt/UpdatedAt 必须使用 `string` 类型，JSON 序列化也返回字符串。
- **[2026-07-30] SQLite WAL 连接锁**：在同一个 `sql.DB` 连接上，如果 SELECT rows 未关闭就执行 UPDATE，会导致 SQLITE_BUSY。正确做法：先用 `rows.Scan` 收集数据到 slice → `rows.Close()` → 再执行写操作。
- **[2026-07-30] dhtmlx-gantt v10 事件不可靠**：`onAfterTaskAdd` / `onTaskCreated` 事件在 Community Edition v10 中不触发，即使 `attachEvent` 返回 handler ID。改用 React 按钮直接调 API + fetchData 刷新，绕开 gantt 事件系统。
- **[2026-07-30] React 甘特图容器竞态**：`gantt.init()` 需要 container DOM 已挂载。如果组件在 loading 状态返回不同的 JSX（不含 container div），useEffect([]) 运行时 containerRef 为 null，gantt 永远不初始化。修复：始终渲染 container div（用 display 控制可见性）。
- **[2026-07-30] dhtmlx-gantt $rendered_type 不可靠**：当在数据中显式设置 `type: "task"` 时，dhtmlx 不会自动检测该任务有子任务而设 `$rendered_type = "project"`。改用 `gantt.hasChild(id)` 运行时判断更可靠。
- **[2026-07-30] useEffect 闭包捕获过期值**：gantt 初始化 useEffect([]) 内的事件处理器捕获初始 `allTasks` 状态（空数组），后续更新触发的双击事件找不到任务。修复：用 `useRef(allTasksRef)` 存储最新值，事件处理器从 ref.current 读取。
- **[2026-07-30] 前端产物复制顺序**：build.bat 和手动构建都必须先 `npm run build` → `cp -r frontend/dist backend/cmd/server/frontend-dist` → `go build`。如果先 `go build` 再复制前端产物，嵌入式二进制仍包含旧前端。
- **[2026-07-30] onLinkDblClick setTimeout**：dhtmlx 内部状态在事件回调中尚未更新完毕，直接调用 `deleteLink()` 会报 `Cannot read properties of undefined (reading 'id')`。修复：`setTimeout(() => deleteLink(linkId), 50)` 推迟到 dhtmlx 内部状态落定后执行。
- **[2026-08-03] UpdateTask 必须回填 URL 参数**：`UpdateTask` 的 `t` 来自请求体 decode（id/project_id 不在请求体内），调用 `Recalculate(h.db, t.ProjectID, t.ID)` 前必须 `t.ID = taskID; t.ProjectID = projectID`，否则收到 (0,0) → 级联排程静默失效。CreateTask 已有此赋值（174-175 行），UpdateTask 曾遗漏（bug-024）。
- **[2026-08-03] 动态 SQL 拼接的 filter 需要表别名**：`buildTimeFilter(tableAlias, ...)` 生成 `p.created_at` 引用，凡拼接该 filter 的查询 `FROM` 必须带别名 `p`（如 `FROM projects p`）。无别名查询会报 `no such column` 且错误被 `QueryRow.Scan` 忽略 → 统计静默为 0（bug-023）。
- **[2026-08-03] 前向传播跳过父任务**：`forwardPass` 中 `parentSet[succ.ID]`（有子任务的任务）被跳过，父任务日期由 `rollupParentDates` 汇总子任务范围维护。验证级联时不能选父任务做触发器/后继——选叶子链路（如 37→38→39）。
- **[2026-08-03] PUT /tasks 是全量更新**：请求体缺字段会把 DB 值清零（duration_days/progress_pct/status 等）。前端 TaskDetailModal 保存时传完整对象没问题，curl/脚本调用必须带全字段，否则会破坏数据（27 的 duration 被清 0 即此因）。

| 2026-08-05 | 调用 API 创建任务前必须确认 projectID 非空（shell 变量提取失败会把任务创建到 project_id=0 的孤儿项目，前端不可见需清理） |
## Decision Log

<!-- Significant technical decisions with rationale. Why X was chosen over Y. -->
- **[2026-07-29] 财年实现策略**：财年/自然年作为展示层过滤器，不改变数据层。后端 API 同时支持 `?year=`（自然年，strftime）和 `?fy=`（财年，BETWEEN 日期范围）。前端通过 localStorage 持久化用户的显示模式偏好，Dashboard 年度选择器根据模式动态切换标签（2026年 / FY27）。财年起始月在 `config.yaml` 中配置（`fiscal.year_start_month`）。
- **[2026-07-29] 协作感知实现策略**：类似 Excel 协同编辑的选中可见性，而非硬锁定。前端通过 WebSocket 发送 task_focus/task_blur 消息，后端 broadcastExcept 广播给房间内其他用户（排除发送者）。甘特图通过 `gantt.addTaskLayer()` 渲染聚焦标签（用户名+色条）。TaskHandler 注入 Hub，所有写操作后调用 `BroadcastTaskUpdate` 通知其他客户端刷新。旧消息超时 15 秒自动清除。

## User Preferences

- **UI风格偏好**：暖白底色（参考 s2.jpg）+ 潭绿冷绿点缀的冷静配色；不要纯黑高对比、不要过于花哨的仪表盘风格；密度紧凑合适，可容纳 5+ 项目
- **配色决策流程**：先让 ui-ux-pro-max 生成候选设计系统（python scripts/search.py --design-system），再让 frontend-design 批判筛选以避免 AI 模板化外观

- **[2026-07-31] 自动缩放算法**：从最细档位遍历，找到第一个"项目总像素宽度 ≤ 可用图表宽度"的级别，确保首次打开无滚动条完整显示全部任务。手动放大后允许滚动条。
- **[2026-07-31] dhtmlx-gantt 双击与 render 冲突**：`onTaskClick` 内调用 `gantt.render()` 会重置内部"第一次点击"状态导致双击永远无法触发。解决方案：通过 React state 触发 useEffect 延迟 render，避免在 click 事件处理器内同步调用 render。
- **[2026-07-31] dhtmlx-gantt 行高密度**：`row_height: 28` + `scale_height: 40` + `min_column_width: 40` 可在 1920px 屏幕显示约 36 行任务。
- **[2026-08-03] 看板进度语义**：整体完成率与项目进度统一为"顶层任务时长加权"(SUM(duration_days×progress_pct)/NULLIF(SUM(duration_days),0),过滤 parent_id IS NULL OR 0),不再用 AVG(progress_pct) 简单平均。子任务进度通过父任务体现:recalcParentProgress 递归维护父任务进度,rollupParentDates 维护父任务 duration_days(子任务区间工作日数),两层配合使加权计算有意义。
- **[2026-08-03] GitNexus detect_changes 风险解读**：risk=critical 是改动符号数量驱动(本次 36 个符号全部是 diff 内入口符号),不代表破坏性。判断是否安全看 affected_processes 的 changed_steps:全部为 step 1(改动符号即流程入口)则无下游破坏。
- **[2026-08-03] Windows 终端 curl 写中文会污染数据库**：Git Bash/GBK 终端里 `curl -X PUT -d '{"name":"中文"}'` 发送的字节是 GBK 编码，Go 后端按 UTF-8 解析成替换字符（U+FFFD）入库，导致甘特图中文乱码。测试/脚本写中文必须：用 UTF-8 文件 + `curl --data-binary @file.json`，或 python -c 直接操作。产品代码无 bug（bug-025）。
- **[2026-08-03] UpdateTask 是全列覆盖 UPDATE**：PUT /tasks 只传部分字段会清空其余列（name/日期/进度等）。排序等单字段更新必须走专用 PATCH 端点（/tasks/{id}/sort_order），不能用 PUT。
- **[2026-08-03] # 列行号语义**：甘特图 # 列显示 `task.$index + 1`（dhtmlx 全树深度优先 0 基索引，渲染前赋值），与数据库 id 解耦；拖拽全局重排后行号自动连续。弹窗"前置任务快速添加"的 ID 提示已与行号不对应，待后续改按行号解析。

## Key Learnings
- gitnexus detect_changes 对新增 handler/符号常报行号偏移误报（把相邻函数标为 touched，风险 HIGH/MEDIUM）：新增代码不修改既有符号时，先人工核对 diff 再决定是否警示，不要盲信风险等级（2026-08-05，回收站 Task 1/4 两次出现）
- SDD 审查中发现"Important 但实为文档建议"的项：协调者可按审查者自析结论裁决为 Minor deferred，不必为无真实缺陷的建议跑完整 fix loop（2026-08-05，Task 2 RestoreProject auto-commit 时序）
- **[2026-08-10] 部分更新保护模式**：PUT 全量语义下某字段"零值=合法值"时（如 actual_* 空串），先 io.ReadAll 整读 body → 两次 json.Unmarshal（struct + map[string]json.RawMessage）→ 键缺失时查 DB 旧值保留，防零值静默覆盖用户数据。
- **[2026-08-10] i18next 三个坑**：(1) init 完成时不触发 languageChanged 事件，html lang/title 同步须在 init 后手动调一次；(2) 数字开头的对象键（如 `zoom.3days`）会被 i18next 解析异常返回键名，改用数组索引键（`zoom.1`）；(3) dhtmlx-gantt 的 `%a` 是 am/pm 标记而非星期缩写，星期用 `%D`（locale days_short 驱动）。另外：Git Bash heredoc 会把 `\\n` 折叠成 `\n`，python 脚本里匹配含反斜杠的文本要用 r-string 或 chr(92)；React 组件里 map 回调参数与 useTranslation 的 t 同名会遮蔽（TS2349），翻译函数改名 tr。
- **[2026-08-10] i18n 架构决策**：用户选定 i18next + react-i18next；语言切换后整页 reload（gantt 初始化时固化 scale/tooltip/缩放标签，无干净二次初始化路径）；后端错误消息保持中文，前端按 error.code 映射（getErrorMessage 回退链：errors.<CODE> → 后端 message → 网络 err.message → fallbackKey）；邮件模板只用英文；localStorage key "followitup-lang"。

## Do-Not-Repeat
- **[2026-08-05] go build 嵌入目录 + 管道吞退出码**：embed 指令 `//go:embed all:frontend-dist` 在 `backend/cmd/server/main.go`，嵌入目录是 `backend/cmd/server/frontend-dist`（相对 .go 文件），不是 `backend/frontend-dist`！前端产物必须复制到 `cmd/server/frontend-dist`。构建命令严禁 `go build | tail` 之类管道（管道退出码是 tail 的，go build 失败被静默）；exe 被运行进程占用时 Windows 下构建会失败，必须先杀进程再 build。
- **[2026-08-05] 页面无变化先查 bundle 文件名**：前端改了不生效时，先比对「页面加载的 script src」vs「exe 内嵌的 index-*.js 文件名」(grep -a -o)，再比对 exe 嵌入 vs cmd/server/frontend-dist 磁盘内容，快速定位是哪一层旧了。

## Decision Log
- **[2026-08-05] 甘特连线画法(用户定义,已确认完美)**：
  - 单前置：5 段折线——源条右缘 → 右 20px → 垂直下到"空隙中央"（(sy+ty)/2，自动避让中间条）→ 水平左到目标左缘外 20px → 垂直下到目标中线 → 水平连入，箭头尖端正好落条左缘。
  - 多前置（同目标多个源）：合并画法——各源水平线先汇合到**公共右边界**（时间最长的条右缘）+20px，垂直下折到**公共下边界**（任务列表最下面的源底边）与目标条顶部之间的空隙中央，共享水平段/垂直段/**一个箭头**连入目标。
  - 硬约束：任何线段不得侵入甘特条（贴边/端点接触允许）。空隙中央选择 = 所有条 y 区间并集补集（空隙带）中，最接近 (lo+hi)/2 且所有线段不穿条的中央。
  - 实现要点：垂直段 x 是单点、水平段 y 是单点，侵入检测对单点必须用"严格在条内"（区间真重叠对单点失效）；多前置共享箭头，双击删除整组依赖。

- **[2026-08-05] 版本号规则(用户定义，2026-08-10 作废)**：版本号 = 0 + 月 + 日，当前为开发版。如 8月5日 = v0.8.5，9月15日 = v0.9.15。代码位置：backend/internal/server/server.go 日志 + README.md 顶部。
- **[2026-08-10] 版本号规则 v2(用户定义)**：进入用户测试阶段，版本号改为 **v1.8.10**(主版本 1 + 原日期号)，不再每日递增。位置：server.go 日志 + README.md 英/中两处。后续发版递增规则待用户定义。

- **[2026-08-06] 项目锚点日期(用户定义)**：项目开始(正排)/结束(倒排)日期是**唯一锚点**,放在项目详情页页首方向选择旁编辑,保存后全项目重排。正排链头(无显式+隐式前置、非 manual、非父)start 恒 = max(项目开始日期, 约束日期);倒排链尾 end 恒 = 项目结束日期。任务弹窗日期保持只读。前端联动:ProjectDetail 保存后 refreshKey 递增 → 甘特图 useOutletContext 监听重新 fetchData。UpdateProject 未携带字段保留旧值。

- **[2026-08-06] 乱码防护(强化 bug-025)**：写入口(Create/UpdateTask、Create/UpdateProject)新增 hasBadEncoding 校验——name/description 含**连续 ≥2 个 U+FFFD** 即 400 INVALID_ENCODING。真实中文文本不会出现连续替换字符(它是 GBK 字节被按 UTF-8 解码失败时产生的指纹),检测到即源头拦截,不再静默入库。我自己插入数据仍须遵循:UTF-8 文件 + --data-binary @file.json,或 python 显式 UTF-8。

- **[2026-08-06] 链头时长变更传播(Do-Not-Repeat 级)**：Recalculate 中 fixTriggerEnd(触发任务自身 end 重算)必须放在 recalc 之前——链头任务无前置不走 applyCandidate,若 end 修正发生在传播后,后继仍按旧 end 排。教训:修正"触发任务自身"的写操作必须在 loadTasks 之前完成,传播阶段才能读到新值。

- **[2026-08-06] 关键路径实现要点**：ComputeTotalFloat 导出函数(复用 forwardPass/rollup/backwardPass,不写库),ListTasks 返回 critical_ids,前端 task_class 红色左缘。倒推必须与正推语义对称(显式前置存在则隐式失效)、必须用工作日语义(SubWorkDays,非 shiftDate)。
- **[2026-08-06] dhtmlx-gantt GPL 无 marker 插件**：addMarker/plugins({marker:true}) 在 GPL 版不存在(静默 undefined)。今日线用 posFromDate(new Date()) 自绘 SVG 竖线(挂 drawMergedLinks 重绘)。
- **[2026-08-06] 批次1交付**：关键路径高亮、状态/进度联动防呆(completed↔100%、>0%→进行中、0%→待开始)、时长列(甘特图+列表)、今日线自绘。

## Decision Log
- **[2026-08-10] 项目详情公开只读(用户拍板)**：GET /api/projects/{id} 移出 RequireAuth 组，未登录可浏览项目页（甘特图/任务列表/过滤只读，与 /tasks 公开一致）；项目 CRUD 写操作仍需登录。前端 readonly 守卫（勾选框/批量条/复制粘贴/添加导入按钮）即此场景的防线。
- **[2026-08-10] 项目日期随任务推导(用户定义)**：正排项目 end_date 自动 = 最晚任务结束；倒排项目 start_date 自动 = 最早任务开始。锚点仍分别是正排 start / 倒排 end（用户设定，rollup 不覆盖）。实现要点：rollup 调用必须在「写库循环之后」取重排结果（提前返回分支前的那次取 DB 旧值，仅兜底 changes==0 场景），且两个分支各需一次——只放一处会在另一分支失效（本次踩坑：先删了写库后的调用导致项目日期停留在旧值）。
- **[2026-08-19] 收工技能升级用户级(用户拍板)**：work-wrap-up 从 ndm 项目复制到 ~/.claude/skills/，全局所有项目可用。内容通用(.wolf 记录 + git 提交推送，无项目特定路径)，ndm 内副本保留(项目级同名技能优先，不冲突)。缺点：全局目录不随 git 同步，其他电脑需手动复制一份。

## Key Learnings
- **[2026-08-10] 多选组件交互职责分离**：MultiUserSelect 教训——"点击标签区展开下拉"与"×按钮删除"在同一区域时,真实环境一次点击可能派发两个 click(第二个错位命中删除按钮)导致误删,且即改即存放大损失(清空 owner 级联改派清空任务负责人)。正确模式:标签区只负责显示与删除(×),展开列表走独立按钮;删除按钮加坐标校验(getBoundingClientRect 比对 clientX/Y)拦截错位事件。
- **[2026-08-10] 改派触发必须集合比较**：UpdateProject 的 ownerChanged 不能凭"请求体含 owner_ids 键"判定——前端 {...project} 展开使每次改日期/方向都携带 owner_ids,会静默改派所有 open/delayed 任务并 version+1。必须 loadProjectOwners 取旧集合与请求 ids 做集合比较(空集相等、顺序无关),仅真正变化才触发。
- **[2026-08-10] owner_ids 子查询必须 COALESCE**：GROUP_CONCAT 空集返回 NULL,Go 把 SQL NULL scan 到 string 报错 → ProjectList 的 continue 静默剔除项目(看板消失)+ GetProject 404。凡子查询返回标量的列都要 COALESCE(...,'')。
- **[2026-08-19] "收工"技能定位**：收工技能真实文件名 `work-wrap-up`，位于 F:\projects\ndm\.claude\skills\work-wrap-up\（另有 .claude/workflows/work-wrap-up.js），已 git 提交并推送到远程，文件完整。它**不属于 followITup 仓库**——本项目 .claude/skills/ 仅 gitnexus/ldap-sync/project-status。followITup 会话里的"收工"是 memory.md 惯例（提交→推送→记 memory），非技能调用。排查技能缺失先想"它在哪个项目/全局"。
- **[2026-08-20] 前端构建前置 + 弹窗日期刷新**：(1) npm run build 前若 tsc 报 TS2307 找不到 react-i18next/i18next/html2canvas 等模块，是 node_modules 缺失，先 npm install。(2) 编辑任务弹窗加/删前置依赖后必须刷新弹窗 start/end 状态(GET /tasks/{id})，否则保存时 handleSave 用旧日期全列覆盖排程结果(bug-257)。排程只改 start/end 不改 duration，刷新时不要动 duration。
- **[2026-08-20] 任务弹窗两栏布局(用户拍板)**：左栏=名称/父任务+类型/负责人/日期与进度/状态+优先级；右栏=前置任务(编辑时)+约束。负责人靠上(父任务之后)避免下拉向下展开撑滚动；右栏 grid 1fr 1.15fr 容纳"批量添加"按钮；弹窗内紧凑化(input padding 6px 10px、divider 6/5、dep-list max-height 120px)。
- **[2026-08-20] 收工必须调用技能或完整执行其 Phase 5**：用户说「收工」时 work-wrap-up 技能(全局 ~/.claude/skills/work-wrap-up/SKILL.md)要求 commit 后必须 git push。本次手动收工只 commit 漏 push，被用户纠正。技能 Phase 5 第5步明确「git push origin <branch>，失败立即报错」——收工结尾必 push，别停。 
- **[2026-08-20] 用户机器 DPI 缩放导致显示不一致**：用户笔记本物理 1920x1200 + Windows 125% 缩放，有效 CSS 视口 = 1536x960(物理/1.25)。Playwright 浏览器 devicePixelRatio=1，browser_resize 设 1920 会让窗口超出用户 1536 有效宽、右边被裁(标题栏中/EN 按钮被截)。**在这台机器上 Playwright 测试/截图应设 1536x960，不是 1920x1200**。网页本身流式自适应(project-detail width 自适应)，1536 下 Navbar/布局不溢出——问题在测试视口设错，不在网页。 
- **[2026-08-20] 系统设置 tab 分页 + 部署网址邮件门控(用户拍板)**：系统设置 3 栏目(邮件通知/财年与密码/节假日)改 tab 分页，单卡片 maxWidth 800 居中，消除 1536 视口下滚动。部署网址 base_url 作为邮件发送前提：settings 加 KeyBaseURL，mail.Send 入口统一检查非空(空则返回'部署网址未配置'拒发)，body 末尾追加英文 'Sign in at: <url>'(邮件模板只用英文)。settings 表 key-value 结构，加字段无需 migration。
