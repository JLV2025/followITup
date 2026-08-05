package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"followitup/internal/auth"
	"followitup/internal/models"
	"followitup/internal/util"

	"github.com/go-chi/chi/v5"
)

// ProjectHandler 项目与看板端点
type ProjectHandler struct {
	db               *sql.DB
	mid              *auth.Middleware
	fiscalStartMonth int
}

// NewProjectHandler 创建项目端点实例
func NewProjectHandler(db *sql.DB, mid *auth.Middleware, fiscalStartMonth int) *ProjectHandler {
	return &ProjectHandler{db: db, mid: mid, fiscalStartMonth: fiscalStartMonth}
}

// RegisterRoutes 注册路由
func (h *ProjectHandler) RegisterRoutes(r chi.Router) {
	// 看板统计（公开只读）
	r.Get("/api/dashboard/stats", h.DashboardStats)
	r.Get("/api/dashboard/projects", h.ProjectList)

	// 项目 CRUD（需登录）
	r.Group(func(r chi.Router) {
		r.Use(h.mid.RequireAuth)
		r.Get("/api/projects", h.ListProjects)               // ?deleted=1 已删项目
		r.Post("/api/projects", h.CreateProject)
		r.Post("/api/projects/{id}/restore", h.RestoreProject)
		r.Put("/api/projects/{id}", h.UpdateProject)
		r.Delete("/api/projects/{id}", h.DeleteProject)
		r.Get("/api/projects/{id}", h.GetProject)
		r.Post("/api/projects/{id}/members", h.AddMember)
		r.Delete("/api/projects/{id}/members/{userID}", h.RemoveMember)
	})
}

// DashboardStats 看板顶部统计
func (h *ProjectHandler) DashboardStats(w http.ResponseWriter, r *http.Request) {
	year := r.URL.Query().Get("year")
	fy := r.URL.Query().Get("fy")
	filter, args := h.buildTimeFilter("p", year, fy)

	var activeCount, atRiskCount, dueThisWeek int
	var overallPct float64

	// 活跃项目数
	query := "SELECT COUNT(*) FROM projects p WHERE p.deleted_at IS NULL AND p.status = 'active'" + filter
	h.db.QueryRow(query, args...).Scan(&activeCount)

	// 有风险/超期项目数
	query = `SELECT COUNT(DISTINCT p.id) FROM projects p
		JOIN tasks t ON t.project_id = p.id
		WHERE p.deleted_at IS NULL AND t.deleted_at IS NULL
		AND (t.status = 'delayed' OR t.status = 'overdue')` + filter
	h.db.QueryRow(query, args...).Scan(&atRiskCount)

	// 本周到期的任务数
	query = `SELECT COUNT(*) FROM tasks t
		JOIN projects p ON p.id = t.project_id
		WHERE p.deleted_at IS NULL AND t.deleted_at IS NULL
		AND t.status != 'completed'
		AND t.end_date >= date('now') AND t.end_date <= date('now', '+7 days')` + filter
	h.db.QueryRow(query, args...).Scan(&dueThisWeek)

	// 整体完成率（顶层任务时长加权，子任务进度经父任务汇总体现）
	query = `SELECT COALESCE(SUM(t.duration_days * t.progress_pct) / NULLIF(SUM(t.duration_days), 0), 0)
		FROM tasks t
		JOIN projects p ON p.id = t.project_id
		WHERE p.deleted_at IS NULL AND t.deleted_at IS NULL AND p.status = 'active'
		AND (t.parent_id IS NULL OR t.parent_id = 0)` + filter
	h.db.QueryRow(query, args...).Scan(&overallPct)

	// 基线完成率（顶层任务时长加权，baseline 口径；无基线时 SUM(NULL) → 0）
	var baselineProgress float64
	h.db.QueryRow(`SELECT COALESCE(SUM(t.baseline_progress_pct * t.baseline_duration_days) / NULLIF(SUM(t.baseline_duration_days), 0), 0)
		FROM tasks t WHERE t.project_id IN (SELECT id FROM projects p WHERE p.deleted_at IS NULL AND p.status = 'active'`+filter+`)
		AND t.deleted_at IS NULL AND (t.parent_id IS NULL OR t.parent_id = 0)`, args...).Scan(&baselineProgress)

	// 是否有基线（任一活跃项目打过基线快照；进度为 0 的基线也应显示 Δ 对比）
	var hasBaseline bool
	h.db.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM projects p WHERE p.deleted_at IS NULL AND p.status = 'active' AND p.baseline_created_at IS NOT NULL`+filter+`)`, args...).Scan(&hasBaseline)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"active_projects":   activeCount,
		"at_risk":           atRiskCount,
		"due_this_week":     dueThisWeek,
		"overall_progress":  int(overallPct),
		"baseline_progress": int(baselineProgress),
		"has_baseline":      hasBaseline,
	})
}

// ProjectList 项目列表（含看板所需摘要信息）
func (h *ProjectHandler) ProjectList(w http.ResponseWriter, r *http.Request) {
	year := r.URL.Query().Get("year")
	fy := r.URL.Query().Get("fy")
	filter, args := h.buildTimeFilter("p", year, fy)

	baseFilter := "WHERE p.deleted_at IS NULL AND p.status = 'active'" + filter

	query := `SELECT p.id, p.name, p.description, p.start_date, p.end_date, p.status, p.is_public,
		COALESCE(p.baseline_created_at, '') as baseline_created_at,
		COALESCE(p.baseline_created_by, '') as baseline_created_by,
			p.schedule_direction
		FROM projects p ` + baseFilter + ` ORDER BY
		CASE p.status WHEN 'active' THEN 0 ELSE 1 END, p.created_at DESC`

	rows, err := h.db.Query(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "查询项目失败")
		return
	}
	defer rows.Close()

	type ProjectSummary struct {
		models.Project
		TaskCount      int     `json:"task_count"`
		CompletedCount int     `json:"completed_count"`
		Progress       float64 `json:"progress"`
		NextMilestone  string  `json:"next_milestone"`
		RiskCount      int     `json:"risk_count"`
		HasRisk        bool    `json:"has_risk"`
		BaselineEnd    string  `json:"baseline_end"`
		DelayDays      int     `json:"delay_days"`
	}

	var projects []ProjectSummary
	for rows.Next() {
		var p ProjectSummary
		var isPublic int
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.StartDate, &p.EndDate,
			&p.Status, &isPublic, &p.BaselineCreatedAt, &p.BaselineCreatedBy, &p.ScheduleDirection); err != nil {
			continue
		}
		p.IsPublic = isPublic != 0
		// 补充任务统计
		h.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id = ? AND deleted_at IS NULL`, p.ID).Scan(&p.TaskCount)
		h.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id = ? AND deleted_at IS NULL AND status = 'completed'`, p.ID).Scan(&p.CompletedCount)
		// 项目进度：顶层任务时长加权（子任务进度经父任务汇总体现）
		h.db.QueryRow(`SELECT COALESCE(SUM(duration_days * progress_pct) / NULLIF(SUM(duration_days), 0), 0)
			FROM tasks WHERE project_id = ? AND deleted_at IS NULL AND (parent_id IS NULL OR parent_id = 0)`, p.ID).Scan(&p.Progress)
		h.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id = ? AND deleted_at IS NULL AND (status = 'delayed' OR status = 'overdue')`, p.ID).Scan(&p.RiskCount)
		p.HasRisk = p.RiskCount > 0
		// 下一里程碑
		h.db.QueryRow(`SELECT name FROM tasks WHERE project_id = ? AND deleted_at IS NULL AND task_type = 'milestone'
			AND status != 'completed' ORDER BY end_date ASC LIMIT 1`, p.ID).Scan(&p.NextMilestone)

		// 基线项目结束日期 + 偏差天数（无基线时保持零值）
		h.db.QueryRow(`SELECT COALESCE(MAX(baseline_end_date), '') FROM tasks WHERE project_id = ? AND deleted_at IS NULL`, p.ID).Scan(&p.BaselineEnd)
		if p.BaselineEnd != "" {
			h.db.QueryRow(`SELECT CAST(julianday(MAX(end_date)) - julianday(MAX(baseline_end_date)) AS INTEGER)
				FROM tasks WHERE project_id = ? AND deleted_at IS NULL`, p.ID).Scan(&p.DelayDays)
		}

		projects = append(projects, p)
	}

	writeJSON(w, http.StatusOK, projects)
}

// buildTimeFilter 构建时间过滤条件
// 支持 year（自然年）和 fy（财年）两种过滤方式，同时传时 fy 优先
func (h *ProjectHandler) buildTimeFilter(tableAlias string, year, fy string) (string, []interface{}) {
	if fy != "" {
		fyInt, err := strconv.Atoi(fy)
		if err != nil {
			return "", nil
		}
		start, end, err := util.FiscalYearRange(fyInt, h.fiscalStartMonth)
		if err != nil {
			return "", nil
		}
		return " AND " + tableAlias + ".created_at >= ? AND " + tableAlias + ".created_at <= ?",
			[]interface{}{start, end}
	}
	if year != "" {
		return " AND strftime('%Y', " + tableAlias + ".created_at) = ?",
			[]interface{}{year}
	}
	return "", nil
}

// CreateProject 创建项目
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var p models.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}
	if strings.TrimSpace(p.Name) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "项目名称不能为空")
		return
	}

	if p.ScheduleDirection == "" {
		p.ScheduleDirection = "forward" // 默认正推
	}
	result, err := h.db.Exec(
		`INSERT INTO projects (name, description, start_date, end_date, status, schedule_direction)
		 VALUES (?, ?, ?, ?, 'active', ?)`,
		p.Name, p.Description, p.StartDate, p.EndDate, p.ScheduleDirection,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "创建项目失败")
		return
	}

	id, _ := result.LastInsertId()
	p.ID = id
	p.Status = "active"

	// 自动将创建者添加为项目成员
	userID, ok := auth.GetUserID(r.Context())
	if ok {
		h.db.Exec("INSERT OR IGNORE INTO project_members (project_id, user_id, role) VALUES (?, ?, 'owner')", id, userID)
	}

	writeJSON(w, http.StatusCreated, p)
}

// UpdateProject 更新项目
func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	var p models.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}

	// 排程方向锁定：项目内任一任务有进度后不可修改方向
	var curDirection string
	h.db.QueryRow(`SELECT COALESCE(schedule_direction, 'forward') FROM projects WHERE id=? AND deleted_at IS NULL`, id).Scan(&curDirection)
	if p.ScheduleDirection != "" && p.ScheduleDirection != curDirection {
		var cnt int
		h.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id=? AND deleted_at IS NULL AND progress_pct > 0`, id).Scan(&cnt)
		if cnt > 0 {
			writeError(w, http.StatusBadRequest, "DIRECTION_LOCKED", "项目已有任务进度，排程方向不可修改")
			return
		}
	}
	if p.ScheduleDirection == "" {
		p.ScheduleDirection = curDirection // 请求体未携带时保留旧值
	}
	_, err := h.db.Exec(
		`UPDATE projects SET name=?, description=?, start_date=?, end_date=?, status=?, is_public=?,
		       schedule_direction=?, updated_at=datetime('now')
		 WHERE id=? AND deleted_at IS NULL`,
		p.Name, p.Description, p.StartDate, p.EndDate, p.Status, boolToInt(p.IsPublic),
		p.ScheduleDirection, id,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "更新项目失败")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "更新成功"})
}

// DeleteProject 软删除项目
func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	_, err := h.db.Exec("UPDATE projects SET deleted_at = datetime('now') WHERE id = ?", id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "删除项目失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已移至回收站"})
}

// GetProject 获取单个项目详情
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	var p models.Project
	var isPublic int
	err := h.db.QueryRow(
		"SELECT id, name, description, start_date, end_date, status, is_public, schedule_direction FROM projects WHERE id = ? AND deleted_at IS NULL",
		id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.StartDate, &p.EndDate, &p.Status, &isPublic, &p.ScheduleDirection)
	p.IsPublic = isPublic != 0
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "项目不存在")
		return
	}

	writeJSON(w, http.StatusOK, p)
}

// AddMember 添加项目成员
func (h *ProjectHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req struct {
		UserID int64  `json:"user_id"`
		Role   string `json:"role"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Role == "" {
		req.Role = "editor"
	}
	_, err := h.db.Exec("INSERT OR REPLACE INTO project_members (project_id, user_id, role) VALUES (?, ?, ?)",
		projectID, req.UserID, req.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "添加成员失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "成员已添加"})
}

// RemoveMember 移出项目成员
func (h *ProjectHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	userID, _ := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	h.db.Exec("DELETE FROM project_members WHERE project_id = ? AND user_id = ?", projectID, userID)
	writeJSON(w, http.StatusOK, map[string]string{"message": "成员已移除"})
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

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
