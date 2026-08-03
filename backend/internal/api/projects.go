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
		r.Post("/api/projects", h.CreateProject)
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
	query := "SELECT COUNT(*) FROM projects WHERE deleted_at IS NULL AND status = 'active'" + filter
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"active_projects":  activeCount,
		"at_risk":          atRiskCount,
		"due_this_week":    dueThisWeek,
		"overall_progress": int(overallPct),
	})
}

// ProjectList 项目列表（含看板所需摘要信息）
func (h *ProjectHandler) ProjectList(w http.ResponseWriter, r *http.Request) {
	year := r.URL.Query().Get("year")
	fy := r.URL.Query().Get("fy")
	filter, args := h.buildTimeFilter("p", year, fy)

	baseFilter := "WHERE p.deleted_at IS NULL AND p.status = 'active'" + filter

	query := `SELECT p.id, p.name, p.description, p.start_date, p.end_date, p.status, p.is_public
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
	}

	var projects []ProjectSummary
	for rows.Next() {
		var p ProjectSummary
		var isPublic int
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.StartDate, &p.EndDate,
			&p.Status, &isPublic); err != nil {
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

	result, err := h.db.Exec(
		`INSERT INTO projects (name, description, start_date, end_date, status) VALUES (?, ?, ?, ?, 'active')`,
		p.Name, p.Description, p.StartDate, p.EndDate,
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

	_, err := h.db.Exec(
		`UPDATE projects SET name=?, description=?, start_date=?, end_date=?, status=?, is_public=?, updated_at=datetime('now')
		 WHERE id=? AND deleted_at IS NULL`,
		p.Name, p.Description, p.StartDate, p.EndDate, p.Status, boolToInt(p.IsPublic), id,
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
		"SELECT id, name, description, start_date, end_date, status, is_public FROM projects WHERE id = ? AND deleted_at IS NULL",
		id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.StartDate, &p.EndDate, &p.Status, &isPublic)
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
