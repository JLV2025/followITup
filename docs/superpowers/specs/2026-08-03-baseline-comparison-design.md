# 基线对比功能 — 设计文档

> 版本：v1.0
> 日期：2026-08-03
> 状态：已批准（用户逐节确认）

---

## 1. 背景与目标

项目经理需要在项目执行过程中回答两个问题：

1. **排程漂移**：当前计划与制定基线时的计划差了多少天？
2. **执行偏差**：任务实际开始/完成日期与计划相比如何？

本功能通过"基线快照 + 实际执行日期"两层对比提供答案：甘特图上叠加基线条（计划存档）与实际执行条，看板显示偏差统计。

## 2. 需求决策（用户确认）

| 决策点 | 结论 |
|--------|------|
| 快照范围 | **单一当前基线**，快照核心排程字段（start/end/duration/progress） |
| 甘特呈现 | 基线条（深灰）+ 实际执行条（浅绿），**紧贴任务条上下边缘，无间隔** |
| 看板统计 | 统计卡"整体完成"整合进度偏差 Δ% + 项目卡片 Δ 天数徽标 |
| 实际日期 | 弹窗可编辑 + 状态变更自动填充（已有值不覆盖） |
| 管理入口 | 甘特图工具栏"基线"下拉菜单（创建/信息/清除） |
| 存储方案 | **方案 A：基线列**（tasks 加 4 列 + projects 加 2 列） |

## 3. 数据模型（迁移 v4）

```sql
ALTER TABLE tasks ADD COLUMN baseline_start_date TEXT;      -- 基线开始（快照）
ALTER TABLE tasks ADD COLUMN baseline_end_date TEXT;        -- 基线结束（快照）
ALTER TABLE tasks ADD COLUMN baseline_duration_days INTEGER;-- 基线时长（快照）
ALTER TABLE tasks ADD COLUMN baseline_progress_pct REAL;    -- 基线进度（快照）
ALTER TABLE projects ADD COLUMN baseline_created_at TEXT;   -- 基线创建时间
ALTER TABLE projects ADD COLUMN baseline_created_by TEXT;   -- 基线创建人
```

**设计说明**：
- 依赖关系不单独快照——任务级基线列已含日期，依赖变更由当前排程结果体现，避免过度设计
- "单一当前基线"语义下，基线列优于快照表：创建 = 一条 UPDATE，统计 = 直接 SQL 聚合
- 将来若需多基线（Baseline1/2/3），加表迁移即可平滑升级，基线列保留作"当前基线"

## 4. 后端 API

新文件 `backend/internal/api/baseline.go`（BaselineHandler），路由挂载于 `server.go`：

| 方法 | 路径 | 行为 |
|------|------|------|
| POST | `/api/projects/{id}/baseline` | 创建/覆盖基线：事务内 `UPDATE tasks SET baseline_*=当前值 WHERE project_id=? AND deleted_at IS NULL` + 写 projects 元数据；成功后 WS 广播刷新 |
| DELETE | `/api/projects/{id}/baseline` | 清除基线：4 列 + 元数据置 NULL；广播刷新 |
| GET | `/api/projects/{id}/baseline` | 返回 `{ created_at, created_by, task_count }` |

**权限**：editor+ 可创建/清除，viewer 只读（沿用 tasks.go 既有项目成员角色校验模式）。

**模型变更**：
- `Task` 加 4 个 `baseline_*` 字段，任务列表/详情响应自动携带
- `Project` 加 `baseline_created_at` / `baseline_created_by`

## 5. 实际日期自动填充（tasks.go UpdateTask）

```
status 变为 in_progress 且 actual_start 为空 → actual_start = 今天（YYYY-MM-DD）
status 变为 completed 且 actual_end 为空   → actual_end = 今天
已有值不覆盖（用户手动改过的保留）
```

TaskDetailModal 提供「实际开始/实际结束」日期输入，手动可编辑，配合自动填充。

## 6. 前端甘特图基线层

### gantt-adapter.ts
`GanttTask` 加 4 字段：`baseline_start_date` / `baseline_end_date` / `actual_start` / `actual_end`，`toGanttTask` 透传。

### ProjectGantt.tsx — 绘制层（复用 addTaskLayer，注册在协作聚焦层之前）

```
┌────────────────────────────────────────────┐  行高 28px
│▬▬▬▬▬▬▬▬▬▬▬▬                              │ ← 基线条 4px，紧贴任务条顶边（top: -4px）
│  ▓▓▓▓▓▓▓▓░░░░░░░░░                        │ ← 任务条 20px（当前计划）
│  ▔▔▔▔▔▔▔▔                                 │ ← 实际执行条 4px，紧贴任务条底边（bottom: -4px）
└────────────────────────────────────────────┘
```

- 定位：`gantt.posFromDate()` 计算基线起止像素 x 坐标
- 条件：有 `baseline_start_date` 才画基线条；有 `actual_start` 才画实际条
- 颜色：基线条 `#6B7280`（灰），实际条 `#86EFAC`（浅绿）
- 两条细条均在 28px 行内解决（任务条 20px 居中，上下各留 4px 空隙），不与相邻行重叠

### 工具栏"基线"下拉（缩放控件旁）

```
[基线 ▾]  →  创建基线（confirm 覆盖确认）
             ├─ 基线信息（创建时间/创建人/任务数）
             ├─ 清除基线（confirm 确认）
```

- 按钮态：有基线时实心 + 显示创建日期，无基线时空心
- 创建成功 → WS 广播刷新 + 基线层即时出现

### TaskDetailModal.tsx
- 新增「实际开始 / 实际结束」日期输入（可编辑）
- 有基线时显示 `基线: 07-01 ~ 08-15` + 偏差徽标 `Δ +3d`（当前开始 − 基线开始）

### ganttStore.ts
加 `baselineMeta` 状态（created_at/created_by/task_count）+ 创建/清除 action。

## 7. 看板偏差统计

### API（改 projects.go DashboardStats + 项目列表查询）

延续"顶层任务时长加权"既有语义，扩展聚合：

```sql
-- 基线完成率（顶层任务，baseline 口径）
SUM(顶层 baseline_progress_pct*baseline_duration_days) / NULLIF(SUM(顶层 baseline_duration_days), 0)

-- 基线项目结束 = MAX(baseline_end_date)（全任务，rollup 保证父任务覆盖子任务）
-- 项目偏差天数 = MAX(end_date) − MAX(baseline_end_date)
```

### 前端 Dashboard.tsx

**统计卡"整体完成"整合进度偏差**（保持 4 列布局，不新增卡）：

```
┌─────────────────┐
│ 整体完成         │
│ 64%   Δ +8%     │ ← 偏差小字：绿 + / 红 − / 无基线不显示
└─────────────────┘
```

**项目卡片 Δ 天数徽标**（项目名右侧，有基线时显示）：

```
🔴 CRM系统上线                        Δ +3d  [详情 →]
```

- 提前 → 灰/绿，延期 → 红
- 无基线项目不显示徽标

## 8. 测试计划

`backend/internal/api/baseline_test.go`：

| 用例 | 断言 |
|------|------|
| 创建基线 | 所有任务 baseline_* = 当前值；projects 元数据写入；200 |
| 覆盖基线 | 第二次创建后快照 = 最新值 |
| 清除基线 | baseline_* 全 NULL，元数据清除 |
| 权限 | viewer 创建 → 403 |
| 实际日期自动填充 | →in_progress 记 actual_start；→completed 记 actual_end；已有值不覆盖 |
| 回归 | 既有 `go test ./...` 全过 + 前端 tsc + 完整 exe 构建 |

## 9. 明确不做（YAGNI）

- 多命名基线（Baseline1/2/3）— 迁移预留，不做
- 基线回滚/恢复 — 基线是参考线，不写回
- 依赖关系快照 — 任务级基线列已含日期
- 看板新增独立"进度偏差"卡 — 整合进"整体完成"卡
