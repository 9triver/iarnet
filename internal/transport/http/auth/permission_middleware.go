package auth

import (
	"net/http"
	"strings"

	"github.com/9triver/iarnet/internal/config"
	httpauth "github.com/9triver/iarnet/internal/transport/http/util/auth"
	"github.com/9triver/iarnet/internal/transport/http/util/response"
)

// PermissionMiddleware 权限控制中间件
// 根据路径和用户角色进行权限检查
func (api *API) PermissionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// 允许登录和验证码接口
		if path == "/auth/login" || strings.HasPrefix(path, "/api/captcha") {
			next.ServeHTTP(w, r)
			return
		}

		// 允许 OPTIONS 请求（CORS 预检）
		if r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}

		// 获取用户角色
		username := httpauth.GetUsernameFromContext(r.Context())
		if username == "" {
			response.Unauthorized("authentication required").WriteJSON(w)
			return
		}

		userRole := api.userManager.GetUserRole(username)
		if userRole == "" {
			// 如果角色为空，根据用户名判断默认角色
			if username == "admin" {
				userRole = config.RoleSuperAdmin
			} else {
				userRole = config.RoleNormalUser
			}
		}

		// 根据路径检查权限
		requiredRole := api.getRequiredRoleForPath(path, r.Method)
		if requiredRole == "" {
			// 路径不需要特殊权限，允许访问
			next.ServeHTTP(w, r)
			return
		}

		// 检查权限
		if !api.hasPermission(userRole, requiredRole) {
			response.Forbidden("insufficient_permissions").WriteJSON(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getRequiredRoleForPath 根据路径和方法获取所需角色
func (api *API) getRequiredRoleForPath(path string, method string) config.UserRole {
	// 用户管理相关接口：只有超级管理员可以访问
	if strings.HasPrefix(path, "/auth/users") {
		return config.RoleSuperAdmin
	}

	// 日志审计接口：平台管理员和超级管理员可以访问
	if strings.HasPrefix(path, "/audit") {
		return config.RolePlatformAdmin
	}

	// 资源管理接口的修改操作：普通用户不能修改
	if strings.HasPrefix(path, "/resource/provider") {
		if method == "POST" || method == "PUT" || method == "DELETE" {
			// 创建、更新、删除操作需要平台管理员权限
			return config.RolePlatformAdmin
		}
		// 查询操作所有用户都可以访问
		return ""
	}

	// 其他接口默认允许所有已登录用户访问
	return ""
}

// hasPermission 检查用户是否有权限
func (api *API) hasPermission(userRole, requiredRole config.UserRole) bool {
	switch requiredRole {
	case config.RoleSuperAdmin:
		// 只有超级管理员可以访问
		return userRole == config.RoleSuperAdmin
	case config.RolePlatformAdmin:
		// 平台管理员和超级管理员可以访问
		return userRole == config.RolePlatformAdmin || userRole == config.RoleSuperAdmin
	case config.RoleNormalUser:
		// 所有用户都可以访问
		return true
	default:
		return false
	}
}
