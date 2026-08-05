# 回收站（已删除任务/项目恢复）设计

日期：2026-08-05
状态：待用户复审

## 背景与目标

项目/任务删除均为软删除（`deleted_at` 时间戳，数据保留），但**没有恢复入口**——只能手动 SQL 恢复。同时确认权限模型：**不区分管理员/普通用户**（10 人可信小团队，删除权限全员一致），管理员特权仅限系统配置（用户管理等，已有 AdminOnly）。

本功能提供对称的恢复能力：项目页「回收站」恢复任务、首页「回收站」恢复项目。

## 用户决策记录（已确认）

1. 删除权限统一：普通用户与管理员均可删除项目/任务（与现状一致，不引入角色区分）
2. 任务恢复入口：项目页（甘特图工具栏）「回收站」按钮 → 弹窗列出已删除任务 → 点击恢复
3. 项目恢复入口：首页看板「回收站」入口 → 弹窗列出已删除项目 → 点击恢复；恢复项目自动带出项目内任务
4. ~~恢复任务不触发排程~~ **恢复任务实时全项目重算**（2026-08-05 变更）：恢复的任务完全纳入排程（RecalculateAll，同排序保存语义），日期按当前依赖链重新推导、隐式链按当前 sort_order 重建；显式依赖仍需手动重连（删除时已物理清理）
5. 统一文案「回收站」；已删除任务弹窗信息尽量详细（名称/删除时间/原日期范围/工期/进度/排序）
6. 不做彻底删除，数据永久保留（软删可恢复）

## 现状（保持不变的部分）

- `projects.deleted_at` / `tasks.deleted_at`：软删除标记，查询均过滤 `deleted_at IS NULL`
- `DELETE /api/projects/{id}`：软删项目（任务不动，任务 deleted_at 保持为空）
- `DELETE /api/projects/{id}/tasks/{taskID}`：软删任务 + **物理清理依赖**（dependencies 行删除）
- 权限：所有项目/任务 API 仅 `RequireAuth`（登录即可），无角色校验
- 前端：任务删除入口在 TaskDetailModal/TaskListView；项目删除无入口（本次不新增删除入口，只做恢复）

## 后端设计（4 个端点）

### 1. 列出已删除任务

```
GET /api/projects/{id}/tasks/deleted
```

- 返回项目内 `deleted_at IS NOT NULL` 的任务，按删除时间倒序
- 字段：id、name、task_type、start_date、end_date、duration_days、progress_pct、status、assignee、sort_order、parent_id、deleted_at
- 路由注意：`/tasks/deleted` 注册在 tasks 路由组（`/api/projects/{id}/tasks` 组内加 `r.Get("/deleted", ...)`），与 `/tasks/{taskID}` 无冲突（静态路径优先注册）

### 2. 恢复任务

```
POST /api/projects/{id}/tasks/{taskID}/restore
```

- 执行：`UPDATE tasks SET deleted_at = NULL WHERE id = ? AND project_id = ? AND deleted_at IS NOT NULL`
- **不触发排程**（用户决策 4）
- 返回更新后的任务对象；影响 0 行（任务不存在或未删除）→ 404「任务不存在或未删除」

### 3. 列出已删除项目

```
GET /api/projects?deleted=1
```

- 复用 ProjectList 路径，加 `deleted` 查询参数分支：`WHERE p.deleted_at IS NOT NULL ORDER BY p.deleted_at DESC`
- 返回字段与 ProjectList 一致（含 name、description、schedule_direction、deleted_at）
- 用查询参数而非 `/api/projects/deleted` 路径，规避 chi 路由 `/{id}` 冲突

### 4. 恢复项目

```
POST /api/projects/{id}/restore
```

- 执行：`UPDATE projects SET deleted_at = NULL WHERE id = ? AND deleted_at IS NOT NULL`
- 项目内任务自动可见（任务 deleted_at 本就为空，仅项目标记被删）——无需批量更新任务
- 影响 0 行 → 404「项目不存在或未删除」
- 不触发排程（项目恢复后任务按原状，与删除前一致）

## 前端设计（2 处入口）

### 1. 项目页回收站（ProjectGantt 工具栏）

- 工具栏「刷新」按钮旁加「回收站」按钮（btn-ghost btn-sm，复用现有样式）
- 点击 → 弹窗（新组件 `RecycleBinModal.tsx`，复用 modal-overlay/modal-card/modal-title/modal-actions 样式）：
  - 标题「回收站」+ 项目名
  - 列表：每行显示任务名、删除时间、原日期范围、工期、进度、排序（信息详细，便于判断）
  - 每行「恢复」按钮 → `POST /api/projects/{id}/tasks/{taskID}/restore` → 成功后刷新列表 + 提示「已恢复，任务已回到甘特图（依赖需手动重连）」
  - 空态：「没有已删除的任务」
- 关闭弹窗后刷新甘特图数据（fetchData）——恢复的任务出现在甘特图

### 2. 首页回收站（Dashboard）

- 看板头部「+ 创建项目」旁加「回收站」按钮（btn-ghost btn-sm）
- 点击 → 弹窗（复用 modal 样式，可内联实现）：
  - 标题「回收站」
  - 列表：每行显示项目名、删除时间、描述
  - 每行「恢复」按钮 → `POST /api/projects/{id}/restore` → 成功后刷新列表 + 项目回到看板（刷新项目列表）
  - 空态：「没有已删除的项目」

## 测试计划

### 后端

无 API 测试设施（既有现状），验证靠：编译 + `go test ./...` 全过 + 浏览器实测。

### 浏览器验证清单

1. 项目页：删除一个任务 → 工具栏「回收站」→ 弹窗显示该任务（含名称/删除时间/日期/工期/进度）→ 点击恢复 → 任务回到甘特图（带原日期），弹窗列表刷新为空态
2. 恢复任务后不自动重排（日期保持删除前值）；改 duration 后引擎正常接管（排程触发）
3. 首页：删除一个项目 → 看板「回收站」→ 弹窗显示该项目 → 点击恢复 → 项目回到看板，项目内任务全部可见
4. 已删除项目内的任务状态：项目删除时任务未被删（任务列表在恢复前不可访问——GetProject 404，恢复后可访问）
5. 恢复不存在/未删除的任务/项目 → 404 提示

## 风险与边界

- 任务显式依赖在删除时已物理清理——恢复后需手动重连（隐式顺序按 sort_order 自动重建）
- 恢复的任务带删除前日期，可能与其他任务日期重叠/悬浮——用户手动调整（改 duration/排序/拖动排程触发）解决，符合"恢复后用户自己去调整顺序和时间安排"
- 项目恢复后 schedule_direction 等字段保留（软删未动数据）
- 数据永久保留，不做物理删除（DB 体积可忽略）

## 明确不做（YAGNI）

- 管理员/普通用户角色区分（删除权限全员一致）
- 彻底删除（物理删除）
- 恢复时自动触发排程（用户决策：手动）
- 批量恢复（一次恢复一个）
- 回收站自动清理（过期自动删）
