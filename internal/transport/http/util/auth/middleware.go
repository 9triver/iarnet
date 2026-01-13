package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/9triver/iarnet/internal/transport/http/util/response"
	"github.com/sirupsen/logrus"
)

// ContextKey 用于在 context 中存储用户信息的 key
type ContextKey string

const (
	// UserContextKey context 中用户名的 key
	UserContextKey ContextKey = "username"
	// RoleContextKey context 中用户角色的 key
	RoleContextKey ContextKey = "role"
)

// AuthMiddleware 认证中间件
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 允许登录接口和健康检查接口不验证
		path := r.URL.Path
		if path == "/auth/login" || path == "/status" || strings.HasPrefix(path, "/auth/login") {
			next.ServeHTTP(w, r)
			return
		}

		// 允许 OPTIONS 请求（CORS 预检）
		if r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}

		// 从请求头获取 token
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			response.Unauthorized("authorization header required").WriteJSON(w)
			return
		}

		// 解析 Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized("invalid authorization header format").WriteJSON(w)
			return
		}

		tokenString := parts[1]

		// 验证 token
		claims, err := ValidateToken(tokenString)
		if err != nil {
			logrus.Debugf("Token validation failed: %v", err)
			response.Unauthorized("invalid or expired token").WriteJSON(w)
			return
		}

		// 将用户信息注入到 context
		ctx := context.WithValue(r.Context(), UserContextKey, claims.Username)
		ctx = context.WithValue(ctx, RoleContextKey, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUsernameFromContext 从 context 中获取用户名
func GetUsernameFromContext(ctx context.Context) string {
	if username, ok := ctx.Value(UserContextKey).(string); ok {
		return username
	}
	return ""
}

// GetRoleFromContext 从 context 中获取用户角色
func GetRoleFromContext(ctx context.Context) string {
	if role, ok := ctx.Value(RoleContextKey).(string); ok {
		return role
	}
	return ""
}
