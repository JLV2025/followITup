# 项目排程方向（正推/倒推）+ duration 驱动 设计

日期：2026-08-04
状态：待用户复审

## 背景与目标

当前排程引擎只有正推（从项目开始日期向后铺），且用户可直接编辑任务日期，导致"用户指定的日期被排程冲掉"、"定死某个交付日期"等诉求没有干净解法。经过重新讨论，确定全新模型：

**日期是引擎的输出，不是用户的输入。** 项目创建时选定排程方向（正推或倒推），方向全局统一，用户只能通过修改任务 duration（时长）间接影响开始/结束时间。整个模型自洽、简单，消灭了字段级锚点、多 deadline 嵌套、工期分配弹窗等全部复杂设计。

## 用户决策记录（已确认）

1. 创建项目时指定排程方向：**正推**（基于项目开始日期）或**倒推**（基于项目完成日期）
2. 任一任务进度 > 0 后，方向**锁定**不可修改
3. 日期由引擎计算，**用户只能通过修改 duration** 影响任务的开始/结束时间
4. 日期计算**排除节假日**（工作日历）
5. 甘特图**禁止拖拽任务条**（日期不可拖）；行拖拽排序保留，改顺序/改 duration 驱动重排，甘特图自动响应
6. **不做工期分配弹窗**（改 duration 即分配）
7. 倒推项目**只有完成日期**，不允许定义开始日期（开始日期 = 引擎输出，用户通过 duration 间接控制）
8. 倒推项目**所有分支（链尾）一律对齐项目完成日期**，不允许任何任务超过；太复杂的项目由用户拆分为多个项目管理

## 现状（现有机制，保持不变的部分）

- `tasks.manual_scheduled`：任务级手动锁定，排程跳过（保留）
- `tasks.constraint_type/constraint_date`：约束日期（保留原行为，不与新模型冲突）
- `projects`：含日期范围字段
- 引擎：`forwardPass`（正推 + 隐式顺序依赖 + 双向跟随 + duration 固定）、`backwardPass`（仅算关键路径 LS/LF/TF，不写回日期）、`rollupParentDates`（父任务汇总）、迭代 5 轮收敛、`LoadCalendar`/`AddWorkDays` 工作日历
- 前端：`TaskDetailModal` 可编辑日期；甘特图支持任务条拖拽；行拖拽排序（`sort_order`）已持久化

## 数据模型（迁移 v5）

```sql
ALTER TABLE projects ADD COLUMN schedule_direction TEXT NOT NULL DEFAULT 'forward';
```

- 取值：`'forward'`（正推）/ `'backward'`（倒推），存量项目默认 `forward`
- 倒推项目：`projects.start_date` 留空不填（前端隐藏/禁用该字段），`end_date` 为项目完成日期（用户定义）
- 不动其他任何字段

## 引擎设计

### 方向路由

`Recalculate`/`RecalculateAll` 根据项目 `schedule_direction` 选择 pass：

- `forward`：现有 `forwardPass` 全量重算（链头 = 无前置任务，start = 项目开始日期）
- `backward`：新增 `backwardSchedule` 全量重算（见下）

全量重算即可，无需触发任务传播的精确增量——项目小、成本低，且新模型下"编辑日期保留"语义已不存在（日期不可编辑）。`manual_scheduled` 任务在两种 pass 中都跳过。

### 倒推 pass（backwardSchedule，写回日期）

输入：任务、依赖、隐式顺序依赖（`buildImplicitPred` 复用）、工作日历、项目完成日期。

算法：

1. **链尾**（无显式后继且无隐式后继的任务）：`end = 项目完成日期`
2. 从链尾沿前驱链（显式 `predDeps` + 隐式 `implicitPred`）倒推，BFS 逐任务确定 `end`，visited 防环：
   - 对任务 X 的每个后继 S（依赖类型 T、lag L），计算 X 的 end 候选（S 的日期已知）：
     - FS：`X.end = S.start - L`
     - FF：`X.end = S.end - L`
     - SS：`X.start ≤ S.start - L` → 转化为 `X.end ≤ (S.start - L) + X.duration`（工作日联立）
     - SF：`X.start ≤ S.end - L` → 转化为 `X.end ≤ (S.end - L) + X.duration`
   - **多后继取最严格**：`X.end = min(全部候选)`
   - `X.start = X.end` 往前数 `X.duration` 个工作日（排除节假日，`AddWorkDays` 的反向对偶）
3. **多链尾一律对齐项目完成日期**（用户决策 8），无例外
4. 父任务 rollup 复用：子任务日期汇总 → 父任务日期（与正推一致）；父任务作为前驱参与下一轮
5. 迭代 5 轮收敛（复用现有循环结构）

**无冲突标记**：倒推项目任何 duration 组合都有解（start 可无限前移），duration 由用户控制，无"早于开始日期"底线。正推项目沿用现有 `constraint_conflict` 行为（`finish_no_later_than` 约束冲突）。

### 工作日对偶

正推：`end = AddWorkDays(cal, start, duration)`（start 起第 duration 个工作日为 end）。
倒推：`start = AddWorkDays(cal, end, -duration)` 或等价实现（从 end 往前数 duration 个工作日），须与正向严格对偶（`forward(backward(x)) == x`），单测验证。

### 触发时机

- 任务 duration 变更 → 保存 → 按方向全量重算 → 落库 → WS 广播（现有链路不动，只改 pass 选择）
- 排序变更（行拖拽）→ 现有 `UpdateTaskSortOrder` → 全量重算
- 依赖增删、项目日期变更 → 全量重算

## 前端设计

### 项目创建表单

- 新增「排程方向」选择：正推（基于开始日期）/ 倒推（基于完成日期）
- 正推：显示「项目开始日期」输入（必填）
- 倒推：显示「项目完成日期」输入（必填），「项目开始日期」不显示

### 项目设置页

- 显示排程方向；存在任一 `progress_pct > 0` 的任务时，方向控件置灰禁用并提示原因

### 任务弹窗（TaskDetailModal）

- 开始日期、结束日期：**只读展示**（引擎输出，不可编辑）
- duration：可编辑（数字输入），保存触发重排
- `manual_scheduled` 手动锁定开关：保留现有行为

### 甘特图（ProjectGantt）

- **禁用任务条拖拽**（dhtmlx-gantt 关闭 drag 配置），防止用户直接改日期
- 行拖拽排序保留（已有）
- 重排结果自动刷新（现有 fetchData 链路）

### 锁定规则（后端）

- `PUT /api/projects/{id}/direction`（或并入项目更新 API）：校验项目内无 `progress_pct > 0` 的任务，否则 400「项目已有任务进度，排程方向不可修改」
- 无进度任务时方向可随时修改（创建后可改，不受创建时的选择限制）；任一任务有进度后锁定

## 测试计划

### 后端单测（scheduler_test.go 追加）

1. `TestBackwardScheduleSingleChain`：A(5天) → B(7天)，完成日期 7/31 → B.end=7/31、B.start=7/31 往前 7 工作日；A.end=B.start、A.start 往前 5 工作日
2. `TestBackwardScheduleMultiTail`：两条独立分支都对齐完成日期
3. `TestBackwardScheduleImplicitPred`：隐式顺序依赖参与倒推（同分支相邻任务衔接）
4. `TestBackwardScheduleDepTypes`：FS/SS/FF/SF + lag 四种类型倒推正确
5. `TestBackwardScheduleWorkdayPairity`：正推→倒推对偶（forward(backward(x)) == x，含节假日日历）
6. `TestBackwardScheduleParentRollup`：父任务 rollup 参与迭代收敛
7. `TestBackwardScheduleManualScheduled`：manual_scheduled 任务在倒推中不被改写
8. `TestDirectionLock`：进度 > 0 后改方向返回错误

### 浏览器验证

- 创建倒推项目（填完成日期）→ 建任务链 → 日期全部从完成日期倒推铺开，无任务超过完成日期
- 改某任务 duration → 其 start（和前置链）自动调整，甘特图自动刷新
- 甘特图任务条不可拖拽；行拖拽排序仍可 → 重排正确
- 任务弹窗日期只读、duration 可编辑
- 任一任务填了进度 → 项目设置里方向置灰
- 正推项目行为与现状一致（回归）

## 风险与边界

- 存量项目默认 `forward`，行为不变，无迁移风险
- 倒推项目里已执行任务（有进度/实际日期）的计划日期仍会被重排——计划修正语义，与正推一致；实际日期（actual_start/actual_end）照旧记录不被覆盖
- 倒推项目 `projects.start_date` 留空：甘特图时间轴范围由任务范围决定（前端取 min(task.start)），不依赖项目开始字段
- 复杂项目（多分支、子项目依赖）→ 用户拆分为多个项目，不做跨项目联动（用户决策）
- duration 输入下限 1 天：**新增前端（输入 min=1）+ 后端校验**（当前无校验）；里程碑（duration=0，`task_type='milestone'`）沿用现有处理，倒推对偶公式在 duration=0 时自然成立（end=start）

## 明确不做（YAGNI）

- 字段级锚点（manual_start_date/manual_end_date）——日期不可编辑，无需锚定
- 工期分配弹窗——改 duration 即分配
- 多 deadline 倒推、嵌套/重叠时段检测、互斥校验、共享边界——项目级方向统一后全部不成立
- 甘特图任务条拖拽
- 倒推项目冲突标记——无冲突底线
- 跨项目排程联动
