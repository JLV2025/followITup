package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"followitup/internal/auth"
	"followitup/internal/settings"

	"github.com/go-chi/chi/v5"
)

type SettingsHandler struct {
	db  *sql.DB
	mid *auth.Middleware
}

func NewSettingsHandler(db *sql.DB, mid *auth.Middleware) *SettingsHandler {
	return &SettingsHandler{db: db, mid: mid}
}

func (h *SettingsHandler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.mid.RequireAuth)
		r.Get("/api/settings", h.GetPublic)
	})
	r.Group(func(r chi.Router) {
		r.Use(h.mid.RequireAuth)
		r.Use(h.adminOnly)
		r.Get("/api/settings/admin", h.GetAll)
		r.Put("/api/settings", h.Put)
		// r.Post("/api/settings/test-email", h.TestEmail) // T2 邮件服务接入后启用
	})
}

// adminOnly 管理员守卫（复用 AdminOnly 的判定逻辑）
func (h *SettingsHandler) adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.GetIsAdmin(r.Context()) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "仅管理员可操作")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetPublic 公开子集（创建用户表单、财年展示需要）
func (h *SettingsHandler) GetPublic(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"fiscal_start_month":  settings.GetInt(h.db, settings.KeyFiscalStartMonth, 4),
		"password_min_length": settings.GetInt(h.db, settings.KeyPasswordMinLen, 8),
	})
}

// GetAll 全量配置（仅管理员）
func (h *SettingsHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	m, err := settings.GetAll(h.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "读取系统配置失败")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// Put 批量更新（仅管理员，白名单 key）
func (h *SettingsHandler) Put(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}
	allowed := make(map[string]bool)
	for _, k := range settings.AllKeys {
		allowed[k] = true
	}
	for k, v := range req {
		if !allowed[k] {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "未知配置项: "+k)
			return
		}
		value := ""
		switch val := v.(type) {
		case string:
			value = val
		case float64:
			value = strconv.FormatFloat(val, 'f', -1, 64)
		default:
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "配置值格式错误: "+k)
			return
		}
		if k == settings.KeyPasswordMinLen {
			n, err := strconv.Atoi(value)
			if err != nil || n < 6 || n > 32 {
				writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "密码最小长度需在 6-32 之间")
				return
			}
		}
		if err := settings.Set(h.db, k, value); err != nil {
			writeError(w, http.StatusInternalServerError, "DB_ERROR", "保存配置失败")
			return
		}
	}
	m, _ := settings.GetAll(h.db)
	writeJSON(w, http.StatusOK, m)
}
