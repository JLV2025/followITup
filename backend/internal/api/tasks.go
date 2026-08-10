package api

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
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
		r.Get("/api/tasks/mine", h.GetMyTasks) // 我的待办(登录用户负责的任务 + 未来一周开始)
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
		// 多负责人:权威在关联表,覆盖快照列并回填 ids
		t.AssigneeIDs, t.Assignee = loadTaskAssignees(h.db, t.ID)
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
		AssigneeIDs  []int64 `json:"assignee_ids"`
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
		// 多负责人:回填 ids(回收站列表只需数组,显示名仍用快照列)
		last := &tasks[len(tasks)-1]
		last.AssigneeIDs, _ = loadTaskAssignees(h.db, last.ID)
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
	// 多负责人:权威在关联表,覆盖快照列并回填 ids
	t.AssigneeIDs, t.Assignee = loadTaskAssignees(h.db, t.ID)

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

	// 实际日期默认跟随计划（实际开始=开始日，实际结束=结束日）；用户显式值优先
	t.ActualStart, t.ActualEnd = fillActualDates(t.ActualStart, t.ActualEnd, t.StartDate, t.EndDate)

	// 解析负责人:assignee_ids 数组优先,其次旧 assignee 文本,都未指定时默认取项目全部 owner
	var assigneeIDs []int64
	var missing []string
	if len(t.AssigneeIDs) > 0 {
		// 请求体直接传入 assignee_ids 数组(去重)
		seen := map[int64]bool{}
		for _, id := range t.AssigneeIDs {
			if !seen[id] {
				assigneeIDs = append(assigneeIDs, id)
				seen[id] = true
			}
		}
	} else if strings.TrimSpace(t.Assignee) != "" {
		assigneeIDs, missing = resolveUserIDs(h.db, splitOwnerNames(t.Assignee))
		if len(missing) > 0 {
			writeError(w, http.StatusBadRequest, "INVALID_OWNER", "负责人["+strings.Join(missing, ",")+"]不是系统用户,请从现有用户中选择")
			return
		}
	} else {
		rows, err := h.db.Query(`SELECT po.user_id FROM project_owners po JOIN projects p ON p.id = po.project_id WHERE p.id = ? AND p.deleted_at IS NULL`, projectID)
		if err == nil {
			for rows.Next() {
				var uid int64
				if rows.Scan(&uid) == nil {
					assigneeIDs = append(assigneeIDs, uid)
				}
			}
			rows.Close()
		}
	}
	// 快照列同步(写入后回填显示名)
	assigneeSnapshot := strings.Join(ownerNamesOf(h.db, assigneeIDs), "; ")

	// 分配下一个 sort_order（单条 INSERT...SELECT 原子完成，消除并发创建重复序号）
	result, err := h.db.Exec(
		`INSERT INTO tasks (project_id, parent_id, name, description, task_type, status, priority,
		 assignee, start_date, end_date, duration_days, progress_pct, actual_start, actual_end,
		 manual_scheduled, constraint_type, constraint_date, sort_order)
		 SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		        COALESCE(MAX(sort_order), -1) + 1
		 FROM tasks WHERE project_id = ? AND deleted_at IS NULL`,
		projectID, t.ParentID, t.Name, t.Description, t.TaskType, t.Status, t.Priority,
		assigneeSnapshot, t.StartDate, t.EndDate, t.DurationDays, t.ProgressPct, t.ActualStart, t.ActualEnd,
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

	// 写关联表(权威)并回填响应模型
	saveTaskAssignees(h.db, id, assigneeIDs)
	t.AssigneeIDs = assigneeIDs
	t.Assignee = assigneeSnapshot

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

	// 状态映射：兼容中文与英文值（含 UI 展示词"待开始/已延期"，与弹窗/列表一致）
	statusMap := map[string]string{
		"未开始": "open", "待开始": "open", "open": "open",
		"进行中": "in_progress", "进行": "in_progress", "in_progress": "in_progress",
		"完成": "completed", "已完成": "completed", "completed": "completed",
		"延迟": "delayed", "延期": "delayed", "已延期": "delayed", "delayed": "delayed", "超期": "delayed",
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
		// 进度（%）：Excel 常带 % 后缀；非法值跳过该行并提示，不静默置 0（避免状态兜底连带失效）
		if p := strings.TrimSpace(getCol(rec, 5)); p != "" {
			p = strings.TrimSuffix(strings.TrimSuffix(p, "%"), "％")
			v, err := strconv.ParseFloat(p, 64)
			if err != nil || v < 0 || v > 100 {
				skipped++
				skipReasons = append(skipReasons, fmt.Sprintf("第%d行:进度[%s]非法,须为 0-100 的数字", i+2, getCol(rec, 5)))
				continue
			}
			row.progress = v
		}
		// 状态（未知词跳过并提示，不静默兜底；状态列留空时按进度 100 兜底为已完成）
		statusCol := strings.TrimSpace(getCol(rec, 6))
		row.status = statusMap[strings.ToLower(statusCol)]
		if row.status == "" {
			if statusCol != "" {
				skipped++
				skipReasons = append(skipReasons, fmt.Sprintf("第%d行:未知状态[%s],可选:未开始/进行中/已完成/延迟(或 open/in_progress/completed/delayed)", i+2, statusCol))
				continue
			}
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
	// sort_order 基数先查一次（不能用 INSERT...SELECT 子查询——空项目时 SELECT 空集
	// 导致 INSERT 0 行且不报错，导入静默全丢）
	var baseSort int64 = -1
	h.db.QueryRow(`SELECT COALESCE(MAX(sort_order), -1) FROM tasks WHERE project_id=? AND deleted_at IS NULL`, projectID).Scan(&baseSort)
	nextSort := baseSort + 1
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
		// 重复 WBS：跳过并提示（否则子任务会全挂到最后一个，先插入的变孤儿）
		if _, dup := wbsToID[row.wbs]; dup {
			skipped++
			skipReasons = append(skipReasons, fmt.Sprintf("行[%s %s]:WBS[%s] 重复,已跳过", row.wbs, row.name, row.wbs))
			continue
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
		// 实际日期默认跟随计划（与 CreateTask 新语义一致：用户选择 > 系统默认）
		actualStart, actualEnd := fillActualDates("", "", row.startDate, endDate)

		result, err := h.db.Exec(
			`INSERT INTO tasks (project_id, parent_id, name, description, task_type, status, priority,
			 assignee, start_date, end_date, duration_days, progress_pct, manual_scheduled,
			 constraint_type, constraint_date, sort_order, actual_start, actual_end)
			 VALUES (?, ?, ?, '', ?, ?, 'medium', ?, ?, ?, ?, ?, 0, '', '', ?, ?, ?)`,
			projectID, parentID, row.name, row.taskType, row.status,
			assignee, row.startDate, endDate, row.duration, row.progress,
			nextSort, actualStart, actualEnd,
		)
		if err != nil {
			skipped++
			skipReasons = append(skipReasons, fmt.Sprintf("行[%s %s]:插入失败 %v", row.wbs, row.name, err))
			continue
		}
		nextSort++
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

// MyTaskItem 我的待办条目（含项目名）
type MyTaskItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	ProjectName string `json:"project_name"`
	Status      string `json:"status"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	ProgressPct float64 `json:"progress_pct"`
	Assignee    string `json:"assignee"`
}

// GetMyTasks 我的待办：view=task(默认)我名下的任务;view=project 我名下项目的任务。
// 每个视角两个分区:mine(进行中)+ starting(未来 N 天开始,未完成)。
func (h *TaskHandler) GetMyTasks(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}
	view := r.URL.Query().Get("view")
	if view != "project" {
		view = "task"
	}

	days := 7
	if d, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && d >= 1 && d <= 30 {
		days = d
	}
	today := time.Now().Format("2006-01-02")
	windowEnd := time.Now().AddDate(0, 0, days).Format("2006-01-02")

	// 待办任务限定条件:task 视角=我负责的任务;project 视角=我负责的项目的任务
	scope := `EXISTS (SELECT 1 FROM task_assignees ta WHERE ta.task_id = t.id AND ta.user_id = ?)`
	scopeArg := userID
	if view == "project" {
		scope = `EXISTS (SELECT 1 FROM project_owners po JOIN projects pj ON pj.id = po.project_id WHERE po.project_id = t.project_id AND po.user_id = ? AND pj.deleted_at IS NULL)`
	}

	var mine []MyTaskItem
	rows, err := h.db.Query(`
		SELECT t.id, t.name, p.name, t.status, COALESCE(t.start_date, ''), COALESCE(t.end_date, ''), t.progress_pct
		FROM tasks t
		JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
		WHERE t.deleted_at IS NULL AND t.status = 'in_progress' AND `+scope+`
		ORDER BY CASE WHEN t.end_date = '' THEN 1 ELSE 0 END, t.end_date`, scopeArg)
	if err == nil {
		for rows.Next() {
			var it MyTaskItem
			if rows.Scan(&it.ID, &it.Name, &it.ProjectName, &it.Status, &it.StartDate, &it.EndDate, &it.ProgressPct) == nil {
				_, it.Assignee = loadTaskAssignees(h.db, it.ID)
				mine = append(mine, it)
			}
		}
		rows.Close()
	}

	var starting []MyTaskItem
	rows2, err := h.db.Query(`
		SELECT t.id, t.name, p.name, t.status, COALESCE(t.start_date, ''), COALESCE(t.end_date, ''), t.progress_pct
		FROM tasks t
		JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
		WHERE t.deleted_at IS NULL AND t.status != 'completed' AND `+scope+`
		  AND t.start_date >= ? AND t.start_date <= ?
		ORDER BY t.start_date`, scopeArg, today, windowEnd)
	if err == nil {
		for rows2.Next() {
			var it MyTaskItem
			if rows2.Scan(&it.ID, &it.Name, &it.ProjectName, &it.Status, &it.StartDate, &it.EndDate, &it.ProgressPct) == nil {
				_, it.Assignee = loadTaskAssignees(h.db, it.ID)
				starting = append(starting, it)
			}
		}
		rows2.Close()
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"mine":     mine,
		"starting": starting,
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

	// 先整体读入再两次解析：请求体只传部分字段（脚本/集成）时，
	// actual_* 缺失须保留 DB 旧值，不能用零值覆盖用户手填的实际日期
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}
	var t models.Task
	if err := json.Unmarshal(raw, &t); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}
	var fieldCheck map[string]json.RawMessage
	json.Unmarshal(raw, &fieldCheck)
	if _, provided := fieldCheck["actual_start"]; !provided {
		h.db.QueryRow(`SELECT COALESCE(actual_start, '') FROM tasks WHERE id=? AND deleted_at IS NULL`, taskID).Scan(&t.ActualStart)
	}
	if _, provided := fieldCheck["actual_end"]; !provided {
		h.db.QueryRow(`SELECT COALESCE(actual_end, '') FROM tasks WHERE id=? AND deleted_at IS NULL`, taskID).Scan(&t.ActualEnd)
	}

	// 负责人:assignee_ids/assignee 都未携带 → 保留 DB 旧关联(与 actual_* 同策略)
	var assigneeIDs []int64
	if _, ok := fieldCheck["assignee_ids"]; ok {
		json.Unmarshal(fieldCheck["assignee_ids"], &assigneeIDs)
		// 去重(与 CreateTask 一致)
		seen := map[int64]bool{}
		var deduped []int64
		for _, id := range assigneeIDs {
			if !seen[id] {
				deduped = append(deduped, id)
				seen[id] = true
			}
		}
		assigneeIDs = deduped
	} else if _, ok := fieldCheck["assignee"]; ok {
		var missing []string
		assigneeIDs, missing = resolveUserIDs(h.db, splitOwnerNames(t.Assignee))
		if len(missing) > 0 {
			writeError(w, http.StatusBadRequest, "INVALID_OWNER", "负责人["+strings.Join(missing, ",")+"]不是系统用户,请从现有用户中选择")
			return
		}
	} else {
		assigneeIDs, _ = loadTaskAssignees(h.db, taskID)
	}
	t.AssigneeIDs = assigneeIDs

	if t.DurationDays < 1 && t.TaskType != "milestone" {
		writeError(w, http.StatusBadRequest, "INVALID_DURATION", "工期至少 1 天")
		return
	}

	// 坏编码防护：连续替换字符（GBK 终端误传中文的指纹）直接拒绝
	if hasBadEncoding(t.Name) || hasBadEncoding(t.Description) {
		writeError(w, http.StatusBadRequest, "INVALID_ENCODING", "名称/描述含非法字符（编码错误），请使用 UTF-8 输入")
		return
	}

	// 实际日期：用户选择优先；未填则默认取计划日期（实际开始=开始日，实际结束=结束日）。
	// 注意：请求体未传 actual_* 时上面已保留 DB 旧值，此处兜底只作用于"显式传空 = 恢复默认跟随计划"
	t.ActualStart, t.ActualEnd = fillActualDates(t.ActualStart, t.ActualEnd, t.StartDate, t.EndDate)
	// 逻辑校验：实际结束不能早于实际开始（超出计划范围允许——提前/延期正是偏差可视化要表达的）
	if t.ActualStart != "" && t.ActualEnd != "" && t.ActualEnd < t.ActualStart {
		writeError(w, http.StatusBadRequest, "INVALID_ACTUAL", "实际结束不能早于实际开始(如未填写实际结束,请同时设置)")
		return
	}

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

	// 负责人快照列:以关联表为准(等写关联表后回填;这里先按当前 ids 拼,写关联表用同一批)
	t.Assignee = strings.Join(ownerNamesOf(h.db, assigneeIDs), "; ")

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

	// 负责人字段在请求体中提供过才写关联表(未提供时上面的 loadTaskAssignees 已保留旧关联,无需重写)
	_, providedIDs := fieldCheck["assignee_ids"]
	_, providedText := fieldCheck["assignee"]
	if providedIDs || providedText {
		saveTaskAssignees(h.db, taskID, assigneeIDs)
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

// fillActualDates 实际日期默认取计划日期（实际开始=开始日，实际结束=结束日）；
// 用户显式填写的值优先，留空则用计划日期（用户选择 > 系统默认）
func fillActualDates(actualStart, actualEnd, planStart, planEnd string) (string, string) {
	if actualStart == "" && planStart != "" {
		actualStart = planStart
	}
	if actualEnd == "" && planEnd != "" {
		actualEnd = planEnd
	}
	return actualStart, actualEnd
}

// splitOwnerNames 拆分负责人文本(分号/逗号/空白分隔),去重保序
func splitOwnerNames(s string) []string {
	raw := strings.FieldsFunc(s, func(r rune) bool { return r == ';' || r == ',' || r == '\n' })
	var out []string
	seen := map[string]bool{}
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// resolveUserIDs 逐名解析活跃用户(email 精确匹配优先,其次 display_name,重名取 id 最小),返回成功 ids 与失败名
func resolveUserIDs(db *sql.DB, names []string) ([]int64, []string) {
	var ids []int64
	var missing []string
	seen := map[int64]bool{}
	for _, name := range names {
		var uid int64
		err := db.QueryRow(`SELECT id FROM users WHERE is_active = 1 AND email = ? LIMIT 1`, name).Scan(&uid)
		if err != nil {
			err = db.QueryRow(`SELECT id FROM users WHERE is_active = 1 AND display_name = ? ORDER BY id LIMIT 1`, name).Scan(&uid)
		}
		if err != nil || seen[uid] {
			if err != nil {
				missing = append(missing, name)
			}
			continue
		}
		seen[uid] = true
		ids = append(ids, uid)
	}
	return ids, missing
}

// saveTaskAssignees 覆盖写任务负责人:事务删旧插新(去重),返回分号分隔显示名快照
func saveTaskAssignees(db *sql.DB, taskID int64, ids []int64) string {
	if len(ids) > 0 {
		seen := map[int64]bool{}
		var uniq []int64
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				uniq = append(uniq, id)
			}
		}
		ids = uniq
	}
	tx, err := db.Begin()
	if err != nil {
		return ""
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM task_assignees WHERE task_id = ?`, taskID); err != nil {
		return ""
	}
	for _, id := range ids {
		tx.Exec(`INSERT OR IGNORE INTO task_assignees (task_id, user_id) VALUES (?, ?)`, taskID, id)
	}
	var snap string
	tx.QueryRow(`SELECT GROUP_CONCAT(u.display_name, '; ') FROM task_assignees ta JOIN users u ON u.id = ta.user_id WHERE ta.task_id = ?`, taskID).Scan(&snap)
	tx.Commit()
	return snap
}

// saveProjectOwners 项目负责人覆盖写(同上)
func saveProjectOwners(db *sql.DB, projectID int64, ids []int64) string {
	if len(ids) > 0 {
		seen := map[int64]bool{}
		var uniq []int64
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				uniq = append(uniq, id)
			}
		}
		ids = uniq
	}
	tx, err := db.Begin()
	if err != nil {
		return ""
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM project_owners WHERE project_id = ?`, projectID); err != nil {
		return ""
	}
	for _, id := range ids {
		tx.Exec(`INSERT OR IGNORE INTO project_owners (project_id, user_id) VALUES (?, ?)`, projectID, id)
	}
	var snap string
	tx.QueryRow(`SELECT GROUP_CONCAT(u.display_name, '; ') FROM project_owners po JOIN users u ON u.id = po.user_id WHERE po.project_id = ?`, projectID).Scan(&snap)
	tx.Commit()
	return snap
}

// loadTaskAssignees 读取任务负责人:ids + 分号分隔显示名快照
func loadTaskAssignees(db *sql.DB, taskID int64) ([]int64, string) {
	rows, err := db.Query(`SELECT ta.user_id, u.display_name FROM task_assignees ta JOIN users u ON u.id = ta.user_id WHERE ta.task_id = ? ORDER BY ta.user_id`, taskID)
	if err != nil {
		return nil, ""
	}
	defer rows.Close()
	var ids []int64
	var names []string
	for rows.Next() {
		var id int64
		var name string
		if rows.Scan(&id, &name) == nil {
			ids = append(ids, id)
			names = append(names, name)
		}
	}
	return ids, strings.Join(names, "; ")
}

// loadProjectOwners 项目负责人(同上)
func loadProjectOwners(db *sql.DB, projectID int64) ([]int64, string) {
	rows, err := db.Query(`SELECT po.user_id, u.display_name FROM project_owners po JOIN users u ON u.id = po.user_id WHERE po.project_id = ? ORDER BY po.user_id`, projectID)
	if err != nil {
		return nil, ""
	}
	defer rows.Close()
	var ids []int64
	var names []string
	for rows.Next() {
		var id int64
		var name string
		if rows.Scan(&id, &name) == nil {
			ids = append(ids, id)
			names = append(names, name)
		}
	}
	return ids, strings.Join(names, "; ")
}

// ownerNamesOf 按 user_id 数组返回显示名(与传入顺序一致,缺失跳过)
func ownerNamesOf(db *sql.DB, ids []int64) []string {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := db.Query(`SELECT id, display_name FROM users WHERE id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	nameByID := map[int64]string{}
	for rows.Next() {
		var id int64
		var name string
		if rows.Scan(&id, &name) == nil {
			nameByID[id] = name
		}
	}
	var out []string
	for _, id := range ids {
		if n, ok := nameByID[id]; ok {
			out = append(out, n)
		}
	}
	return out
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
