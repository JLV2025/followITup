package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	// UserIDKey 用户 ID 上下文键
	UserIDKey contextKey = "user_id"
	// UserEmailKey 用户邮箱上下文键
	UserEmailKey contextKey = "user_email"
	// IsAdminKey 管理员标志上下文键
	IsAdminKey contextKey = "is_admin"
)

// Middleware JWT 鉴权中间件
type Middleware struct {
	jwtSecret []byte
}

// NewMiddleware 创建鉴权中间件
func NewMiddleware(jwtSecret string) *Middleware {
	return &Middleware{jwtSecret: []byte(jwtSecret)}
}

// RequireAuth 要求登录，返回 401 如果未认证
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := m.parseToken(r)
		if err != nil {
			http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"请先登录"}}`, http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		if userID, ok := claims["user_id"].(float64); ok {
			ctx = context.WithValue(ctx, UserIDKey, int64(userID))
		}
		if email, ok := claims["email"].(string); ok {
			ctx = context.WithValue(ctx, UserEmailKey, email)
		}
		if isAdmin, ok := claims["is_admin"].(bool); ok {
			ctx = context.WithValue(ctx, IsAdminKey, isAdmin)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth 可选认证，解析 token 但不强制要求
func (m *Middleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := m.parseToken(r)
		if err == nil {
			ctx := r.Context()
			if userID, ok := claims["user_id"].(float64); ok {
				ctx = context.WithValue(ctx, UserIDKey, int64(userID))
			}
			if email, ok := claims["email"].(string); ok {
				ctx = context.WithValue(ctx, UserEmailKey, email)
			}
			if isAdmin, ok := claims["is_admin"].(bool); ok {
				ctx = context.WithValue(ctx, IsAdminKey, isAdmin)
			}
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) parseToken(r *http.Request) (jwt.MapClaims, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, jwt.ErrSignatureInvalid
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return nil, jwt.ErrSignatureInvalid
	}

	token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return m.jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, jwt.ErrSignatureInvalid
	}

	return claims, nil
}

// GetUserID 从 context 获取用户 ID
func GetUserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(UserIDKey).(int64)
	return id, ok
}

// GetUserEmail 从 context 获取用户邮箱
func GetUserEmail(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(UserEmailKey).(string)
	return email, ok
}

// GetIsAdmin 从 context 获取管理员标志
func GetIsAdmin(ctx context.Context) bool {
	isAdmin, ok := ctx.Value(IsAdminKey).(bool)
	return ok && isAdmin
}
