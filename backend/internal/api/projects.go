package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"followitup/internal/auth"
	"followitup/internal/models"
	"followitup/internal/scheduler"
	"followitup/internal/settings"
	"followitup/internal/util"
	"followitup/internal/ws"

	"github.com/go-chi/chi/v5"
)

// ProjectHandler 项目与看板端点
type ProjectHandler struct {
	db  *sql.DB
	mid *auth.Middleware
	hub *ws.Hub
}

// NewProjectHandler 创建项目端点实例（财年起始月已迁移至 settings 表动态读取）
func NewProjectHandler(db *sql.DB, mid *auth.Middleware, hub *ws.Hub) *ProjectHandler {
	return &ProjectHandler{db: db, mid: mid, hub: hub}
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
		r.Post("/api/projects/{id}/copy", h.CopyProject) // 深拷贝项目
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
// 注意：不再支持 year/fy 过滤——状态总览"进行中"需全量跨年显示，"已完成"按结束日期归属年度在前端过滤
func (h *ProjectHandler) ProjectList(w http.ResponseWriter, r *http.Request) {
	// 返回全部状态（active/completed），前端负责"隐藏已完成"过滤；
	// 排序：老 → 新（创建时间升序），看板编号按此顺序
	query := `SELECT p.id, p.name, p.description, p.start_date, p.end_date, p.status, p.is_public,
		COALESCE(p.baseline_created_at, '') as baseline_created_at,
		COALESCE(p.baseline_created_by, '') as baseline_created_by,
			p.schedule_direction, COALESCE(p.owner, '') as owner
		FROM projects p WHERE p.deleted_at IS NULL ORDER BY p.created_at ASC, p.id ASC`

	rows, err := h.db.Query(query)
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
			&p.Status, &isPublic, &p.BaselineCreatedAt, &p.BaselineCreatedBy, &p.ScheduleDirection, &p.Owner); err != nil {
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
		// 财年起始月从 settings 表动态读取（管理员在系统配置页修改，即时生效）
		fiscalMonth := settings.GetInt(h.db, settings.KeyFiscalStartMonth, 4)
		start, end, err := util.FiscalYearRange(fyInt, fiscalMonth)
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

// ownerIsValidUser 校验所有者必须是已存在的活跃用户（display_name 或 email 精确匹配）
// 项目 owner 用于后续邮件通知，必须是可解析出邮箱的系统用户
func (h *ProjectHandler) ownerIsValidUser(owner string) bool {
	if strings.TrimSpace(owner) == "" {
		return false
	}
	var n int
	err := h.db.QueryRow(`SELECT COUNT(*) FROM users WHERE is_active = 1 AND (display_name = ? OR email = ?)`, owner, owner).Scan(&n)
	return err == nil && n > 0
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
	// 项目所有者必填且必须是系统已有用户（防呆 + 邮件通知需要邮箱）
	if !h.ownerIsValidUser(p.Owner) {
		writeError(w, http.StatusBadRequest, "INVALID_OWNER", "项目所有者必须从现有用户中选择（请先在用户管理创建用户）")
		return
	}

	// 坏编码防护：连续替换字符（GBK 终端误传中文的指纹）直接拒绝
	if hasBadEncoding(p.Name) || hasBadEncoding(p.Description) {
		writeError(w, http.StatusBadRequest, "INVALID_ENCODING", "项目名称/描述含非法字符（编码错误），请使用 UTF-8 输入")
		return
	}

	if p.ScheduleDirection == "" {
		p.ScheduleDirection = "forward" // 默认正推
	}
	result, err := h.db.Exec(
		`INSERT INTO projects (name, description, owner, start_date, end_date, status, schedule_direction)
		 VALUES (?, ?, ?, ?, ?, 'active', ?)`,
		p.Name, p.Description, p.Owner, p.StartDate, p.EndDate, p.ScheduleDirection,
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

// CopyProject 深拷贝项目：项目 + 全部任务（含层级）+ 依赖关系
// 新 id、名称+"(副本)"、日期/负责人/进度保留、状态重置为 active，副本排程重算
func (h *ProjectHandler) CopyProject(w http.ResponseWriter, r *http.Request) {
	srcID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	var src models.Project
	err := h.db.QueryRow(
		`SELECT name, description, start_date, end_date, schedule_direction, owner
		 FROM projects WHERE id=? AND deleted_at IS NULL`, srcID).
		Scan(&src.Name, &src.Description, &src.StartDate, &src.EndDate, &src.ScheduleDirection, &src.Owner)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "项目不存在")
		return
	}

	// 新项目（状态重置为 active）
	res, err := h.db.Exec(
		`INSERT INTO projects (name, description, owner, start_date, end_date, status, schedule_direction)
		 VALUES (?, ?, ?, ?, ?, 'active', ?)`,
		src.Name+"(副本)", src.Description, src.Owner, src.StartDate, src.EndDate, src.ScheduleDirection)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "创建副本项目失败")
		return
	}
	newID, _ := res.LastInsertId()

	// 任务复制（SQLITE_BUSY 防护：所有 SELECT 先读入内存并关闭 rows，再执行写入，
	// 避免读连接持锁时 INSERT/UPDATE 被拒——与启动重排 goroutine 的已知坑同理）
	type srcTask struct {
		t      models.Task
		manual int
	}
	var srcTasks []srcTask
	rows, err := h.db.Query(
		`SELECT id, name, description, task_type, status, priority, assignee,
		 start_date, end_date, duration_days, progress_pct, manual_scheduled,
		 constraint_type, constraint_date, sort_order
		 FROM tasks WHERE project_id=? AND deleted_at IS NULL`, srcID)
	if err == nil {
		for rows.Next() {
			var st srcTask
			if err := rows.Scan(&st.t.ID, &st.t.Name, &st.t.Description, &st.t.TaskType, &st.t.Status, &st.t.Priority,
				&st.t.Assignee, &st.t.StartDate, &st.t.EndDate, &st.t.DurationDays, &st.t.ProgressPct, &st.manual,
				&st.t.ConstraintType, &st.t.ConstraintDate, &st.t.SortOrder); err == nil {
				srcTasks = append(srcTasks, st)
			}
		}
		rows.Close()
	}

	oldToNew := map[int64]int64{}
	var oldIDs []int64
	for _, st := range srcTasks {
		r2, err := h.db.Exec(
			`INSERT INTO tasks (project_id, parent_id, name, description, task_type, status, priority,
			 assignee, start_date, end_date, duration_days, progress_pct, manual_scheduled,
			 constraint_type, constraint_date, sort_order)
			 VALUES (?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newID, st.t.Name, st.t.Description, st.t.TaskType, st.t.Status, st.t.Priority, st.t.Assignee,
			st.t.StartDate, st.t.EndDate, st.t.DurationDays, st.t.ProgressPct, st.manual,
			st.t.ConstraintType, st.t.ConstraintDate, st.t.SortOrder)
		if err != nil {
			log.Printf("[Copy] 任务[%d %s]插入失败: %v", st.t.ID, st.t.Name, err)
			continue
		}
		nid, _ := r2.LastInsertId()
		oldToNew[st.t.ID] = nid
		oldIDs = append(oldIDs, st.t.ID)
	}

	// 回填 parent_id（新旧映射）
	type srcParent struct {
		id     int64
		parent int64
		valid  bool
	}
	var parents []srcParent
	prows, err := h.db.Query(`SELECT id, parent_id FROM tasks WHERE project_id=? AND deleted_at IS NULL`, srcID)
	if err == nil {
		for prows.Next() {
			var p srcParent
			var parent sql.NullInt64
			if prows.Scan(&p.id, &parent) == nil {
				p.parent, p.valid = parent.Int64, parent.Valid
				parents = append(parents, p)
			}
		}
		prows.Close()
	}
	for _, p := range parents {
		if p.valid {
			if np, ok := oldToNew[p.parent]; ok {
				if nid, ok2 := oldToNew[p.id]; ok2 {
					h.db.Exec(`UPDATE tasks SET parent_id=? WHERE id=?`, np, nid)
				}
			}
		}
	}

	// 复制依赖（映射新旧任务 id；dependencies 无 project_id 列，按源任务 id 集合过滤）
	if len(oldIDs) > 0 {
		type srcDep struct {
			pred, succ, lag int64
			dtype           string
		}
		var deps []srcDep
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(oldIDs)), ",")
		args := make([]interface{}, len(oldIDs))
		for i, id := range oldIDs {
			args[i] = id
		}
		drows, err := h.db.Query(
			`SELECT predecessor_id, successor_id, dep_type, lag_days FROM dependencies
			 WHERE predecessor_id IN (`+placeholders+`)`, args...)
		if err == nil {
			for drows.Next() {
				var d srcDep
				if drows.Scan(&d.pred, &d.succ, &d.dtype, &d.lag) == nil {
					deps = append(deps, d)
				}
			}
			drows.Close()
		}
		for _, d := range deps {
			if np, ok1 := oldToNew[d.pred]; ok1 {
				if ns, ok2 := oldToNew[d.succ]; ok2 {
					h.db.Exec(`INSERT INTO dependencies (predecessor_id, successor_id, dep_type, lag_days)
						VALUES (?, ?, ?, ?)`, np, ns, d.dtype, d.lag)
				}
			}
		}
	}

	// 副本排程重算
	if _, err := scheduler.RecalculateAll(h.db, newID); err != nil {
		log.Printf("[Scheduler] 复制项目 %d 重算失败: %v", newID, err)
	}
	// 创建者加入副本成员
	if userID, ok := auth.GetUserID(r.Context()); ok {
		h.db.Exec("INSERT OR IGNORE INTO project_members (project_id, user_id, role) VALUES (?, ?, 'owner')", newID, userID)
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"id": newID, "name": src.Name + "(副本)"})
}

// UpdateProject 更新项目
func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	var p models.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}

	// 坏编码防护：连续替换字符（GBK 终端误传中文的指纹）直接拒绝
	if hasBadEncoding(p.Name) || hasBadEncoding(p.Description) {
		writeError(w, http.StatusBadRequest, "INVALID_ENCODING", "项目名称/描述含非法字符（编码错误），请使用 UTF-8 输入")
		return
	}

	// 读取旧值：判断日期/方向是否变化（变化才重排），未携带的字段保留旧值
	var curDirection, oldStart, oldEnd, oldOwner string
	h.db.QueryRow(`SELECT COALESCE(schedule_direction, 'forward'), COALESCE(start_date, ''), COALESCE(end_date, ''), COALESCE(owner, '') FROM projects WHERE id=? AND deleted_at IS NULL`, id).Scan(&curDirection, &oldStart, &oldEnd, &oldOwner)

	// 排程方向锁定：项目内任一任务有进度后不可修改方向
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
	if p.StartDate == "" {
		p.StartDate = oldStart // 未携带时保留旧值（避免空串覆盖）
	}
	if p.EndDate == "" {
		p.EndDate = oldEnd
	}
	if p.Owner == "" {
		p.Owner = oldOwner // 未携带时保留旧值
	}
	// 修改所有者时必须指向已有用户（邮件通知需要邮箱）
	if p.Owner != oldOwner && !h.ownerIsValidUser(p.Owner) {
		writeError(w, http.StatusBadRequest, "INVALID_OWNER", "项目所有者必须从现有用户中选择")
		return
	}
	_, err := h.db.Exec(
		`UPDATE projects SET name=?, description=?, owner=?, start_date=?, end_date=?, status=?, is_public=?,
		       schedule_direction=?, updated_at=datetime('now')
		 WHERE id=? AND deleted_at IS NULL`,
		p.Name, p.Description, p.Owner, p.StartDate, p.EndDate, p.Status, boolToInt(p.IsPublic),
		p.ScheduleDirection, id,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "更新项目失败")
		return
	}

	// 项目所有者变更 → 未开始（待开始/已延期）任务自动改派给新 owner；已完成/进行中保持不变
	if p.Owner != oldOwner {
		res, err := h.db.Exec(
			`UPDATE tasks SET assignee=?, version=version+1
			 WHERE project_id=? AND deleted_at IS NULL AND status IN ('open', 'delayed')`,
			p.Owner, id,
		)
		if err != nil {
			log.Printf("[Project] 项目 %d owner 变更后改派任务失败: %v", id, err)
		} else if n, _ := res.RowsAffected(); n > 0 {
			log.Printf("[Project] 项目 %d owner 变更,改派 %d 个未开始任务", id, n)
		}
	}

	// 项目开始/结束日期或排程方向变化 → 全项目重排（正排：链头对齐新开始日期；倒排：链尾对齐新完成日期）
	if p.StartDate != oldStart || p.EndDate != oldEnd || p.ScheduleDirection != curDirection {
		changes, err := scheduler.RecalculateAll(h.db, id)
		if err != nil {
			log.Printf("[Project] 项目 %d 日期/方向变更后重排失败: %v", id, err)
		} else {
			log.Printf("[Project] 项目 %d 日期/方向变更,重排 %d 个任务", id, len(changes))
		}
		// 广播给项目房间内其他在线用户（其页面自动刷新）
		if h.hub != nil {
			userID, _ := auth.GetUserID(r.Context())
			userName, _ := auth.GetUserEmail(r.Context())
			h.hub.BroadcastTaskUpdate(id, userID, userName, 0, nil)
		}
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
		"SELECT id, name, description, owner, start_date, end_date, status, is_public, schedule_direction FROM projects WHERE id = ? AND deleted_at IS NULL",
		id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Owner, &p.StartDate, &p.EndDate, &p.Status, &isPublic, &p.ScheduleDirection)
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
