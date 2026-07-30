package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"followitup/internal/auth"

	"github.com/go-chi/chi/v5"
)

type CalendarHandler struct {
	db  *sql.DB
	mid *auth.Middleware
}

func NewCalendarHandler(db *sql.DB, mid *auth.Middleware) *CalendarHandler {
	return &CalendarHandler{db: db, mid: mid}
}

func (h *CalendarHandler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.mid.RequireAuth)
		r.Get("/api/calendar", h.List)
		r.Post("/api/calendar", h.Create)
		r.Delete("/api/calendar/{id}", h.Delete)
	})
}

func (h *CalendarHandler) List(w http.ResponseWriter, r *http.Request) {
	year := r.URL.Query().Get("year")
	var rows *sql.Rows
	var err error
	if year != "" {
		rows, err = h.db.Query(
			"SELECT id, date, type, label FROM calendar WHERE strftime('%Y', date) = ? ORDER BY date", year)
	} else {
		rows, err = h.db.Query("SELECT id, date, type, label FROM calendar ORDER BY date")
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "查询日历失败")
		return
	}
	defer rows.Close()

	type Entry struct {
		ID    int64  `json:"id"`
		Date  string `json:"date"`
		Type  string `json:"type"`
		Label string `json:"label"`
	}
	var entries []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Date, &e.Type, &e.Label); err == nil {
			entries = append(entries, e)
		}
	}
	if entries == nil {
		entries = []Entry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func (h *CalendarHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date  string `json:"date"`
		Type  string `json:"type"`
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Date == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "缺少 date 字段")
		return
	}
	if req.Type == "" {
		req.Type = "holiday"
	}
	_, err := h.db.Exec("INSERT OR REPLACE INTO calendar (date, type, label) VALUES (?, ?, ?)",
		req.Date, req.Type, req.Label)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "添加日历条目失败")
		return
	}
	writeJSON(w, http.StatusCreated, req)
}

func (h *CalendarHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "无效的 ID")
		return
	}
	h.db.Exec("DELETE FROM calendar WHERE id = ?", id)
	writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
}
