package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"followitup/internal/auth"
	"followitup/internal/mail"
	"followitup/internal/models"
	"followitup/internal/settings"

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
	// 管理员接口
	r.Get("/api/admin/users", withAuth(h.mid, h.AdminOnly(h.ListUsers)))
	// 创建用户：全部登录用户可创建（仅管理员可设置管理员角色）
	r.Post("/api/admin/users", withAuth(h.mid, h.CreateUser))
	// 删除用户 / 提升降级管理员（仅管理员）
	r.Delete("/api/admin/users/{id}", withAuth(h.mid, h.AdminOnly(h.DeleteUser)))
	r.Put("/api/admin/users/{id}/role", withAuth(h.mid, h.AdminOnly(h.SetUserRole)))
	// 用户精简列表（assignee 下拉数据源）
	r.Get("/api/users", withAuth(h.mid, h.PublicUsers))
}

// DeleteUser 删除用户（软删，仅管理员；管理员需先降级）
func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	currentID, _ := auth.GetUserID(r.Context())
	if id == currentID {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "不能删除自己")
		return
	}
	var isAdmin int
	err := h.svc.DB().QueryRow(
		`SELECT is_admin FROM users WHERE id = ? AND is_active = 1`, id).Scan(&isAdmin)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "用户不存在")
		return
	}
	if isAdmin == 1 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请先取消其管理员身份")
		return
	}
	if err := h.svc.DeleteUser(id); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "删除用户失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "用户已删除"})
}

// SetUserRole 提升/降级管理员（仅管理员）
func (h *AuthHandler) SetUserRole(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req struct {
		IsAdmin bool `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}
	if err := h.svc.SetUserRole(id, req.IsAdmin); err != nil {
		if strings.Contains(err.Error(), "至少保留") {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
		writeError(w, http.StatusNotFound, "NOT_FOUND", "用户不存在")
		return
	}
	user, err := h.svc.GetUserByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "用户不存在")
		return
	}
	writeJSON(w, http.StatusOK, user)
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

// ListUsers 列出所有用户（仅管理员）
func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "查询用户失败")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

// CreateUser 创建本地用户（全部登录用户可创建；仅管理员可设置管理员角色）
func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		IsAdmin     bool   `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "邮箱不能为空")
		return
	}
	if !validEmailFormat(req.Email) {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "邮箱格式不正确")
		return
	}
	// 显示名：未提供时从邮箱推导
	if req.DisplayName == "" {
		req.DisplayName = auth.DeriveDisplayName(req.Email)
	}
	// 仅管理员可创建管理员；普通用户传 true 强制忽略
	if req.IsAdmin && !auth.GetIsAdmin(r.Context()) {
		req.IsAdmin = false
	}
	minLen := settings.GetInt(h.svc.DB(), settings.KeyPasswordMinLen, 8)
	password := auth.GenerateRandomPassword(minLen)
	if err := h.svc.CreateUser(req.Email, password, req.DisplayName, "local", req.IsAdmin, true); err != nil {
		writeError(w, http.StatusConflict, "DUPLICATE", err.Error())
		return
	}
	// 发送通知邮件；失败不阻塞创建，明文密码退回给创建者手动传达
	mailErr := mail.SendTemporaryPassword(h.svc.DB(), req.Email, req.DisplayName, password)
	if mailErr != nil {
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"message":          "用户创建成功（邮件发送失败，请手动告知初始密码）",
			"initial_password": password,
			"mail_sent":        false,
		})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":   "用户创建成功，初始密码已发送至邮箱",
		"mail_sent": true,
	})
}

// validEmailFormat 简单邮箱格式校验
func validEmailFormat(email string) bool {
	at := strings.Index(email, "@")
	if at <= 0 || at == len(email)-1 {
		return false
	}
	return strings.Index(email[at+1:], ".") > 0
}

// PublicUsers 返回活跃用户精简列表（assignee 下拉数据源）
func (h *AuthHandler) PublicUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.DB().Query(
		`SELECT id, display_name, email FROM users WHERE is_active = 1 ORDER BY display_name`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "查询用户失败")
		return
	}
	defer rows.Close()
	type Item struct {
		ID          int64  `json:"id"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
	}
	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.DisplayName, &it.Email); err == nil {
			items = append(items, it)
		}
	}
	if items == nil {
		items = []Item{}
	}
	writeJSON(w, http.StatusOK, items)
}

// AdminOnly 包装需要管理员权限的 handler
func (h *AuthHandler) AdminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !auth.GetIsAdmin(r.Context()) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "仅管理员可操作")
			return
		}
		next(w, r)
	}
}

// ChangePassword 修改密码（首登改密成功后返回新 token）
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

	token, err := h.svc.ChangePasswordAndRetoken(userID, req.OldPassword, req.NewPassword)
	if err != nil {
		writeError(w, http.StatusBadRequest, "PASSWORD_CHANGE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}
