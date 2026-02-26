package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/domain/audit"
	audittypes "github.com/9triver/iarnet/internal/domain/audit/types"
	userrepo "github.com/9triver/iarnet/internal/infra/repository/auth"
	httpauth "github.com/9triver/iarnet/internal/transport/http/util/auth"
	"github.com/9triver/iarnet/internal/transport/http/util/response"
	"github.com/9triver/iarnet/internal/util"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// PasswordMaxAge 密码最长有效期：3 个月（约 90 天）
const PasswordMaxAge = 90 * 24 * time.Hour

func RegisterRoutes(router *mux.Router, cfg *config.Config, userRepo userrepo.UserRepo, auditMgr *audit.Manager) {
	api := NewAPI(cfg, userRepo, auditMgr)
	router.HandleFunc("/auth/login", api.handleLogin).Methods("POST")
	router.HandleFunc("/auth/logout", api.handleLogout).Methods("POST")
	router.HandleFunc("/auth/me", api.handleGetCurrentUser).Methods("GET")
	router.HandleFunc("/auth/change-password", api.handleChangePassword).Methods("POST")
	router.HandleFunc("/auth/change-password-with-credential", api.handleChangePasswordWithCredential).Methods("POST")

	// 注册用户管理路由
	api.RegisterUserRoutes(router)

	// 注册恢复路由（紧急恢复超级管理员）
	api.RegisterRecoveryRoute(router)
}

type API struct {
	config      *config.Config
	userManager *UserManager
	userRepo    userrepo.UserRepo
	auditMgr    *audit.Manager
}

func NewAPI(cfg *config.Config, userRepo userrepo.UserRepo, auditMgr *audit.Manager) *API {
	return &API{
		config:      cfg,
		userManager: NewUserManager(cfg, userRepo),
		userRepo:    userRepo,
		auditMgr:    auditMgr,
	}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Username          string `json:"username"`
	Token             string `json:"token"`
	Role              string `json:"role,omitempty"`
	PasswordExpired   bool   `json:"password_expired"`
	PasswordExpiresAt string `json:"password_expires_at,omitempty"`
}

type GetCurrentUserResponse struct {
	Username          string `json:"username"`
	Role              string `json:"role"`
	PasswordExpired   bool   `json:"password_expired"`
	PasswordExpiresAt string `json:"password_expires_at,omitempty"`
}

// hashSHA256 和 isHexString 函数已移至 password.go
// 为了向后兼容，这里保留这些函数的引用（通过 password.go 导出）

// validatePasswordComplexity 验证密码复杂度
// 要求：8-16位，包含大小写字母、数字、特殊字符
func validatePasswordComplexity(password string) error {
	// 如果是哈希值（64位十六进制），先不验证复杂度（因为前端已经验证过了）
	// 但我们需要在前端发送哈希值之前验证原始密码
	// 这里我们假设如果密码是64位十六进制，说明前端已经验证过，我们只验证长度
	if len(password) == 64 && isHexString(password) {
		// 这是哈希值，无法验证原始密码复杂度
		// 但我们可以要求前端在发送前验证
		return nil
	}

	// 验证明文密码复杂度
	if len(password) < 8 || len(password) > 16 {
		return errors.New("密码长度必须为8-16位")
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	// 特殊字符正则表达式
	specialCharRegex := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>/?]`)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case specialCharRegex.MatchString(string(char)):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("密码必须包含至少一个大写字母")
	}
	if !hasLower {
		return errors.New("密码必须包含至少一个小写字母")
	}
	if !hasDigit {
		return errors.New("密码必须包含至少一个数字")
	}
	if !hasSpecial {
		return errors.New("密码必须包含至少一个特殊字符")
	}

	return nil
}

// calculatePasswordExpiry 计算密码是否已过期以及过期时间
func calculatePasswordExpiry(user *userrepo.UserDAO) (expired bool, expiresAt *time.Time) {
	if user == nil {
		return false, nil
	}

	// 如果没有记录密码修改时间，则认为未开启过期策略
	if user.PasswordChangedAt.IsZero() {
		return false, nil
	}

	expireTime := user.PasswordChangedAt.Add(PasswordMaxAge)
	now := time.Now()
	if now.After(expireTime) {
		return true, &expireTime
	}
	return false, &expireTime
}

// recordOperation 记录认证与账号相关的操作日志
func (api *API) recordOperation(r *http.Request, operation audittypes.OperationType, resourceType, resourceID, action string, before, after map[string]interface{}) {
	if api.auditMgr == nil {
		return
	}

	// 从 context 中获取用户信息（由认证中间件注入；登录场景可能为空）
	user := httpauth.GetUsernameFromContext(r.Context())
	if user == "" {
		user = "anonymous"
	}

	ip := getClientIP(r)
	logID := util.GenIDWith("op.auth.")

	opLog := &audittypes.OperationLog{
		ID:           logID,
		User:         user,
		Operation:    operation,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
		Before:       before,
		After:        after,
		Timestamp:    time.Now(),
		IP:           ip,
	}

	// 异步记录，避免影响主流程
	go func() {
		ctx := context.Background()
		if err := api.auditMgr.RecordOperation(ctx, opLog); err != nil {
			logrus.Errorf("Failed to record auth operation log: %v", err)
		}
	}()
}

// getClientIP 获取客户端IP地址
func getClientIP(r *http.Request) string {
	// 检查 X-Forwarded-For 头（代理/负载均衡器）
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// 检查 X-Real-IP 头
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// 直接从 RemoteAddr 获取
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
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

	// 检查账户是否被停用
	if userDAO.Status == "disabled" {
		logrus.Warnf("Login blocked: account disabled for user %s", username)
		resp := response.BusinessError(http.StatusForbidden, "账户已停用，请联系管理员启用后再登录", map[string]any{
			"disabled": true,
		})
		resp.Error = "account_disabled"
		_ = resp.WriteJSON(w)
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
	// 前端发送的是 SHA-256 哈希值，数据库存储的是 bcrypt 哈希值
	// 支持向后兼容：如果数据库存储的是旧格式（SHA-256 哈希或明文），也进行验证
	passwordMatch := false

	// 首先尝试使用 bcrypt 验证（新格式）
	if strings.HasPrefix(userDAO.Password, "$2a$") || strings.HasPrefix(userDAO.Password, "$2b$") || strings.HasPrefix(userDAO.Password, "$2y$") {
		// 数据库存储的是 bcrypt 哈希值
		passwordMatch = VerifyPassword(req.Password, userDAO.Password)
		logrus.Debugf("Password verification (bcrypt): match=%v", passwordMatch)
	} else {
		// 向后兼容：数据库存储的是旧格式（SHA-256 哈希或明文）
		if len(req.Password) == 64 && isHexString(req.Password) {
			// 收到的密码是哈希值
			if len(userDAO.Password) == 64 && isHexString(userDAO.Password) {
				// 数据库存储的是 SHA-256 哈希值，直接比较
				passwordMatch = strings.EqualFold(userDAO.Password, req.Password)
				logrus.Debugf("Password verification (hashed vs hashed, legacy): match=%v", passwordMatch)
			} else {
				// 数据库存储的是明文，对明文进行哈希后比较（向后兼容）
				hashedDBPassword := hashSHA256(userDAO.Password)
				passwordMatch = strings.EqualFold(hashedDBPassword, req.Password)
				logrus.Debugf("Password verification (hashed vs plaintext, legacy): match=%v", passwordMatch)
			}
		} else {
			// 收到的密码是明文（向后兼容）
			if len(userDAO.Password) == 64 && isHexString(userDAO.Password) {
				// 数据库存储的是 SHA-256 哈希值，对收到的明文进行哈希后比较
				hashedReceivedPassword := hashSHA256(req.Password)
				passwordMatch = strings.EqualFold(userDAO.Password, hashedReceivedPassword)
				logrus.Debugf("Password verification (plaintext vs hashed, legacy): match=%v", passwordMatch)
			} else {
				// 数据库存储的是明文，直接比较（向后兼容）
				passwordMatch = userDAO.Password == req.Password
				logrus.Debugf("Password verification (plaintext vs plaintext, legacy): match=%v", passwordMatch)
			}
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

	// 计算密码是否已过期以及过期时间
	passwordExpired, passwordExpiresAt := calculatePasswordExpiry(userDAO)

	// 如果密码已过期，则阻止登录，要求先修改密码
	if passwordExpired {
		var passwordExpiresAtStr string
		if passwordExpiresAt != nil {
			passwordExpiresAtStr = passwordExpiresAt.Format("2006-01-02 15:04:05")
		}
		logrus.Warnf("Login blocked: password expired for user %s", username)
		resp := response.BusinessError(http.StatusForbidden, "密码已过期，需要先修改密码后才能登录", map[string]any{
			"passwordExpired":   true,
			"passwordExpiresAt": passwordExpiresAtStr,
		})
		// 使用 error 字段作为机器可读错误码，便于前端识别
		resp.Error = "password_expired"
		_ = resp.WriteJSON(w)
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

	var passwordExpiresAtStr string
	if passwordExpiresAt != nil {
		passwordExpiresAtStr = passwordExpiresAt.Format("2006-01-02 15:04:05")
	}

	resp := LoginResponse{
		Username:          username,
		Token:             token,
		Role:              string(userRole),
		PasswordExpired:   passwordExpired,
		PasswordExpiresAt: passwordExpiresAtStr,
	}
	response.Success(resp).WriteJSON(w)

	// 记录登录操作日志
	api.recordOperation(r, audittypes.OperationTypeUserLogin, "user", username,
		fmt.Sprintf("用户登录系统: %s", username), nil, map[string]interface{}{
			"role":              userRole,
			"password_expires":  passwordExpiresAtStr,
			"password_expired":  passwordExpired,
		})
}

func (api *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	// 获取当前登录用户
	username := httpauth.GetUsernameFromContext(r.Context())

	// 从请求头获取 token
	authHeader := r.Header.Get("Authorization")
	var tokenString string
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			tokenString = parts[1]
		}
	}

	// 将 token 加入黑名单，使其立即失效
	if tokenString != "" {
		// 解析 token 获取过期时间（不验证黑名单，因为我们要将其加入黑名单）
		expirationTime, err := parseTokenForRevocation(tokenString)
		if err == nil {
			// 将 token 加入黑名单
			httpauth.RevokeToken(tokenString, expirationTime)
			if username != "" {
				logrus.Infof("User logged out: %s, token revoked", username)
			} else {
				logrus.Infof("Token revoked (user not authenticated)")
			}
		} else {
			// 如果无法解析 token，仍然尝试加入黑名单（使用默认过期时间）
			// 这样可以防止无效 token 被重复使用
			defaultExpiration := time.Now().Add(24 * time.Hour)
			httpauth.RevokeToken(tokenString, defaultExpiration)
			if username != "" {
				logrus.Infof("User logged out: %s (token parse failed, but added to blacklist)", username)
			}
		}
	} else if username != "" {
		logrus.Infof("User logged out: %s (no token provided)", username)
	}

	// 如果用户未登录，也返回成功（幂等性）
	response.Success(map[string]string{"message": "退出登录成功"}).WriteJSON(w)

	// 记录登出操作日志
	if username != "" {
		api.recordOperation(r, audittypes.OperationTypeUserLogout, "user", username,
			fmt.Sprintf("用户退出登录: %s", username), nil, nil)
	}
}

// parseTokenForRevocation 解析 token 用于撤销（不检查黑名单）
// 返回过期时间和错误
func parseTokenForRevocation(tokenString string) (time.Time, error) {
	// 定义 Claims 结构（与 jwt.go 中的一致）
	type Claims struct {
		Username string `json:"username"`
		Role     string `json:"role"`
		jwt.RegisteredClaims
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return httpauth.GetJWTSecret(), nil
	})

	if err != nil {
		return time.Time{}, err
	}

	if !token.Valid {
		return time.Time{}, errors.New("invalid token")
	}

	// 获取 token 的过期时间
	expirationTime := time.Now().Add(24 * time.Hour) // 默认24小时
	if claims.ExpiresAt != nil {
		expirationTime = claims.ExpiresAt.Time
	}

	return expirationTime, nil
}

func (api *API) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	// 从 context 中获取用户名和角色（由中间件注入）
	username := httpauth.GetUsernameFromContext(r.Context())
	role := httpauth.GetRoleFromContext(r.Context())
	if username == "" {
		response.Unauthorized("authentication required").WriteJSON(w)
		return
	}

	ctx := r.Context()
	userDAO, err := api.userRepo.GetByName(ctx, username)
	if err != nil {
		logrus.Errorf("Failed to get user when handling /auth/me: %v", err)
	}

	// 如果角色为空，从数据库获取
	if role == "" {
		if userDAO != nil {
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

	// 根据用户信息计算密码过期状态
	passwordExpired, passwordExpiresAt := calculatePasswordExpiry(userDAO)
	var passwordExpiresAtStr string
	if passwordExpiresAt != nil {
		passwordExpiresAtStr = passwordExpiresAt.Format("2006-01-02 15:04:05")
	}

	resp := GetCurrentUserResponse{
		Username:          username,
		Role:              role,
		PasswordExpired:   passwordExpired,
		PasswordExpiresAt: passwordExpiresAtStr,
	}
	response.Success(resp).WriteJSON(w)
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePasswordWithCredentialRequest 使用用户名+旧密码修改密码（无需已登录）
type ChangePasswordWithCredentialRequest struct {
	Username    string `json:"username"`
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

	// 验证新密码复杂度
	// 注意：如果前端发送的是哈希值，我们需要在前端验证原始密码复杂度
	// 这里我们验证明文密码（如果前端发送的是明文）或要求前端在发送哈希前验证
	if len(req.NewPassword) != 64 || !isHexString(req.NewPassword) {
		// 这是明文密码，验证复杂度
		if err := validatePasswordComplexity(req.NewPassword); err != nil {
			response.BadRequest(err.Error()).WriteJSON(w)
			return
		}
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
	// 前端发送的是 SHA-256 哈希值，数据库存储的是 bcrypt 哈希值
	// 支持向后兼容：如果数据库存储的是旧格式（SHA-256 哈希或明文），也进行验证
	passwordMatch := false

	// 首先尝试使用 bcrypt 验证（新格式）
	if strings.HasPrefix(userDAO.Password, "$2a$") || strings.HasPrefix(userDAO.Password, "$2b$") || strings.HasPrefix(userDAO.Password, "$2y$") {
		// 数据库存储的是 bcrypt 哈希值
		passwordMatch = VerifyPassword(req.OldPassword, userDAO.Password)
	} else {
		// 向后兼容：数据库存储的是旧格式（SHA-256 哈希或明文）
		if len(req.OldPassword) == 64 && isHexString(req.OldPassword) {
			// 收到的密码是哈希值
			if len(userDAO.Password) == 64 && isHexString(userDAO.Password) {
				// 数据库存储的是 SHA-256 哈希值，直接比较
				passwordMatch = strings.EqualFold(userDAO.Password, req.OldPassword)
			} else {
				// 数据库存储的是明文，对明文进行哈希后比较（向后兼容）
				hashedDBPassword := hashSHA256(userDAO.Password)
				passwordMatch = strings.EqualFold(hashedDBPassword, req.OldPassword)
			}
		} else {
			// 收到的密码是明文（向后兼容）
			if len(userDAO.Password) == 64 && isHexString(userDAO.Password) {
				// 数据库存储的是 SHA-256 哈希值，对收到的明文进行哈希后比较
				hashedReceivedPassword := hashSHA256(req.OldPassword)
				passwordMatch = strings.EqualFold(userDAO.Password, hashedReceivedPassword)
			} else {
				// 数据库存储的是明文，直接比较（向后兼容）
				passwordMatch = userDAO.Password == req.OldPassword
			}
		}
	}

	if !passwordMatch {
		logrus.Warnf("Change password failed: incorrect old password for user %s", username)
		response.BadRequest("旧密码不正确").WriteJSON(w)
		return
	}

	// 更新密码
	// 前端发送的是 SHA-256 哈希值，使用 bcrypt 加密存储
	hashedPassword, err := HashPassword(req.NewPassword)
	if err != nil {
		logrus.Errorf("Failed to hash password: %v", err)
		response.InternalError("更新密码失败：密码加密错误").WriteJSON(w)
		return
	}
	userDAO.Password = hashedPassword

	if err := api.userRepo.Update(ctx, userDAO, true); err != nil {
		logrus.Errorf("Failed to update password: %v", err)
		response.InternalError("更新密码失败").WriteJSON(w)
		return
	}

	logrus.Infof("User %s changed password", username)

	response.Success(map[string]string{"message": "密码修改成功"}).WriteJSON(w)

	// 记录自助修改密码操作日志
	api.recordOperation(r, audittypes.OperationTypeUserChangePassword, "user", username,
		fmt.Sprintf("用户自助修改密码: %s", username), nil, map[string]interface{}{
			"method": "self_service",
		})
}

// handleChangePasswordWithCredential 使用用户名和旧密码修改密码（用于密码过期后强制修改）
// 注意：该接口不依赖登录态，因此在中间件中已放行，但会严格校验用户名和旧密码是否匹配。
func (api *API) handleChangePasswordWithCredential(w http.ResponseWriter, r *http.Request) {
	var req ChangePasswordWithCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		response.BadRequest("用户名不能为空").WriteJSON(w)
		return
	}
	if req.OldPassword == "" {
		response.BadRequest("旧密码不能为空").WriteJSON(w)
		return
	}
	if req.NewPassword == "" {
		response.BadRequest("新密码不能为空").WriteJSON(w)
		return
	}

	// 验证新密码复杂度（同 handleChangePassword）
	if len(req.NewPassword) != 64 || !isHexString(req.NewPassword) {
		if err := validatePasswordComplexity(req.NewPassword); err != nil {
			response.BadRequest(err.Error()).WriteJSON(w)
			return
		}
	}

	// 从数据库获取用户
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

	// 验证旧密码（逻辑与登录/handleChangePassword 保持一致）
	passwordMatch := false
	if strings.HasPrefix(userDAO.Password, "$2a$") || strings.HasPrefix(userDAO.Password, "$2b$") || strings.HasPrefix(userDAO.Password, "$2y$") {
		passwordMatch = VerifyPassword(req.OldPassword, userDAO.Password)
	} else {
		if len(req.OldPassword) == 64 && isHexString(req.OldPassword) {
			if len(userDAO.Password) == 64 && isHexString(userDAO.Password) {
				passwordMatch = strings.EqualFold(userDAO.Password, req.OldPassword)
			} else {
				hashedDBPassword := hashSHA256(userDAO.Password)
				passwordMatch = strings.EqualFold(hashedDBPassword, req.OldPassword)
			}
		} else {
			if len(userDAO.Password) == 64 && isHexString(userDAO.Password) {
				hashedReceivedPassword := hashSHA256(req.OldPassword)
				passwordMatch = strings.EqualFold(userDAO.Password, hashedReceivedPassword)
			} else {
				passwordMatch = userDAO.Password == req.OldPassword
			}
		}
	}

	if !passwordMatch {
		logrus.Warnf("Change password with credential failed: incorrect old password for user %s", username)
		response.BadRequest("旧密码不正确").WriteJSON(w)
		return
	}

	// 更新密码
	hashedPassword, err := HashPassword(req.NewPassword)
	if err != nil {
		logrus.Errorf("Failed to hash password: %v", err)
		response.InternalError("更新密码失败：密码加密错误").WriteJSON(w)
		return
	}
	userDAO.Password = hashedPassword

	if err := api.userRepo.Update(ctx, userDAO, true); err != nil {
		logrus.Errorf("Failed to update password (with credential): %v", err)
		response.InternalError("更新密码失败").WriteJSON(w)
		return
	}

	logrus.Infof("User %s changed password via credential endpoint", username)

	response.Success(map[string]string{"message": "密码修改成功，请重新登录"}).WriteJSON(w)

	// 记录通过凭证修改密码的操作日志（通常用于密码过期场景）
	api.recordOperation(r, audittypes.OperationTypeUserChangePassword, "user", username,
		fmt.Sprintf("用户通过凭证修改密码: %s", username), nil, map[string]interface{}{
			"method": "credential",
		})
}
