package api

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
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
	r.Get("/api/projects/{id}/tasks/deleted", h.ListDeletedTasks) // 回收站：必须注册在 /{taskID} 之前
	r.Get("/api/projects/{id}/tasks/{taskID}", h.GetTask)

	// 写操作（需登录）
	r.Group(func(r chi.Router) {
		r.Use(h.mid.RequireAuth)
		r.Post("/api/projects/{id}/tasks", h.CreateTask)
		r.Post("/api/projects/{id}/tasks/import", h.ImportTasks) // 必须注册在 /{taskID} 之前
		r.Post("/api/projects/{id}/tasks/{taskID}/restore", h.RestoreTask)
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
		        COALESCE(baseline_start_date, '') AS baseline_start_date, COALESCE(baseline_end_date, '') AS baseline_end_date,
		        COALESCE(baseline_duration_days, 0) AS baseline_duration_days, COALESCE(baseline_progress_pct, 0) AS baseline_progress_pct,
		        COALESCE(actual_start, '') AS actual_start, COALESCE(actual_end, '') AS actual_end,
		        manual_scheduled, constraint_type, constraint_date,
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

	// 关键路径：计算总浮动时间（不写库），TF=0 的任务即关键路径
	criticalIDs := []int64{}
	if tfMap, err := scheduler.ComputeTotalFloat(h.db, projectID); err == nil {
		for id, tf := range tfMap {
			if tf == 0 {
				criticalIDs = append(criticalIDs, id)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tasks":         tasks,
		"dependencies":  deps,
		"critical_ids":  criticalIDs,
	})
}

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

// RestoreTask 恢复已删除任务（软删除置空，恢复后触发排程实时重建依赖链）
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

	// 恢复后立即全项目重算（同排序保存）：恢复的任务完全纳入排程，
	// 日期按当前链重新推导、隐式顺序链按当前 sort_order 实时重建
	if _, err := scheduler.RecalculateAll(h.db, projectID); err != nil {
		log.Printf("[Scheduler] 恢复任务 %d 后项目 %d 重算失败: %v", taskID, projectID, err)
	}

	// 返回恢复后的任务（复用 GetTask 的查询逻辑：按 id 查询单任务并 JSON 返回）
	h.GetTask(w, r)
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
		        COALESCE(actual_start, '') AS actual_start, COALESCE(actual_end, '') AS actual_end,
		        manual_scheduled, constraint_type, constraint_date,
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

	if t.DurationDays < 1 && t.TaskType != "milestone" {
		writeError(w, http.StatusBadRequest, "INVALID_DURATION", "工期至少 1 天")
		return
	}

	// 坏编码防护：连续替换字符（GBK 终端误传中文的指纹）直接拒绝
	if hasBadEncoding(t.Name) || hasBadEncoding(t.Description) {
		writeError(w, http.StatusBadRequest, "INVALID_ENCODING", "名称/描述含非法字符（编码错误），请使用 UTF-8 输入")
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

	// 防呆：未指定负责人时默认取项目 owner
	if strings.TrimSpace(t.Assignee) == "" {
		var owner string
		h.db.QueryRow(`SELECT COALESCE(owner, '') FROM projects WHERE id=? AND deleted_at IS NULL`, projectID).Scan(&owner)
		t.Assignee = owner
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

	// 触发自动排程（正推：新任务无后继，Recalculate 传播链为空，无副作用；倒推：全量倒推对齐完成日期）
	if changed := triggersReschedule(t); changed {
		if _, err := scheduler.Recalculate(h.db, projectID, id); err != nil {
			log.Printf("[Scheduler] 项目 %d 重算失败: %v", projectID, err)
		}
	}

	h.broadcastChange(r, projectID, id)
	if t.ParentID != nil && *t.ParentID > 0 {
		h.recalcParentProgress(id)
	}

	writeJSON(w, http.StatusCreated, t)
}

// ImportTasks 批量导入任务（CSV，UTF-8 无 BOM；前端负责读文件并做 GBK 兜底解码）
// 表头：任务名, WBS编号, 工期(天), 开始日期, 负责人, 进度(%), 状态
// 层级由 WBS 编号推导（1 / 1.1 / 1.2.1 …），父必须在子之前出现；工期 0 或空 = 里程碑
func (h *TaskHandler) ImportTasks(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	var req struct {
		CSV string `json:"csv"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}

	reader := csv.NewReader(strings.NewReader(req.CSV))
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_CSV", "CSV 解析失败: "+err.Error())
		return
	}
	if len(records) < 2 {
		writeError(w, http.StatusBadRequest, "EMPTY_CSV", "CSV 至少需要表头 + 一行数据")
		return
	}
	records = records[1:] // 跳过表头

	// 状态映射：兼容中文与英文值
	statusMap := map[string]string{
		"未开始": "open", "open": "open",
		"进行中": "in_progress", "in_progress": "in_progress", "进行": "in_progress",
		"完成": "completed", "已完成": "completed", "completed": "completed",
		"延迟": "delayed", "delayed": "delayed", "超期": "delayed",
	}

	type importRow struct {
		wbs        string
		name       string
		duration   int
		startDate  string
		assignee   string
		progress   float64
		status     string
		taskType   string
		parentWBS  string
	}

	rows := make([]importRow, 0, len(records))
	skipped := 0
	skipReasons := []string{}
	for i, rec := range records {
		if len(rec) < 2 {
			skipped++
			skipReasons = append(skipReasons, fmt.Sprintf("第%d行:列数不足", i+2))
			continue
		}
		row := importRow{
			wbs:       strings.TrimSpace(rec[1]),
			name:      strings.TrimSpace(rec[0]),
			startDate: strings.TrimSpace(getCol(rec, 3)),
			assignee:  strings.TrimSpace(getCol(rec, 4)),
		}
		if row.name == "" {
			skipped++
			skipReasons = append(skipReasons, fmt.Sprintf("第%d行:任务名为空", i+2))
			continue
		}
		if hasBadEncoding(row.name) {
			skipped++
			skipReasons = append(skipReasons, fmt.Sprintf("第%d行:名称含非法字符(编码错误),请使用 UTF-8", i+2))
			continue
		}
		// 工期：0/空 = 里程碑
		if d := strings.TrimSpace(getCol(rec, 2)); d != "" && d != "0" {
			days, err := strconv.Atoi(d)
			if err != nil || days < 1 {
				skipped++
				skipReasons = append(skipReasons, fmt.Sprintf("第%d行:工期必须是正整数", i+2))
				continue
			}
			row.duration = days
			row.taskType = "task"
		} else {
			row.taskType = "milestone"
		}
		// 进度（%）
		if p := strings.TrimSpace(getCol(rec, 5)); p != "" {
			if v, err := strconv.ParseFloat(p, 64); err == nil {
				row.progress = clampFloat(v, 0, 100)
			}
		}
		// 状态（进度 100 兜底为已完成）
		row.status = statusMap[strings.ToLower(strings.TrimSpace(getCol(rec, 6)))]
		if row.status == "" {
			if row.progress >= 100 {
				row.status = "completed"
			} else {
				row.status = "open"
			}
		}
		// 父级 WBS：去掉最后一段（1.2.1 → 1.2；1 → 无父）
		if idx := strings.LastIndex(row.wbs, "."); idx > 0 {
			row.parentWBS = row.wbs[:idx]
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		writeError(w, http.StatusBadRequest, "EMPTY_CSV", "没有可导入的任务行")
		return
	}

	// 逐行插入（父必须在子之前出现，父 WBS 查 map）
	wbsToID := map[string]int64{}
	imported := 0
	for _, row := range rows {
		var parentID *int64
		if row.parentWBS != "" {
			pid, ok := wbsToID[row.parentWBS]
			if !ok {
				skipped++
				skipReasons = append(skipReasons, fmt.Sprintf("行[%s %s]:父级 WBS[%s] 不存在(需在子之前出现)", row.wbs, row.name, row.parentWBS))
				continue
			}
			parentID = &pid
		}
		// 排程:有开始日期+工期 → 算工作日 end_date;否则留空交给 RecalculateAll 自动排
		endDate := ""
		if row.startDate != "" && row.duration > 0 {
			endDate = scheduler.AddWorkDays(nil, row.startDate, row.duration)
		}
		// 防呆:负责人空 → 项目 owner
		assignee := row.assignee
		if strings.TrimSpace(assignee) == "" {
			var owner string
			h.db.QueryRow(`SELECT COALESCE(owner, '') FROM projects WHERE id=? AND deleted_at IS NULL`, projectID).Scan(&owner)
			assignee = owner
		}

		result, err := h.db.Exec(
			`INSERT INTO tasks (project_id, parent_id, name, description, task_type, status, priority,
			 assignee, start_date, end_date, duration_days, progress_pct, manual_scheduled,
			 constraint_type, constraint_date, sort_order)
			 SELECT ?, ?, ?, '', ?, ?, 'medium', ?, ?, ?, ?, ?, 0, '', '', COALESCE(MAX(sort_order), -1) + 1
			 FROM tasks WHERE project_id = ? AND deleted_at IS NULL`,
			projectID, parentID, row.name, row.taskType, row.status,
			assignee, row.startDate, endDate, row.duration, row.progress,
			projectID,
		)
		if err != nil {
			skipped++
			skipReasons = append(skipReasons, fmt.Sprintf("行[%s %s]:插入失败 %v", row.wbs, row.name, err))
			continue
		}
		id, _ := result.LastInsertId()
		wbsToID[row.wbs] = id
		imported++
	}

	// 排程 + 父进度重算 + 广播
	if imported > 0 {
		if _, err := scheduler.RecalculateAll(h.db, projectID); err != nil {
			log.Printf("[Scheduler] 导入后项目 %d 重算失败: %v", projectID, err)
		}
		for _, id := range wbsToID {
			h.recalcParentProgress(id)
		}
		h.broadcastChange(r, projectID, 0)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"imported": imported,
		"skipped":  skipped,
		"errors":   skipReasons,
	})
}

// getCol 取 CSV 行中第 idx 列（越界返回空串）
func getCol(rec []string, idx int) string {
	if idx < len(rec) {
		return rec[idx]
	}
	return ""
}

// clampFloat 将 v 限制在 [min, max]
func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
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

	if t.DurationDays < 1 && t.TaskType != "milestone" {
		writeError(w, http.StatusBadRequest, "INVALID_DURATION", "工期至少 1 天")
		return
	}

	// 坏编码防护：连续替换字符（GBK 终端误传中文的指纹）直接拒绝
	if hasBadEncoding(t.Name) || hasBadEncoding(t.Description) {
		writeError(w, http.StatusBadRequest, "INVALID_ENCODING", "名称/描述含非法字符（编码错误），请使用 UTF-8 输入")
		return
	}

	// 实际日期自动填充（需先读旧值，已有值不覆盖）
	var oldActualStart, oldActualEnd string
	h.db.QueryRow(`SELECT COALESCE(actual_start, ''), COALESCE(actual_end, '') FROM tasks WHERE id=? AND deleted_at IS NULL`, taskID).Scan(&oldActualStart, &oldActualEnd)
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
	// 排序变化 → 隐式顺序依赖变化 → 全项目重算（同分支相邻任务默认顺序衔接）
	if _, err := scheduler.RecalculateAll(h.db, projectID); err != nil {
		log.Printf("[Scheduler] 排序后项目 %d 重算失败: %v", projectID, err)
	}
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

	// 排程验证：若新依赖导致循环依赖（排程失败），回滚该依赖并明确提示——
	// 此前失败仅记日志，用户设置成环前置时甘特条不动且无任何提示
	if _, err := scheduler.Recalculate(h.db, projectID, dep.PredecessorID); err != nil {
		h.db.Exec("DELETE FROM dependencies WHERE predecessor_id = ? AND successor_id = ?",
			dep.PredecessorID, dep.SuccessorID)
		writeError(w, http.StatusBadRequest, "CIRCULAR_DEPENDENCY", "该前置会导致循环依赖，未添加：请检查前置任务链")
		return
	}
	h.broadcastChange(r, projectID, 0)

	writeJSON(w, http.StatusCreated, dep)
}

// DeleteDependency 删除依赖
func (h *TaskHandler) DeleteDependency(w http.ResponseWriter, r *http.Request) {
	depID, _ := strconv.ParseInt(chi.URLParam(r, "depID"), 10, 64)

	// 需要 projectID 来触发排程——从路由参数获取
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	h.db.Exec("DELETE FROM dependencies WHERE id = ?", depID)
	// 依赖结构变化 → 全量重算（此前 trigger=0 空转：删除依赖后链上任务日期不更新）
	if _, err := scheduler.RecalculateAll(h.db, projectID); err != nil {
		log.Printf("[Scheduler] 删除依赖后项目 %d 重算失败: %v", projectID, err)
	}
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
