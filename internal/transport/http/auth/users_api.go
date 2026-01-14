package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/9triver/iarnet/internal/config"
	userrepo "github.com/9triver/iarnet/internal/infra/repository/auth"
	httpauth "github.com/9triver/iarnet/internal/transport/http/util/auth"
	"github.com/9triver/iarnet/internal/transport/http/util/response"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// RegisterUserRoutes 注册用户管理路由
func (api *API) RegisterUserRoutes(router *mux.Router) {
	router.HandleFunc("/auth/users", api.handleGetUsers).Methods("GET")
	router.HandleFunc("/auth/users", api.handleCreateUser).Methods("POST")
	router.HandleFunc("/auth/users/{id}", api.handleGetUser).Methods("GET")
	router.HandleFunc("/auth/users/{id}", api.handleUpdateUser).Methods("PUT")
	router.HandleFunc("/auth/users/{id}", api.handleDeleteUser).Methods("DELETE")
	router.HandleFunc("/auth/users/{id}/unlock", api.handleUnlockUser).Methods("POST")
}

// GetUsersResponse 获取用户列表响应
type GetUsersResponse struct {
	Users []UserInfo `json:"users"`
	Total int        `json:"total"`
}

// UserInfo 用户信息（不包含密码）
type UserInfo struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	Locked      bool   `json:"locked"`
	LockedUntil string `json:"locked_until,omitempty"`
	FailedCount int    `json:"failed_count"`
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Password string `json:"password,omitempty"`
	Role     string `json:"role,omitempty"`
}

// handleGetUsers 获取用户列表
func (api *API) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	// 检查权限：只有超级管理员可以查看用户列表
	if !api.checkPermission(r, config.RoleSuperAdmin) {
		response.Forbidden("insufficient_permissions").WriteJSON(w)
		return
	}

	ctx := r.Context()
	userDAOs, err := api.userRepo.GetAll(ctx)
	if err != nil {
		logrus.Errorf("Failed to get users: %v", err)
		response.InternalError("failed to get users").WriteJSON(w)
		return
	}

	users := make([]UserInfo, 0, len(userDAOs))
	for _, userDAO := range userDAOs {
		locked := api.userManager.IsUserLocked(userDAO.Name)
		lockedUntil := api.userManager.GetLockedUntil(userDAO.Name)
		failedCount := api.userManager.GetFailedCount(userDAO.Name)

		role := string(userDAO.Role)
		if role == "" {
			// 如果角色为空，根据用户名判断默认角色
			if userDAO.Name == "admin" {
				role = string(config.RoleSuperAdmin)
			} else {
				role = string(config.RoleNormalUser)
			}
		}

		userInfo := UserInfo{
			Name:        userDAO.Name,
			Role:        role,
			Locked:      locked,
			FailedCount: failedCount,
		}

		if lockedUntil != nil {
			userInfo.LockedUntil = lockedUntil.Format("2006-01-02 15:04:05")
		}

		users = append(users, userInfo)
	}

	resp := GetUsersResponse{
		Users: users,
		Total: len(users),
	}
	response.Success(resp).WriteJSON(w)
}

// handleGetUser 获取单个用户信息
func (api *API) handleGetUser(w http.ResponseWriter, r *http.Request) {
	// 检查权限：只有超级管理员可以查看用户信息
	if !api.checkPermission(r, config.RoleSuperAdmin) {
		response.Forbidden("insufficient_permissions").WriteJSON(w)
		return
	}

	vars := mux.Vars(r)
	username := vars["id"]

	ctx := r.Context()
	userDAO, err := api.userRepo.GetByName(ctx, username)
	if err != nil {
		response.NotFound("user not found").WriteJSON(w)
		return
	}

	locked := api.userManager.IsUserLocked(userDAO.Name)
	lockedUntil := api.userManager.GetLockedUntil(userDAO.Name)
	failedCount := api.userManager.GetFailedCount(userDAO.Name)

	role := string(userDAO.Role)
	if role == "" {
		// 如果角色为空，根据用户名判断默认角色
		if userDAO.Name == "admin" {
			role = string(config.RoleSuperAdmin)
		} else {
			role = string(config.RoleNormalUser)
		}
	}

	userInfo := UserInfo{
		Name:        userDAO.Name,
		Role:        role,
		Locked:      locked,
		FailedCount: failedCount,
	}

	if lockedUntil != nil {
		userInfo.LockedUntil = lockedUntil.Format("2006-01-02 15:04:05")
	}

	response.Success(userInfo).WriteJSON(w)
}

// handleCreateUser 创建用户
func (api *API) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	// 检查权限：只有超级管理员可以创建用户
	if !api.checkPermission(r, config.RoleSuperAdmin) {
		response.Forbidden("insufficient_permissions").WriteJSON(w)
		return
	}

	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	// 验证输入
	username := strings.TrimSpace(req.Name)
	if username == "" {
		response.BadRequest("username is required").WriteJSON(w)
		return
	}

	if req.Password == "" {
		response.BadRequest("password is required").WriteJSON(w)
		return
	}

	// 验证角色
	role := config.UserRole(strings.TrimSpace(req.Role))
	if role == "" {
		role = config.RoleNormalUser
	} else if role != config.RoleNormalUser && role != config.RolePlatformAdmin && role != config.RoleSuperAdmin {
		response.BadRequest("invalid role. Must be one of: normal, platform, super").WriteJSON(w)
		return
	}

	// 检查用户是否已存在
	ctx := r.Context()
	_, err := api.userRepo.GetByName(ctx, username)
	if err == nil {
		response.BadRequest("user already exists").WriteJSON(w)
		return
	}

	// 创建新用户
	// 前端发送的是哈希值（64位十六进制），我们直接存储哈希值
	// 这样可以避免在传输过程中暴露明文密码
	passwordToStore := req.Password
	if len(req.Password) == 64 && isHexString(req.Password) {
		// 收到哈希值，直接存储
		passwordToStore = req.Password
	} else {
		// 收到明文（向后兼容），先哈希再存储
		passwordToStore = hashSHA256(req.Password)
	}

	userDAO := &userrepo.UserDAO{
		ID:       username, // 使用用户名作为 ID
		Name:     username,
		Password: passwordToStore,
		Role:     role,
	}

	if err := api.userRepo.Create(ctx, userDAO); err != nil {
		logrus.Errorf("Failed to create user: %v", err)
		response.InternalError("failed to create user").WriteJSON(w)
		return
	}

	logrus.Infof("User created: %s (role: %s)", username, role)

	userInfo := UserInfo{
		Name: userDAO.Name,
		Role: string(userDAO.Role),
	}

	if userInfo.Role == "" {
		// 如果角色为空，根据用户名判断默认角色
		if userDAO.Name == "admin" {
			userInfo.Role = string(config.RoleSuperAdmin)
		} else {
			userInfo.Role = string(config.RoleNormalUser)
		}
	}

	response.Created(userInfo).WriteJSON(w)
}

// handleUpdateUser 更新用户
func (api *API) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	// 检查权限：只有超级管理员可以更新用户
	if !api.checkPermission(r, config.RoleSuperAdmin) {
		response.Forbidden("insufficient_permissions").WriteJSON(w)
		return
	}

	vars := mux.Vars(r)
	username := vars["id"]

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	ctx := r.Context()
	userDAO, err := api.userRepo.GetByName(ctx, username)
	if err != nil {
		response.NotFound("user not found").WriteJSON(w)
		return
	}

	// 检查是否是配置的初始超级管理员
	isInitialSuperAdmin := api.config.SuperAdmin != nil && api.config.SuperAdmin.Name == username
	currentUsername := httpauth.GetUsernameFromContext(r.Context())

	// 如果尝试修改初始超级管理员的密码，且当前用户不是该用户自己，则拒绝
	if req.Password != "" && isInitialSuperAdmin && currentUsername != username {
		response.Forbidden("cannot_modify_initial_super_admin_password").WriteJSON(w)
		return
	}

	// 更新密码（如果提供）
	// 前端发送的是哈希值（64位十六进制），我们直接存储哈希值
	// 这样可以避免在传输过程中暴露明文密码
	if req.Password != "" {
		if len(req.Password) == 64 && isHexString(req.Password) {
			// 收到哈希值，直接存储
			userDAO.Password = req.Password
		} else {
			// 收到明文（向后兼容），先哈希再存储
			userDAO.Password = hashSHA256(req.Password)
		}
	}

	// 更新角色（如果提供）
	if req.Role != "" {
		role := config.UserRole(strings.TrimSpace(req.Role))
		if role != config.RoleNormalUser && role != config.RolePlatformAdmin && role != config.RoleSuperAdmin {
			response.BadRequest("invalid role. Must be one of: normal, platform, super").WriteJSON(w)
			return
		}
		userDAO.Role = role
	}

	if err := api.userRepo.Update(ctx, userDAO); err != nil {
		logrus.Errorf("Failed to update user: %v", err)
		response.InternalError("failed to update user").WriteJSON(w)
		return
	}

	logrus.Infof("User updated: %s", username)

	role := string(userDAO.Role)
	if role == "" {
		// 如果角色为空，根据用户名判断默认角色
		if userDAO.Name == "admin" {
			role = string(config.RoleSuperAdmin)
		} else {
			role = string(config.RoleNormalUser)
		}
	}

	userInfo := UserInfo{
		Name: userDAO.Name,
		Role: role,
	}

	response.Success(userInfo).WriteJSON(w)
}

// handleDeleteUser 删除用户
func (api *API) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	// 检查权限：只有超级管理员可以删除用户
	if !api.checkPermission(r, config.RoleSuperAdmin) {
		response.Forbidden("insufficient_permissions").WriteJSON(w)
		return
	}

	vars := mux.Vars(r)
	username := vars["id"]

	// 检查是否是配置的初始超级管理员，如果是则不允许删除
	if api.config.SuperAdmin != nil && api.config.SuperAdmin.Name == username {
		response.Forbidden("cannot_delete_initial_super_admin").WriteJSON(w)
		return
	}

	ctx := r.Context()
	userDAO, err := api.userRepo.GetByName(ctx, username)
	if err != nil {
		response.NotFound("user not found").WriteJSON(w)
		return
	}

	// 删除用户
	if err := api.userRepo.Delete(ctx, userDAO.ID); err != nil {
		logrus.Errorf("Failed to delete user: %v", err)
		response.InternalError("failed to delete user").WriteJSON(w)
		return
	}

	// 清除登录尝试记录
	api.userManager.UnlockUser(username)

	logrus.Infof("User deleted: %s", username)

	response.Success(nil).WriteJSON(w)
}

// handleUnlockUser 解锁用户
func (api *API) handleUnlockUser(w http.ResponseWriter, r *http.Request) {
	// 检查权限：只有超级管理员可以解锁用户
	if !api.checkPermission(r, config.RoleSuperAdmin) {
		response.Forbidden("insufficient_permissions").WriteJSON(w)
		return
	}

	vars := mux.Vars(r)
	username := vars["id"]

	ctx := r.Context()
	_, err := api.userRepo.GetByName(ctx, username)
	if err != nil {
		response.NotFound("user not found").WriteJSON(w)
		return
	}

	// 解锁用户
	api.userManager.UnlockUser(username)

	logrus.Infof("User unlocked: %s", username)

	response.Success(nil).WriteJSON(w)
}

// checkPermission 检查用户权限
func (api *API) checkPermission(r *http.Request, requiredRole config.UserRole) bool {
	username := httpauth.GetUsernameFromContext(r.Context())
	if username == "" {
		return false
	}

	userRole := api.userManager.GetUserRole(username)
	if userRole == "" {
		userRole = config.RoleNormalUser
	}

	// 权限检查逻辑
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
