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

// hasBadEncoding 检测坏编码指纹：连续 ≥2 个替换字符（U+FFFD）。
// Windows 终端（GBK）经 curl 直传中文时，GBK 字节被按 UTF-8 解析会产生连串 U+FFFD，
// 真实中文文本绝不会出现连续替换字符——检测到即拒绝入库，避免乱码污染数据。
func hasBadEncoding(s string) bool {
	count := 0
	for _, r := range s {
		if r == '�' {
			count++
			if count >= 2 {
				return true
			}
		} else {
			count = 0
		}
	}
	return false
}

// RegisterRoutes 注册所有 API 路由
func RegisterRoutes(r chi.Router, authHandler *AuthHandler) {
	authHandler.RegisterRoutes(r)
}
