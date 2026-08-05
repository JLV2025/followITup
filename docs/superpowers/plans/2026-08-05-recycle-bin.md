# 回收站（已删除任务/项目恢复）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 项目页与首页各加「回收站」入口：恢复已删除任务（不触发排程）与已删除项目（自动带出项目内任务）。

**Architecture:** 后端新增 4 个端点（已删任务列表/恢复任务、已删项目列表/恢复项目），复用软删除 `deleted_at` 机制；前端两处入口：ProjectGantt 工具栏「回收站」按钮 → RecycleBinModal 弹窗列已删任务，Dashboard 头部「回收站」按钮 → 弹窗列已删项目。权限全员一致（仅 RequireAuth）。

**Tech Stack:** Go 1.22+（chi/SQLite）/ React 18 TypeScript / Vite

## Global Constraints

- 不引入新依赖（后端、前端均不新增包）
- 所有注释、提交信息使用简体中文；专业术语保留英文
- 改符号前 `gitnexus_impact({target, direction:"upstream"})`，HIGH/CRITICAL 风险先告知用户
- 提交前 `gitnexus_detect_changes()` 验证影响范围
- 后端每步跑 `go build ./...` + `go test ./...`；前端每步跑 `npx tsc --noEmit`
- 每个逻辑变更单独提交（中文提交信息）
- 设计依据：`docs/superpowers/specs/2026-08-05-recycle-bin-design.md`（用户确认 6 条决策）
- 后端 api 包无测试设施：handler 正确性靠编译 + 全量测试 + 浏览器实测

---

### Task 1: 后端任务回收站端点（列表 + 恢复）

**Files:**
- Modify: `backend/internal/api/tasks.go`（路由注册 + 2 个 handler）

**Interfaces:**
- Consumes: 现有 `writeError/writeJSON`、`models.Task`、`boolToInt2` 模式
- Produces:
  - `GET /api/projects/{id}/tasks/deleted` → `{data: [...]}` 已删任务数组（id/name/task_type/start_date/end_date/duration_days/progress_pct/status/assignee/sort_order/parent_id/deleted_at），按 deleted_at DESC
  - `POST /api/projects/{id}/tasks/{taskID}/restore` → 200 返回恢复后的任务对象；404「任务不存在或未删除」

- [ ] **Step 1: 注册路由（顺序关键）**

`tasks.go` 的 `RegisterRoutes` 中，**`r.Get("/api/projects/{id}/tasks/{taskID}", h.GetTask)` 之前**插入只读的 deleted 列表（chi 顺序匹配，`/tasks/deleted` 必须先于 `/{taskID}` 注册，否则 "deleted" 会被当作 taskID）：

```go
	// 只读（公开或可选认证）
	r.Get("/api/projects/{id}/tasks", h.ListTasks)
	r.Get("/api/projects/{id}/tasks/deleted", h.ListDeletedTasks) // 回收站：必须注册在 /{taskID} 之前
	r.Get("/api/projects/{id}/tasks/{taskID}", h.GetTask)
```

RequireAuth 组内加 restore（`/tasks/{taskID}/restore` 路径更长不冲突）：

```go
		r.Post("/api/projects/{id}/tasks/{taskID}/restore", h.RestoreTask)
```

- [ ] **Step 2: 实现 ListDeletedTasks**

在 `tasks.go` 的 ListTasks 附近追加（查询字段与 ListTasks 保持一致，加 deleted_at）：

```go
// ListDeletedTasks 列出项目已删除任务（回收站）
func (h *TaskHandler) ListDeletedTasks(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	rows, err := h.db.Query(
		`SELECT id, project_id, parent_id, name, task_type, status, priority, assignee,
		        start_date, end_date, duration_days, progress_pct,
		        sort_order, deleted_at
		 FROM tasks WHERE project_id = ? AND deleted_at IS NOT NULL
		 ORDER BY deleted_at DESC`, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "查询已删除任务失败")
		return
	}
	defer rows.Close()

	type DeletedTask struct {
		ID           int64   `json:"id"`
		ProjectID    int64   `json:"project_id"`
		ParentID     *int64  `json:"parent_id"`
		Name         string  `json:"name"`
		TaskType     string  `json:"task_type"`
		Status       string  `json:"status"`
		Priority     string  `json:"priority"`
		Assignee     string  `json:"assignee"`
		StartDate    string  `json:"start_date"`
		EndDate      string  `json:"end_date"`
		DurationDays int     `json:"duration_days"`
		ProgressPct  float64 `json:"progress_pct"`
		SortOrder    int     `json:"sort_order"`
		DeletedAt    string  `json:"deleted_at"`
	}
	var tasks []DeletedTask
	for rows.Next() {
		var t DeletedTask
		var parentID sql.NullInt64
		if err := rows.Scan(&t.ID, &t.ProjectID, &parentID, &t.Name, &t.TaskType, &t.Status,
			&t.Priority, &t.Assignee, &t.StartDate, &t.EndDate, &t.DurationDays,
			&t.ProgressPct, &t.SortOrder, &t.DeletedAt); err != nil {
			continue
		}
		if parentID.Valid {
			t.ParentID = &parentID.Int64
		}
		tasks = append(tasks, t)
	}
	writeJSON(w, http.StatusOK, tasks)
}
```

- [ ] **Step 3: 实现 RestoreTask**

```go
// RestoreTask 恢复已删除任务（软删除置空，不触发排程）
func (h *TaskHandler) RestoreTask(w http.ResponseWriter, r *http.Request) {
	taskID, _ := strconv.ParseInt(chi.URLParam(r, "taskID"), 10, 64)
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	result, err := h.db.Exec(
		`UPDATE tasks SET deleted_at = NULL, updated_at = datetime('now')
		 WHERE id = ? AND project_id = ? AND deleted_at IS NOT NULL`,
		taskID, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "恢复任务失败")
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "任务不存在或未删除")
		return
	}

	// 返回恢复后的任务（复用 GetTask 的查询逻辑：按 id 查询单任务并 JSON 返回）
	h.GetTask(w, r)
}
```

（`h.GetTask(w, r)` 复用现有单任务查询，保证返回结构与前端一致；注意 GetTask 过滤 `deleted_at IS NULL`，恢复后已满足。）

- [ ] **Step 4: 编译 + 全量测试**

Run: `cd F:\projects\followITup\backend && go build ./... && go test ./...`
Expected: 编译通过、全部 PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/api/tasks.go
git commit -m "后端:任务回收站端点(已删任务列表GET /tasks/deleted + 恢复POST /tasks/{id}/restore,不触发排程)"
```

---

### Task 2: 后端项目回收站端点（列表 + 恢复）

**Files:**
- Modify: `backend/internal/api/projects.go`（路由注册 + 2 个 handler）

**Interfaces:**
- Consumes: `models.Project`、`writeError/writeJSON`、`boolToInt` 模式
- Produces:
  - `GET /api/projects?deleted=1`（RequireAuth 组内）→ `{data: [...]}` 已删项目数组（id/name/description/start_date/end_date/status/is_public/schedule_direction/deleted_at），按 deleted_at DESC；`deleted` 非 "1" 时返回未删项目列表（同结构，无财年过滤）
  - `POST /api/projects/{id}/restore` → 200 返回恢复后的项目；404「项目不存在或未删除」

- [ ] **Step 1: 注册路由**

`projects.go` 的 `RegisterRoutes` RequireAuth 组内（`GET /api/projects` 无路径段，与 `GET /api/projects/{id}` 路径形状不同，无冲突；`POST /{id}/restore` 与现有 POST 不冲突）：

```go
		r.Get("/api/projects", h.ListProjects)               // ?deleted=1 已删项目
		r.Post("/api/projects", h.CreateProject)
		r.Post("/api/projects/{id}/restore", h.RestoreProject)
		r.Put("/api/projects/{id}", h.UpdateProject)
```

- [ ] **Step 2: 实现 ListProjects**

在 `projects.go` 追加（`deleted=1` → 已删项目；否则 → 未删项目，字段复用 ProjectSummary 简化版）：

```go
// ListProjects 项目列表：?deleted=1 返回已删除项目（回收站），否则返回未删除项目
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	deleted := r.URL.Query().Get("deleted")
	where := "deleted_at IS NULL"
	if deleted == "1" {
		where = "deleted_at IS NOT NULL"
	}
	rows, err := h.db.Query(
		`SELECT id, name, description, start_date, end_date, status, is_public,
		        COALESCE(schedule_direction, 'forward'), deleted_at
		 FROM projects WHERE ` + where + ` ORDER BY deleted_at DESC, created_at DESC`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "查询项目失败")
		return
	}
	defer rows.Close()

	type ProjectItem struct {
		ID                int64   `json:"id"`
		Name              string  `json:"name"`
		Description       string  `json:"description"`
		StartDate         string  `json:"start_date"`
		EndDate           string  `json:"end_date"`
		Status            string  `json:"status"`
		IsPublic          bool    `json:"is_public"`
		ScheduleDirection string  `json:"schedule_direction"`
		DeletedAt         *string `json:"deleted_at"`
	}
	var projects []ProjectItem
	for rows.Next() {
		var p ProjectItem
		var isPublic int
		var deletedAt sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.StartDate, &p.EndDate,
			&p.Status, &isPublic, &p.ScheduleDirection, &deletedAt); err != nil {
			continue
		}
		p.IsPublic = isPublic != 0
		if deletedAt.Valid {
			p.DeletedAt = &deletedAt.String
		}
		projects = append(projects, p)
	}
	writeJSON(w, http.StatusOK, projects)
}
```

- [ ] **Step 3: 实现 RestoreProject**

```go
// RestoreProject 恢复已删除项目（项目标记置空；项目内任务本就未删，自动可见；不触发排程）
func (h *ProjectHandler) RestoreProject(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	result, err := h.db.Exec(
		`UPDATE projects SET deleted_at = NULL, updated_at = datetime('now')
		 WHERE id = ? AND deleted_at IS NOT NULL`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "恢复项目失败")
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "项目不存在或未删除")
		return
	}

	// 返回恢复后的项目（复用 GetProject 查询）
	h.GetProject(w, r)
}
```

- [ ] **Step 4: 编译 + 全量测试**

Run: `cd F:\projects\followITup\backend && go build ./... && go test ./...`
Expected: 编译通过、全部 PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/api/projects.go
git commit -m "后端:项目回收站端点(已删项目列表GET /api/projects?deleted=1 + 恢复POST /{id}/restore,任务自动带出)"
```

---

### Task 3: 前端项目页回收站（ProjectGantt 工具栏 + RecycleBinModal）

**Files:**
- Modify: `frontend/src/pages/ProjectGantt.tsx`
- Create: `frontend/src/components/RecycleBinModal.tsx`

**Interfaces:**
- Consumes: Task 1 端点 `GET /api/projects/{id}/tasks/deleted`、`POST /api/projects/{id}/tasks/{taskID}/restore`
- Produces: `RecycleBinModal` 组件（props: `projectId: number`, `onClose: () => void`, `onRestored: () => void`）

- [ ] **Step 1: 创建 RecycleBinModal 组件**

`frontend/src/components/RecycleBinModal.tsx`（弹窗样式仿 TaskDetailModal：modal-overlay/modal-card/modal-title/modal-actions）：

```tsx
import { useEffect, useState } from "react";
import api from "../api/client";

interface DeletedTask {
  id: number;
  name: string;
  task_type: string;
  start_date: string;
  end_date: string;
  duration_days: number;
  progress_pct: number;
  status: string;
  assignee: string;
  sort_order: number;
  deleted_at: string;
}

interface Props {
  projectId: number;
  projectName: string;
  onClose: () => void;
  onRestored: () => void;
}

export default function RecycleBinModal({ projectId, projectName, onClose, onRestored }: Props) {
  const [tasks, setTasks] = useState<DeletedTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [restoringId, setRestoringId] = useState<number | null>(null);

  const fetchDeleted = async () => {
    try {
      const res = await api.get(`/api/projects/${projectId}/tasks/deleted`);
      setTasks(res.data.data || []);
    } catch {
      setError("加载已删除任务失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDeleted();
  }, [projectId]);

  const handleRestore = async (task: DeletedTask) => {
    setRestoringId(task.id);
    try {
      await api.post(`/api/projects/${projectId}/tasks/${task.id}/restore`);
      await fetchDeleted();
      onRestored();
      alert(`「${task.name}」已恢复，任务回到甘特图（显式依赖需手动重连）`);
    } catch (err: any) {
      setError(err?.response?.data?.error?.message || "恢复失败");
    } finally {
      setRestoringId(null);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <div className="modal-title">
          <h2>回收站 · {projectName}</h2>
          <button className="btn btn-ghost btn-sm" onClick={onClose}>×</button>
        </div>
        <div className="modal-body">
          {error && <div className="form-error">{error}</div>}
          {loading ? (
            <p className="text-secondary">加载中...</p>
          ) : tasks.length === 0 ? (
            <p className="text-secondary">没有已删除的任务</p>
          ) : (
            <div className="dep-list">
              {tasks.map((t) => (
                <div className="dep-item" key={t.id}>
                  <div className="dep-item-main">
                    <span className="dep-item-name">
                      {t.name}
                      {t.task_type === "milestone" && <em className="tag">里程碑</em>}
                    </span>
                    <span className="dep-item-detail">
                      {t.start_date || "—"} ~ {t.end_date || "—"} · {t.duration_days}天 · 进度 {t.progress_pct}% · 原排序 #{t.sort_order + 1}
                    </span>
                    <span className="dep-item-detail">删除于 {t.deleted_at?.slice(0, 10)}</span>
                  </div>
                  <button
                    className="btn btn-primary btn-sm"
                    disabled={restoringId === t.id}
                    onClick={() => handleRestore(t)}
                  >
                    {restoringId === t.id ? "恢复中..." : "恢复"}
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
        <div className="modal-actions">
          <button className="btn btn-ghost" onClick={onClose}>关闭</button>
        </div>
      </div>
    </div>
  );
}
```

（dep-list/dep-item 类复用 TaskDetailModal 依赖列表样式；若样式不理想可在 components.css 补 `.dep-item-detail` 等轻量样式。）

- [ ] **Step 2: ProjectGantt 工具栏加「回收站」按钮 + 弹窗挂载**

`ProjectGantt.tsx`：
1. 顶部 import：`import RecycleBinModal from "../components/RecycleBinModal";`
2. 组件内加 state：`const [showRecycleBin, setShowRecycleBin] = useState(false);`
3. 工具栏「↻ 刷新」按钮旁加：

```tsx
              <button
                className="btn btn-ghost btn-sm"
                title="恢复已删除的任务"
                onClick={() => setShowRecycleBin(true)}
              >
                回收站
              </button>
```

4. 弹窗挂载（JSX 末尾，项目名用现有 project name state）：

```tsx
      {showRecycleBin && (
        <RecycleBinModal
          projectId={projectId}
          projectName={project?.name || "项目"}
          onClose={() => setShowRecycleBin(false)}
          onRestored={fetchData}
        />
      )}
```

（`projectId`/`fetchData` 为组件内现有变量，按实际命名对齐；onRestored 复用现有数据刷新函数。）

- [ ] **Step 3: 类型检查**

Run: `cd F:\projects\followITup\frontend && npx tsc --noEmit`
Expected: 无类型错误

- [ ] **Step 4: 提交**

```bash
git add frontend/src/components/RecycleBinModal.tsx frontend/src/pages/ProjectGantt.tsx
git commit -m "前端:项目页回收站(工具栏按钮+弹窗列已删任务,点击恢复,不触发排程)"
```

---

### Task 4: 前端首页回收站（Dashboard 入口 + 弹窗）

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`

**Interfaces:**
- Consumes: Task 2 端点 `GET /api/projects?deleted=1`、`POST /api/projects/{id}/restore`
- Produces: 看板头部「回收站」按钮 + 已删项目弹窗

- [ ] **Step 1: 加按钮与弹窗**

`Dashboard.tsx`：
1. 组件内加 state：

```tsx
  const [showRecycleBin, setShowRecycleBin] = useState(false);
  const [deletedProjects, setDeletedProjects] = useState<any[]>([]);
```

2. 「+ 创建项目」按钮旁加：

```tsx
        <button
          className="btn btn-ghost"
          onClick={async () => {
            try {
              const res = await api.get("/api/projects?deleted=1");
              setDeletedProjects(res.data.data || []);
              setShowRecycleBin(true);
            } catch (err: any) {
              alert(err?.response?.data?.error?.message || "加载回收站失败");
            }
          }}
        >
          回收站
        </button>
```

3. 弹窗（仿 Task 3 的 modal 结构，项目行显示名称/删除时间/描述 + 恢复按钮）：

```tsx
      {showRecycleBin && (
        <div className="modal-overlay" onClick={() => setShowRecycleBin(false)}>
          <div className="modal-card" onClick={(e) => e.stopPropagation()}>
            <div className="modal-title">
              <h2>回收站</h2>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowRecycleBin(false)}>×</button>
            </div>
            <div className="modal-body">
              {deletedProjects.length === 0 ? (
                <p className="text-secondary">没有已删除的项目</p>
              ) : (
                <div className="dep-list">
                  {deletedProjects.map((p) => (
                    <div className="dep-item" key={p.id}>
                      <div className="dep-item-main">
                        <span className="dep-item-name">{p.name}</span>
                        <span className="dep-item-detail">
                          {p.description || "—"} · {p.schedule_direction === "backward" ? "倒排" : "正排"}
                        </span>
                        <span className="dep-item-detail">删除于 {p.deleted_at?.slice(0, 10)}</span>
                      </div>
                      <button
                        className="btn btn-primary btn-sm"
                        onClick={async () => {
                          try {
                            await api.post(`/api/projects/${p.id}/restore`);
                            alert(`项目「${p.name}」已恢复，项目内任务已全部恢复`);
                            setDeletedProjects((prev) => prev.filter((x) => x.id !== p.id));
                            fetchProjects(); // 刷新看板项目列表
                          } catch (err: any) {
                            alert(err?.response?.data?.error?.message || "恢复失败");
                          }
                        }}
                      >
                        恢复
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
            <div className="modal-actions">
              <button className="btn btn-ghost" onClick={() => setShowRecycleBin(false)}>关闭</button>
            </div>
          </div>
        </div>
      )}
```

（`fetchProjects` 为 Dashboard 现有刷新函数名，按实际命名对齐；看板列表刷新函数若叫别的名字，用现有函数。）

- [ ] **Step 2: 类型检查**

Run: `cd F:\projects\followITup\frontend && npx tsc --noEmit`
Expected: 无类型错误

- [ ] **Step 3: 提交**

```bash
git add frontend/src/pages/Dashboard.tsx
git commit -m "前端:首页回收站(头部按钮+弹窗列已删项目,点击恢复自动带出任务)"
```

---

### Task 5: 全量验证回归

**Files:** 无新增

- [ ] **Step 1: 后端全量测试 + 前端类型检查**

Run: `cd F:\projects\followITup\backend && go test ./... && cd ../frontend && npx tsc --noEmit`
Expected: 全部 PASS、无类型错误

- [ ] **Step 2: 影响范围检查**

Run: `gitnexus_detect_changes`（repo: followITup, scope: all）
Expected: 变更集中在 tasks.go/projects.go/ProjectGantt/Dashboard/RecycleBinModal；无意外 HIGH/CRITICAL

- [ ] **Step 3: 构建 + 浏览器验证**

```bash
cd frontend && npm run build
rm -rf ../backend/cmd/server/frontend-dist && cp -r dist ../backend/cmd/server/frontend-dist
cd ../backend && go build -o followitup.exe ./cmd/server/
```

浏览器验证清单：
1. 项目页（如项目 8）：删除一个任务 → 工具栏「回收站」→ 弹窗显示该任务（名称/删除时间/原日期/工期/进度/排序）→ 点击恢复 → 任务回到甘特图（带原日期）→ 弹窗列表刷新
2. 恢复任务后日期保持删除前值（不自动重排）；改该任务 duration → 排程正常接管
3. 首页：删除一个项目 → 看板「回收站」→ 弹窗显示该项目 → 点击恢复 → 项目回到看板，项目内任务全部可见
4. 恢复不存在/未删除的任务/项目 → 404 提示
5. 空态显示「没有已删除的任务/项目」

- [ ] **Step 4: 提交**

```bash
git add -A
git commit -m "验证:回收站功能浏览器回归通过"
```

（若验证发现问题，按 bug 流程先修再提交，并记录 .wolf/buglog.json）
