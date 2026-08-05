package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
		Type      string `json:"type"`
		Label     string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.StartDate == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "缺少 start_date 字段")
		return
	}
	// 兼容单日调用：end_date 缺省 = start_date
	end := req.EndDate
	if end == "" {
		end = req.StartDate
	}
	if req.StartDate > end {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "结束日期不能早于开始日期")
		return
	}
	// 单次范围不能超过一年（366 天）
	days := countDays(req.StartDate, end)
	if days > 366 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "单次范围不能超过一年")
		return
	}
	if req.Type == "" {
		req.Type = "holiday"
	}
	if req.Type != "holiday" && req.Type != "workday" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "type 仅支持 holiday 或 workday")
		return
	}
	// 按日展开写入
	for d := req.StartDate; d <= end; d = nextDay(d) {
		if _, err := h.db.Exec("INSERT OR REPLACE INTO calendar (date, type, label) VALUES (?, ?, ?)",
			d, req.Type, req.Label); err != nil {
			writeError(w, http.StatusInternalServerError, "DB_ERROR", "添加日历条目失败")
			return
		}
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": fmt.Sprintf("已添加 %d 天", days),
		"count":   days,
	})
}

// countDays 计算 start~end 含首尾的天数（YYYY-MM-DD 字符串）
func countDays(start, end string) int {
	var sy, sm, sd, ey, em, ed int
	fmt.Sscanf(start, "%d-%d-%d", &sy, &sm, &sd)
	fmt.Sscanf(end, "%d-%d-%d", &ey, &em, &ed)
	days := 0
	for !(sy == ey && sm == em && sd == ed) {
		days++
		sd++
		dim := daysInMonth(sy, sm)
		if sd > dim {
			sd = 1
			sm++
			if sm > 12 {
				sm = 1
				sy++
			}
		}
	}
	return days + 1
}

// nextDay 返回 date 的次日（YYYY-MM-DD）
func nextDay(date string) string {
	var y, m, d int
	fmt.Sscanf(date, "%d-%d-%d", &y, &m, &d)
	d++
	dim := daysInMonth(y, m)
	if d > dim {
		d = 1
		m++
		if m > 12 {
			m = 1
			y++
		}
	}
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}

// daysInMonth 返回某月天数（用于范围展开）
func daysInMonth(y, m int) int {
	switch m {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	}
	// 2 月：闰年判断
	if (y%4 == 0 && y%100 != 0) || y%400 == 0 {
		return 29
	}
	return 28
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
