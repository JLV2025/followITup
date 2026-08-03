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
