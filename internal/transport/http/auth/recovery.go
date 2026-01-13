package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/transport/http/util/response"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// RecoveryRequest 恢复请求
type RecoveryRequest struct {
	Username string `json:"username"`
	Password string `json:"password"` // 配置文件中的超级管理员密码
}

// RegisterRecoveryRoute 注册恢复路由（紧急恢复超级管理员账户）
func (api *API) RegisterRecoveryRoute(router *mux.Router) {
	router.HandleFunc("/auth/recovery/unlock-super-admin", api.handleRecoveryUnlockSuperAdmin).Methods("POST")
}

// handleRecoveryUnlockSuperAdmin 紧急恢复超级管理员账户
// 此端点不需要认证，但需要提供配置文件中的超级管理员密码
func (api *API) handleRecoveryUnlockSuperAdmin(w http.ResponseWriter, r *http.Request) {
	var req RecoveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		response.BadRequest("username is required").WriteJSON(w)
		return
	}

	// 检查配置中是否有超级管理员配置
	if api.config.SuperAdmin == nil || api.config.SuperAdmin.Name == "" {
		response.Forbidden("super admin recovery is not configured").WriteJSON(w)
		return
	}

	// 验证用户名是否匹配配置中的超级管理员
	if username != api.config.SuperAdmin.Name {
		response.Forbidden("invalid username for recovery").WriteJSON(w)
		return
	}

	// 验证密码是否匹配配置中的超级管理员密码
	if req.Password != api.config.SuperAdmin.Password {
		logrus.Warnf("Recovery attempt failed: incorrect config password for super admin: %s", username)
		response.Unauthorized("invalid recovery password").WriteJSON(w)
		return
	}

	// 检查用户是否存在
	userDAO, err := api.userRepo.GetByName(r.Context(), username)
	if err != nil {
		response.NotFound("user not found").WriteJSON(w)
		return
	}

	// 检查用户是否是超级管理员
	if userDAO.Role != config.RoleSuperAdmin && !(userDAO.Role == "" && username == "admin") {
		response.Forbidden("user is not a super admin").WriteJSON(w)
		return
	}

	// 解锁用户
	api.userManager.UnlockUser(username)

	logrus.Warnf("Super admin %s unlocked via recovery endpoint", username)

	response.Success(map[string]any{
		"message": "Super admin account unlocked successfully",
		"username": username,
	}).WriteJSON(w)
}
