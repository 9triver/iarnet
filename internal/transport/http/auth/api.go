package auth

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/9triver/iarnet/internal/config"
	userrepo "github.com/9triver/iarnet/internal/infra/repository/auth"
	httpauth "github.com/9triver/iarnet/internal/transport/http/util/auth"
	"github.com/9triver/iarnet/internal/transport/http/util/response"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func RegisterRoutes(router *mux.Router, cfg *config.Config, userRepo userrepo.UserRepo) {
	api := NewAPI(cfg, userRepo)
	router.HandleFunc("/auth/login", api.handleLogin).Methods("POST")
	router.HandleFunc("/auth/me", api.handleGetCurrentUser).Methods("GET")
	router.HandleFunc("/auth/change-password", api.handleChangePassword).Methods("POST")

	// 注册用户管理路由
	api.RegisterUserRoutes(router)

	// 注册恢复路由（紧急恢复超级管理员）
	api.RegisterRecoveryRoute(router)
}

type API struct {
	config      *config.Config
	userManager *UserManager
	userRepo    userrepo.UserRepo
}

func NewAPI(cfg *config.Config, userRepo userrepo.UserRepo) *API {
	return &API{
		config:      cfg,
		userManager: NewUserManager(cfg, userRepo),
		userRepo:    userRepo,
	}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Username string `json:"username"`
	Token    string `json:"token"`
}

type GetCurrentUserResponse struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

// hashSHA256 对字符串进行 SHA-256 哈希
func hashSHA256(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

// isHexString 检查字符串是否为有效的十六进制字符串
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func (api *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	// 验证用户名和密码
	username := strings.TrimSpace(req.Username)
	if username == "" {
		response.BadRequest("username is required").WriteJSON(w)
		return
	}

	// 从数据库查找用户
	ctx := r.Context()
	userDAO, err := api.userRepo.GetByName(ctx, username)
	if err != nil {
		// 检查是否是用户不存在的错误
		if errors.Is(err, sql.ErrNoRows) {
			// 用户不存在
			logrus.Warnf("User not found in database: %s", username)
			userDAO = nil
		} else {
			// 其他数据库错误
			logrus.Errorf("Failed to get user from database: %v", err)
			response.InternalError("failed to query user").WriteJSON(w)
			return
		}
	} else {
		logrus.Debugf("User found in database: %s (role: %s)", username, userDAO.Role)
	}

	sendInvalidCredentialResponse := func(remaining int) {
		resp := response.BusinessError(http.StatusUnauthorized, "用户名或密码错误", map[string]any{
			"remainingAttempts": remaining,
			"locked":            false,
		})
		resp.Error = fmt.Sprintf("invalid username or password, remaining attempts: %d", remaining)
		_ = resp.WriteJSON(w)
	}

	sendLockedResponse := func() {
		lockedUntil := api.userManager.GetLockedUntil(username)
		lockedUntilStr := ""
		if lockedUntil != nil {
			lockedUntilStr = lockedUntil.Format("2006-01-02 15:04:05")
		}
		resp := response.BusinessError(http.StatusUnauthorized, "账户已锁定", map[string]any{
			"remainingAttempts": 0,
			"locked":            true,
			"lockedUntil":       lockedUntilStr,
		})
		resp.Error = "account is locked due to too many failed login attempts"
		_ = resp.WriteJSON(w)
	}

	if userDAO == nil {
		logrus.Warnf("Login failed: user not found: %s", username)
		api.userManager.RecordLoginFailure(username)
		if api.userManager.IsUserLocked(username) {
			sendLockedResponse()
			return
		}
		remaining := api.userManager.GetRemainingAttempts(username)
		sendInvalidCredentialResponse(remaining)
		return
	}

	// 检查用户是否被锁定（超级管理员也会被锁定，需要通过恢复端点解锁）
	if api.userManager.IsUserLocked(username) {
		lockedUntil := api.userManager.GetLockedUntil(username)
		if lockedUntil != nil {
			logrus.Warnf("Login failed: user %s is locked until %v", username, lockedUntil)
			sendLockedResponse()
			return
		}
	}

	// 验证密码
	// 支持两种方式：
	// 1. 如果收到的密码是64位十六进制字符串（SHA-256哈希），则与数据库中的值直接比较（数据库存储的可能是哈希值）
	//    如果数据库存储的是明文，则对明文进行哈希后比较
	// 2. 如果收到的是明文，则对数据库中的值进行哈希后比较（如果数据库存储的是哈希），或直接比较（如果数据库存储的是明文）
	passwordMatch := false
	if len(req.Password) == 64 && isHexString(req.Password) {
		// 收到的密码是哈希值
		// 检查数据库中的密码是否是哈希值（64位十六进制）
		if len(userDAO.Password) == 64 && isHexString(userDAO.Password) {
			// 数据库存储的是哈希值，直接比较
			passwordMatch = strings.EqualFold(userDAO.Password, req.Password)
			logrus.Debugf("Password verification (hashed vs hashed): match=%v", passwordMatch)
		} else {
			// 数据库存储的是明文，对明文进行哈希后比较（向后兼容）
			hashedDBPassword := hashSHA256(userDAO.Password)
			passwordMatch = strings.EqualFold(hashedDBPassword, req.Password)
			logrus.Debugf("Password verification (hashed vs plaintext): match=%v", passwordMatch)
		}
	} else {
		// 收到的密码是明文（向后兼容）
		// 检查数据库中的密码是否是哈希值
		if len(userDAO.Password) == 64 && isHexString(userDAO.Password) {
			// 数据库存储的是哈希值，对收到的明文进行哈希后比较
			hashedReceivedPassword := hashSHA256(req.Password)
			passwordMatch = strings.EqualFold(userDAO.Password, hashedReceivedPassword)
			logrus.Debugf("Password verification (plaintext vs hashed): match=%v", passwordMatch)
		} else {
			// 数据库存储的是明文，直接比较（向后兼容）
			passwordMatch = userDAO.Password == req.Password
			logrus.Debugf("Password verification (plaintext vs plaintext): match=%v", passwordMatch)
		}
	}

	if !passwordMatch {
		logrus.Warnf("Login failed: incorrect password for user: %s", username)
		api.userManager.RecordLoginFailure(username)
		if api.userManager.IsUserLocked(username) {
			sendLockedResponse()
			return
		}
		remaining := api.userManager.GetRemainingAttempts(username)
		sendInvalidCredentialResponse(remaining)
		return
	}

	// 登录成功，清除失败记录
	api.userManager.RecordLoginSuccess(username)

	// 获取用户角色
	userRole := userDAO.Role
	if userRole == "" {
		// 如果角色为空，根据用户名判断默认角色
		if username == "admin" {
			userRole = config.RoleSuperAdmin
		} else {
			userRole = config.RoleNormalUser
		}
	}

	logrus.Infof("User logged in: %s (role: %s)", username, userRole)

	// 生成 JWT token（包含角色信息）
	token, err := httpauth.GenerateToken(username, string(userRole))
	if err != nil {
		logrus.Errorf("Failed to generate token: %v", err)
		response.InternalError("failed to generate token").WriteJSON(w)
		return
	}

	resp := LoginResponse{
		Username: username,
		Token:    token,
	}
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	// 从 context 中获取用户名和角色（由中间件注入）
	username := httpauth.GetUsernameFromContext(r.Context())
	role := httpauth.GetRoleFromContext(r.Context())
	if username == "" {
		response.Unauthorized("authentication required").WriteJSON(w)
		return
	}

	// 如果角色为空，从数据库获取
	if role == "" {
		ctx := r.Context()
		userDAO, err := api.userRepo.GetByName(ctx, username)
		if err == nil && userDAO != nil {
			role = string(userDAO.Role)
		}
		if role == "" {
			// 如果角色为空，根据用户名判断默认角色
			if username == "admin" {
				role = string(config.RoleSuperAdmin)
			} else {
				role = string(config.RoleNormalUser)
			}
		}
	}

	resp := GetCurrentUserResponse{
		Username: username,
		Role:     role,
	}
	response.Success(resp).WriteJSON(w)
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// handleChangePassword 处理用户修改自身密码的请求
func (api *API) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	// 获取当前登录用户
	username := httpauth.GetUsernameFromContext(r.Context())
	if username == "" {
		response.Unauthorized("未登录").WriteJSON(w)
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	// 验证输入
	if req.OldPassword == "" {
		response.BadRequest("旧密码不能为空").WriteJSON(w)
		return
	}
	if req.NewPassword == "" {
		response.BadRequest("新密码不能为空").WriteJSON(w)
		return
	}

	// 从数据库获取用户信息
	ctx := r.Context()
	userDAO, err := api.userRepo.GetByName(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.NotFound("用户不存在").WriteJSON(w)
			return
		}
		logrus.Errorf("Failed to get user from database: %v", err)
		response.InternalError("获取用户信息失败").WriteJSON(w)
		return
	}

	// 验证旧密码
	// 支持两种方式：
	// 1. 如果收到的密码是64位十六进制字符串（SHA-256哈希），则与数据库中的值直接比较（数据库存储的可能是哈希值）
	//    如果数据库存储的是明文，则对明文进行哈希后比较
	// 2. 如果收到的是明文，则对数据库中的值进行哈希后比较（如果数据库存储的是哈希），或直接比较（如果数据库存储的是明文）
	passwordMatch := false
	if len(req.OldPassword) == 64 && isHexString(req.OldPassword) {
		// 收到的密码是哈希值
		// 检查数据库中的密码是否是哈希值（64位十六进制）
		if len(userDAO.Password) == 64 && isHexString(userDAO.Password) {
			// 数据库存储的是哈希值，直接比较
			passwordMatch = strings.EqualFold(userDAO.Password, req.OldPassword)
		} else {
			// 数据库存储的是明文，对明文进行哈希后比较（向后兼容）
			hashedDBPassword := hashSHA256(userDAO.Password)
			passwordMatch = strings.EqualFold(hashedDBPassword, req.OldPassword)
		}
	} else {
		// 收到的密码是明文（向后兼容）
		// 检查数据库中的密码是否是哈希值
		if len(userDAO.Password) == 64 && isHexString(userDAO.Password) {
			// 数据库存储的是哈希值，对收到的明文进行哈希后比较
			hashedReceivedPassword := hashSHA256(req.OldPassword)
			passwordMatch = strings.EqualFold(userDAO.Password, hashedReceivedPassword)
		} else {
			// 数据库存储的是明文，直接比较（向后兼容）
			passwordMatch = userDAO.Password == req.OldPassword
		}
	}

	if !passwordMatch {
		response.BadRequest("旧密码不正确").WriteJSON(w)
		return
	}

	// 更新密码
	// 前端发送的是哈希值（64位十六进制），我们直接存储哈希值
	// 这样可以避免在传输过程中暴露明文密码
	newPassword := req.NewPassword
	if len(req.NewPassword) == 64 && isHexString(req.NewPassword) {
		// 收到哈希值，直接存储
		userDAO.Password = newPassword
	} else {
		// 收到明文（向后兼容），先哈希再存储
		userDAO.Password = hashSHA256(newPassword)
	}

	if err := api.userRepo.Update(ctx, userDAO); err != nil {
		logrus.Errorf("Failed to update password: %v", err)
		response.InternalError("更新密码失败").WriteJSON(w)
		return
	}

	logrus.Infof("User %s changed password", username)

	response.Success(map[string]string{"message": "密码修改成功"}).WriteJSON(w)
}
