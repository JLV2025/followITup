package api

import (
	"encoding/json"
	"net/http"

	"followitup/internal/models"

	"github.com/go-chi/chi/v5"
)

// writeJSON 输出 JSON 成功响应
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(models.APIResponse{Data: data})
}

// writeError 输出 JSON 错误响应
func writeError(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(models.APIResponse{
		Error: &models.APIError{Code: code, Message: message},
	})
}

// RegisterRoutes 注册所有 API 路由
func RegisterRoutes(r chi.Router, authHandler *AuthHandler) {
	authHandler.RegisterRoutes(r)
}
