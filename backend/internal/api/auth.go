package api

import (
	"encoding/json"
	"net/http"

	"followitup/internal/auth"
	"followitup/internal/models"

	"github.com/go-chi/chi/v5"
)

// AuthHandler 认证相关端点
type AuthHandler struct {
	svc *auth.Service
	mid *auth.Middleware
}

// NewAuthHandler 创建认证端点实例
func NewAuthHandler(svc *auth.Service, mid *auth.Middleware) *AuthHandler {
	return &AuthHandler{svc: svc, mid: mid}
}

// RegisterRoutes 注册认证路由
func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	r.Post("/api/auth/login", h.Login)
	r.Post("/api/auth/change-password", withAuth(h.mid, h.ChangePassword))
	r.Get("/api/auth/me", withAuth(h.mid, h.Me))
}

// withAuth 包装需要认证的 handler
func withAuth(mid *auth.Middleware, next http.HandlerFunc) http.HandlerFunc {
	return mid.RequireAuth(http.HandlerFunc(next)).ServeHTTP
}

// Login 处理登录请求
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "邮箱和密码不能为空")
		return
	}

	resp, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Me 获取当前用户信息
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	u, err := h.svc.GetUserByID(userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
		return
	}

	writeJSON(w, http.StatusOK, u)
}

// ChangePassword 修改密码
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	var req models.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}

	if err := h.svc.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, "PASSWORD_CHANGE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "密码修改成功"})
}
