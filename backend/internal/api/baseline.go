package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"followitup/internal/auth"
	"followitup/internal/ws"

	"github.com/go-chi/chi/v5"
)

// BaselineHandler 基线 API：创建/清除/查询项目当前基线
type BaselineHandler struct {
	db  *sql.DB
	mid *auth.Middleware
	hub *ws.Hub
}

// NewBaselineHandler 创建基线端点实例
func NewBaselineHandler(d *sql.DB, mid *auth.Middleware, hub *ws.Hub) *BaselineHandler {
	return &BaselineHandler{db: d, mid: mid, hub: hub}
}

// RegisterRoutes 注册基线相关路由
func (h *BaselineHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/projects/{id}/baseline", h.GetBaseline)
	r.Group(func(r chi.Router) {
		r.Use(h.mid.RequireAuth)
		r.Post("/api/projects/{id}/baseline", h.CreateBaseline)
		r.Delete("/api/projects/{id}/baseline", h.DeleteBaseline)
	})
}

// createBaselineTx 快照当前任务排程字段到基线列（事务内）
func createBaselineTx(d *sql.DB, projectID, userID int64, userName string) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE tasks SET
		baseline_start_date = start_date,
		baseline_end_date = end_date,
		baseline_duration_days = duration_days,
		baseline_progress_pct = progress_pct
		WHERE project_id = ? AND deleted_at IS NULL`, projectID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE projects SET
		baseline_created_at = ?, baseline_created_by = ?
		WHERE id = ?`, time.Now().Format("2006-01-02 15:04:05"), userName, projectID); err != nil {
		return err
	}
	return tx.Commit()
}

// clearBaselineTx 清除基线（事务内）
func clearBaselineTx(d *sql.DB, projectID int64) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE tasks SET
		baseline_start_date = NULL, baseline_end_date = NULL,
		baseline_duration_days = NULL, baseline_progress_pct = NULL
		WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE projects SET baseline_created_at = NULL, baseline_created_by = NULL WHERE id = ?`, projectID); err != nil {
		return err
	}
	return tx.Commit()
}

// CreateBaseline 创建项目基线快照
func (h *BaselineHandler) CreateBaseline(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "无效的项目ID")
		return
	}
	userID, _ := auth.GetUserID(r.Context())
	userName, _ := auth.GetUserEmail(r.Context())

	if err := createBaselineTx(h.db, projectID, userID, userName); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "创建基线失败")
		return
	}

	h.hub.BroadcastTaskUpdate(projectID, userID, userName, 0, nil)
	writeJSON(w, http.StatusOK, map[string]string{"message": "基线已创建"})
}

// DeleteBaseline 清除项目基线
func (h *BaselineHandler) DeleteBaseline(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "无效的项目ID")
		return
	}
	userID, _ := auth.GetUserID(r.Context())
	userName, _ := auth.GetUserEmail(r.Context())

	if err := clearBaselineTx(h.db, projectID); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "清除基线失败")
		return
	}

	h.hub.BroadcastTaskUpdate(projectID, userID, userName, 0, nil)
	writeJSON(w, http.StatusOK, map[string]string{"message": "基线已清除"})
}

// GetBaseline 查询项目基线数据
func (h *BaselineHandler) GetBaseline(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "无效的项目ID")
		return
	}

	var createdAt, createdBy sql.NullString
	h.db.QueryRow(`SELECT baseline_created_at, baseline_created_by FROM projects WHERE id=?`, projectID).Scan(&createdAt, &createdBy)

	rows, err := h.db.Query(`SELECT id, name, baseline_start_date, baseline_end_date,
		baseline_duration_days, baseline_progress_pct
		FROM tasks WHERE project_id=? AND deleted_at IS NULL AND baseline_start_date IS NOT NULL
		ORDER BY id`, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "查询基线失败")
		return
	}
	defer rows.Close()

	type BaselineTask struct {
		ID                   int64    `json:"id"`
		Name                 string   `json:"name"`
		BaselineStartDate    *string  `json:"baseline_start_date"`
		BaselineEndDate      *string  `json:"baseline_end_date"`
		BaselineDurationDays *int     `json:"baseline_duration_days"`
		BaselineProgressPct  *float64 `json:"baseline_progress_pct"`
	}

	var tasks []BaselineTask
	for rows.Next() {
		var t BaselineTask
		if err := rows.Scan(&t.ID, &t.Name, &t.BaselineStartDate, &t.BaselineEndDate,
			&t.BaselineDurationDays, &t.BaselineProgressPct); err == nil {
			tasks = append(tasks, t)
		}
	}
	if tasks == nil {
		tasks = []BaselineTask{}
	}

	result := map[string]interface{}{
		"tasks": tasks,
	}
	if createdAt.Valid {
		result["baseline_created_at"] = createdAt.String
	}
	if createdBy.Valid {
		result["baseline_created_by"] = createdBy.String
	}

	writeJSON(w, http.StatusOK, result)
}
