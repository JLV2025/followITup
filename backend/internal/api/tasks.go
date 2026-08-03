package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"followitup/internal/auth"
	"followitup/internal/models"
	"followitup/internal/scheduler"
	"followitup/internal/ws"

	"github.com/go-chi/chi/v5"
)

// TaskHandler 任务 CRUD 端点
type TaskHandler struct {
	db  *sql.DB
	mid *auth.Middleware
	hub *ws.Hub
}

// NewTaskHandler 创建任务端点实例
func NewTaskHandler(db *sql.DB, mid *auth.Middleware, hub *ws.Hub) *TaskHandler {
	return &TaskHandler{db: db, mid: mid, hub: hub}
}

// RegisterRoutes 注册路由
func (h *TaskHandler) RegisterRoutes(r chi.Router) {
	// 只读（公开或可选认证）
	r.Get("/api/projects/{id}/tasks", h.ListTasks)
	r.Get("/api/projects/{id}/tasks/{taskID}", h.GetTask)

	// 写操作（需登录）
	r.Group(func(r chi.Router) {
		r.Use(h.mid.RequireAuth)
		r.Post("/api/projects/{id}/tasks", h.CreateTask)
		r.Put("/api/projects/{id}/tasks/{taskID}", h.UpdateTask)
		r.Patch("/api/projects/{id}/tasks/{taskID}/sort_order", h.UpdateTaskSortOrder)
		r.Delete("/api/projects/{id}/tasks/{taskID}", h.DeleteTask)
		r.Post("/api/projects/{id}/dependencies", h.AddDependency)
		r.Delete("/api/projects/{id}/dependencies/{depID}", h.DeleteDependency)
	})
}

// ListTasks 获取项目的所有任务
func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	rows, err := h.db.Query(
		`SELECT id, project_id, COALESCE(parent_id, 0), name, description, task_type, status, priority,
		        assignee, start_date, end_date, duration_days, progress_pct,
		        baseline_start_date, baseline_end_date, baseline_duration_days, baseline_progress_pct,
		        actual_start, actual_end, manual_scheduled, constraint_type, constraint_date,
		        sort_order, version,
		        COALESCE(deleted_at, ''), created_at, updated_at
		 FROM tasks WHERE project_id = ? AND deleted_at IS NULL
		 ORDER BY sort_order, id`, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "查询任务失败")
		return
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		var parentID int64
		var manualSched int
		var deletedAt string
		if err := rows.Scan(&t.ID, &t.ProjectID, &parentID, &t.Name, &t.Description,
			&t.TaskType, &t.Status, &t.Priority, &t.Assignee,
			&t.StartDate, &t.EndDate, &t.DurationDays, &t.ProgressPct,
				&t.BaselineStartDate, &t.BaselineEndDate, &t.BaselineDurationDays, &t.BaselineProgressPct,
			&t.ActualStart, &t.ActualEnd, &manualSched, &t.ConstraintType, &t.ConstraintDate,
			&t.SortOrder, &t.Version,
			&deletedAt, &t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		if parentID > 0 {
			t.ParentID = &parentID
		}
		t.ManualScheduled = manualSched != 0
		tasks = append(tasks, t)
	}

	// 也加载依赖关系
	deps, _ := h.loadDependencies(projectID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tasks":        tasks,
		"dependencies": deps,
	})
}

// GetTask 获取单个任务
func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	taskID, _ := strconv.ParseInt(chi.URLParam(r, "taskID"), 10, 64)

	var t models.Task
	var parentID int64
	var manualSched int
	var deletedAt string
	err := h.db.QueryRow(
		`SELECT id, project_id, COALESCE(parent_id, 0), name, description, task_type, status, priority,
		        assignee, start_date, end_date, duration_days, progress_pct,
		        actual_start, actual_end, manual_scheduled, constraint_type, constraint_date,
		        sort_order, version,
		        COALESCE(deleted_at, ''), created_at, updated_at
		 FROM tasks WHERE id = ? AND deleted_at IS NULL`, taskID,
	).Scan(&t.ID, &t.ProjectID, &parentID, &t.Name, &t.Description,
		&t.TaskType, &t.Status, &t.Priority, &t.Assignee,
		&t.StartDate, &t.EndDate, &t.DurationDays, &t.ProgressPct,
		&t.ActualStart, &t.ActualEnd, &manualSched, &t.ConstraintType, &t.ConstraintDate,
		&t.SortOrder, &t.Version,
		&deletedAt, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "任务不存在")
		return
	}
	if parentID > 0 {
		t.ParentID = &parentID
	}
	t.ManualScheduled = manualSched != 0

	writeJSON(w, http.StatusOK, t)
}

// CreateTask 创建任务
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	var t models.Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}

	// 校验 parent_id
	if t.ParentID != nil {
		if err := h.validateParent(*t.ParentID, projectID, 0); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARENT", err.Error())
			return
		}
	}

	// 计算工作日 end_date
	if t.StartDate != "" && t.DurationDays > 0 && t.EndDate == "" {
		t.EndDate = scheduler.AddWorkDays(nil, t.StartDate, t.DurationDays)
	}

	// 分配下一个 sort_order（单条 INSERT...SELECT 原子完成，消除并发创建重复序号）
	result, err := h.db.Exec(
		`INSERT INTO tasks (project_id, parent_id, name, description, task_type, status, priority,
		 assignee, start_date, end_date, duration_days, progress_pct, manual_scheduled,
		 constraint_type, constraint_date, sort_order)
		 SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		        COALESCE(MAX(sort_order), -1) + 1
		 FROM tasks WHERE project_id = ? AND deleted_at IS NULL`,
		projectID, t.ParentID, t.Name, t.Description, t.TaskType, t.Status, t.Priority,
		t.Assignee, t.StartDate, t.EndDate, t.DurationDays, t.ProgressPct,
		boolToInt2(t.ManualScheduled), t.ConstraintType, t.ConstraintDate,
		projectID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "创建任务失败")
		return
	}

	id, _ := result.LastInsertId()
	t.ID = id
	t.ProjectID = projectID

	h.broadcastChange(r, projectID, id)
	if t.ParentID != nil && *t.ParentID > 0 {
		h.recalcParentProgress(id)
	}

	writeJSON(w, http.StatusCreated, t)
}

// UpdateTask 更新任务（含乐观锁）
func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	taskID, _ := strconv.ParseInt(chi.URLParam(r, "taskID"), 10, 64)
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	var t models.Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}

	// 实际日期自动填充（需先读旧值，已有值不覆盖）
	var oldActualStart, oldActualEnd string
	h.db.QueryRow(`SELECT actual_start, actual_end FROM tasks WHERE id=? AND deleted_at IS NULL`, taskID).Scan(&oldActualStart, &oldActualEnd)
	t.ActualStart, t.ActualEnd = fillActualDates(t.Status, oldActualStart, oldActualEnd)

	// URL 参数回填（请求体不含 id/project_id，排程级联需要）
	t.ID = taskID
	t.ProjectID = projectID

	// 校验 parent_id
	if t.ParentID != nil {
		if err := h.validateParent(*t.ParentID, projectID, taskID); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARENT", err.Error())
			return
		}
	}

	oldVersion := t.Version
	t.Version = oldVersion + 1

	result, err := h.db.Exec(
		`UPDATE tasks SET
		 name=?, description=?, task_type=?, status=?, priority=?, assignee=?,
		 start_date=?, end_date=?, duration_days=?, progress_pct=?,
		 actual_start=?, actual_end=?, manual_scheduled=?, constraint_type=?, constraint_date=?,
		 parent_id=?, sort_order=?, version=?, updated_at=datetime('now')
		 WHERE id=? AND deleted_at IS NULL AND version=?`,
		t.Name, t.Description, t.TaskType, t.Status, t.Priority, t.Assignee,
		t.StartDate, t.EndDate, t.DurationDays, t.ProgressPct,
		t.ActualStart, t.ActualEnd, boolToInt2(t.ManualScheduled), t.ConstraintType, t.ConstraintDate,
		t.ParentID, t.SortOrder, t.Version,
		taskID, oldVersion,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "更新任务失败")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusConflict, "CONFLICT", "任务已被他人修改，请刷新后重试")
		return
	}

	// 触发自动排程（同步执行，确保结果落库后再返回前端）
	if changed := triggersReschedule(t); changed {
		if _, err := scheduler.Recalculate(h.db, t.ProjectID, t.ID); err != nil {
			log.Printf("[Scheduler] 项目 %d 重算失败: %v", t.ProjectID, err)
		}
	}

	// 广播变更给项目内其他用户
	h.broadcastChange(r, t.ProjectID, taskID)
	if t.ParentID != nil && *t.ParentID > 0 {
		h.recalcParentProgress(taskID)
	}

	writeJSON(w, http.StatusOK, t)
}

// UpdateTaskSortOrder 仅更新排序序号（拖拽排序用），带乐观锁。
// 不触发排程、不覆盖其他字段——UpdateTask 是全列覆盖 UPDATE，不能用于排序。
func (h *TaskHandler) UpdateTaskSortOrder(w http.ResponseWriter, r *http.Request) {
	taskID, _ := strconv.ParseInt(chi.URLParam(r, "taskID"), 10, 64)
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	var body struct {
		SortOrder int `json:"sort_order"`
		Version   int `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}

	result, err := h.db.Exec(
		`UPDATE tasks SET sort_order=?, version=version+1, updated_at=datetime('now')
		 WHERE id=? AND project_id=? AND deleted_at IS NULL AND version=?`,
		body.SortOrder, taskID, projectID, body.Version)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "更新排序失败")
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusConflict, "CONFLICT", "任务已被他人修改，请刷新后重试")
		return
	}

	h.broadcastChange(r, projectID, taskID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":         taskID,
		"sort_order": body.SortOrder,
		"version":    body.Version + 1,
	})
}

// fillActualDates 实际日期自动填充：
// status 变为 in_progress 且无实际开始 → 记今天；变为 completed 且无实际结束 → 记今天；已有值不覆盖
func fillActualDates(status, actualStart, actualEnd string) (string, string) {
	today := time.Now().Format("2006-01-02")
	if status == "in_progress" && actualStart == "" {
		actualStart = today
	}
	if status == "completed" && actualEnd == "" {
		actualEnd = today
	}
	return actualStart, actualEnd
}

// DeleteTask 软删除任务，同时清理关联的依赖关系
func (h *TaskHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	taskID, _ := strconv.ParseInt(chi.URLParam(r, "taskID"), 10, 64)
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	// 记录父任务 ID，删除后重算进度
	var oldParentID int64
	h.db.QueryRow("SELECT COALESCE(parent_id, 0) FROM tasks WHERE id = ?", taskID).Scan(&oldParentID)

	// 软删除任务本身
	h.db.Exec("UPDATE tasks SET deleted_at = datetime('now') WHERE id = ?", taskID)

	// 清理引用此任务的所有依赖（作为前置或后置）
	h.db.Exec("DELETE FROM dependencies WHERE predecessor_id = ? OR successor_id = ?", taskID, taskID)

	// 删除后触发排程，确保受影响的后继任务日期重算
	h.triggerReschedule(projectID, 0)
	h.broadcastChange(r, projectID, taskID)
	if oldParentID > 0 {
		h.recalcParentProgress(taskID)
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已移至回收站"})
}

// AddDependency 添加任务依赖
func (h *TaskHandler) AddDependency(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var dep models.Dependency
	json.NewDecoder(r.Body).Decode(&dep)
	if dep.DepType == "" {
		dep.DepType = "FS"
	}
	_, err := h.db.Exec(
		"INSERT OR IGNORE INTO dependencies (predecessor_id, successor_id, dep_type, lag_days) VALUES (?, ?, ?, ?)",
		dep.PredecessorID, dep.SuccessorID, dep.DepType, dep.LagDays,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "创建依赖失败")
		return
	}

	h.triggerReschedule(projectID, dep.PredecessorID)
	h.broadcastChange(r, projectID, 0)

	writeJSON(w, http.StatusCreated, dep)
}

// DeleteDependency 删除依赖
func (h *TaskHandler) DeleteDependency(w http.ResponseWriter, r *http.Request) {
	depID, _ := strconv.ParseInt(chi.URLParam(r, "depID"), 10, 64)

	// 需要 projectID 来触发排程——从路由参数获取
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	h.db.Exec("DELETE FROM dependencies WHERE id = ?", depID)
	h.triggerReschedule(projectID, 0)
	h.broadcastChange(r, projectID, 0)

	writeJSON(w, http.StatusOK, map[string]string{"message": "依赖已删除"})
}

// loadDependencies 加载项目的所有依赖（排除已删除任务的依赖）
func (h *TaskHandler) loadDependencies(projectID int64) ([]models.Dependency, error) {
	rows, err := h.db.Query(
		`SELECT d.id, d.predecessor_id, d.successor_id, d.dep_type, d.lag_days
		 FROM dependencies d
		 JOIN tasks t ON t.id = d.predecessor_id
		 WHERE t.project_id = ? AND t.deleted_at IS NULL
		 ORDER BY d.id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []models.Dependency
	for rows.Next() {
		var d models.Dependency
		if err := rows.Scan(&d.ID, &d.PredecessorID, &d.SuccessorID, &d.DepType, &d.LagDays); err == nil {
			deps = append(deps, d)
		}
	}
	return deps, nil
}

// triggersReschedule 判断任务变更是否需要触发排程
func triggersReschedule(t models.Task) bool {
	// 日期、工期、类型变更 → 触发
	return t.StartDate != "" || t.EndDate != "" || t.DurationDays > 0 ||
		t.TaskType == "milestone" || t.TaskType == "task"
}

// broadcastChange 广播任务变更通知给项目房间内的其他用户
func (h *TaskHandler) broadcastChange(r *http.Request, projectID int64, taskID int64) {
	if h.hub == nil {
		return
	}
	userID, _ := auth.GetUserID(r.Context())
	userName, _ := auth.GetUserEmail(r.Context())
	h.hub.BroadcastTaskUpdate(projectID, userID, userName, taskID, nil)
}

func boolToInt2(b bool) int {
	if b {
		return 1
	}
	return 0
}

// validateParent 校验 parent_id 合法性
func (h *TaskHandler) validateParent(parentID, projectID, excludeID int64) error {
	// 不能设置为自己
	if parentID == excludeID {
		return fmt.Errorf("不能将自己设为子任务")
	}

	var parentProjectID int64
	err := h.db.QueryRow("SELECT project_id FROM tasks WHERE id = ? AND deleted_at IS NULL", parentID).Scan(&parentProjectID)
	if err != nil {
		return fmt.Errorf("父任务不存在")
	}
	if parentProjectID != projectID {
		return fmt.Errorf("父任务不属于同一项目")
	}

	return nil
}

// triggerReschedule 触发排程（同步执行）
func (h *TaskHandler) triggerReschedule(projectID int64, taskID int64) {
	if _, err := scheduler.Recalculate(h.db, projectID, taskID); err != nil {
		log.Printf("[Scheduler] 项目 %d 重算失败: %v", projectID, err)
	}
}

// recalcParentProgress 递归向上重算父任务进度（时长加权平均）
// 无子任务的任务保持原进度不变（手动设置）
func (h *TaskHandler) recalcParentProgress(taskID int64) {
	var parentID int64
	h.db.QueryRow("SELECT COALESCE(parent_id, 0) FROM tasks WHERE id = ? AND deleted_at IS NULL", taskID).Scan(&parentID)
	if parentID == 0 {
		return
	}

	// 时长加权平均：SUM(duration_days * progress_pct) / SUM(duration_days)
	var weightedPct float64
	h.db.QueryRow(
		`SELECT COALESCE(SUM(duration_days * progress_pct) / NULLIF(SUM(duration_days), 0), 0)
		 FROM tasks WHERE parent_id = ? AND deleted_at IS NULL`, parentID,
	).Scan(&weightedPct)

	h.db.Exec("UPDATE tasks SET progress_pct = ?, updated_at = datetime('now') WHERE id = ? AND deleted_at IS NULL",
		weightedPct, parentID)

	// 递归向上
	h.recalcParentProgress(parentID)
}
