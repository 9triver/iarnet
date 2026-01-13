package auth

import (
	"context"
	"sync"
	"time"

	"github.com/9triver/iarnet/internal/config"
	userrepo "github.com/9triver/iarnet/internal/infra/repository/auth"
	"github.com/sirupsen/logrus"
)

// LoginAttempt 登录尝试记录
type LoginAttempt struct {
	FailedCount    int        // 失败次数
	LastFailedTime time.Time  // 最后失败时间
	LockedUntil    *time.Time // 锁定到期时间（nil 表示未锁定）
}

// UserManager 用户管理器
type UserManager struct {
	config         *config.Config
	userRepo       userrepo.UserRepo
	loginAttempts  map[string]*LoginAttempt // 用户名 -> 登录尝试记录
	mu             sync.RWMutex             // 保护 loginAttempts 的读写锁
	maxFailedCount int                      // 最大失败次数（默认5次）
	lockDuration   time.Duration            // 锁定时长（默认30分钟）
}

// NewUserManager 创建用户管理器
func NewUserManager(cfg *config.Config, userRepo userrepo.UserRepo) *UserManager {
	return &UserManager{
		config:         cfg,
		userRepo:       userRepo,
		loginAttempts:  make(map[string]*LoginAttempt),
		maxFailedCount: 5,
		lockDuration:   30 * time.Minute,
	}
}

// RecordLoginFailure 记录登录失败
func (um *UserManager) RecordLoginFailure(username string) {
	um.mu.Lock()
	defer um.mu.Unlock()

	attempt, exists := um.loginAttempts[username]
	if !exists {
		attempt = &LoginAttempt{
			FailedCount:    0,
			LastFailedTime: time.Now(),
		}
		um.loginAttempts[username] = attempt
	}

	attempt.FailedCount++
	attempt.LastFailedTime = time.Now()

	// 如果失败次数达到阈值，锁定账户
	if attempt.FailedCount >= um.maxFailedCount {
		lockedUntil := time.Now().Add(um.lockDuration)
		attempt.LockedUntil = &lockedUntil
		logrus.Warnf("User %s locked due to %d failed login attempts. Locked until %v", username, attempt.FailedCount, lockedUntil)
	} else {
		logrus.Warnf("User %s failed login attempt %d/%d", username, attempt.FailedCount, um.maxFailedCount)
	}
}

// RecordLoginSuccess 记录登录成功，清除失败记录
func (um *UserManager) RecordLoginSuccess(username string) {
	um.mu.Lock()
	defer um.mu.Unlock()

	delete(um.loginAttempts, username)
}

// IsUserLocked 检查用户是否被锁定
func (um *UserManager) IsUserLocked(username string) bool {
	um.mu.RLock()
	defer um.mu.RUnlock()

	attempt, exists := um.loginAttempts[username]
	if !exists {
		return false
	}

	if attempt.LockedUntil == nil {
		return false
	}

	// 检查锁定是否已过期
	if time.Now().After(*attempt.LockedUntil) {
		// 锁定已过期，清除记录
		um.mu.RUnlock()
		um.mu.Lock()
		delete(um.loginAttempts, username)
		um.mu.Unlock()
		um.mu.RLock()
		return false
	}

	return true
}

// GetLockedUntil 获取用户锁定到期时间
func (um *UserManager) GetLockedUntil(username string) *time.Time {
	um.mu.RLock()
	defer um.mu.RUnlock()

	attempt, exists := um.loginAttempts[username]
	if !exists || attempt.LockedUntil == nil {
		return nil
	}

	// 检查锁定是否已过期
	if time.Now().After(*attempt.LockedUntil) {
		return nil
	}

	return attempt.LockedUntil
}

// GetFailedCount 获取用户失败次数
func (um *UserManager) GetFailedCount(username string) int {
	um.mu.RLock()
	defer um.mu.RUnlock()

	attempt, exists := um.loginAttempts[username]
	if !exists {
		return 0
	}

	return attempt.FailedCount
}

// UnlockUser 手动解锁用户
func (um *UserManager) UnlockUser(username string) {
	um.mu.Lock()
	defer um.mu.Unlock()

	delete(um.loginAttempts, username)
	logrus.Infof("User %s unlocked manually", username)
}

// GetRemainingAttempts 获取剩余可重试次数
func (um *UserManager) GetRemainingAttempts(username string) int {
	um.mu.RLock()
	defer um.mu.RUnlock()

	remaining := um.maxFailedCount
	if attempt, exists := um.loginAttempts[username]; exists {
		remaining = um.maxFailedCount - attempt.FailedCount
	}
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetUserRole 获取用户角色
func (um *UserManager) GetUserRole(username string) config.UserRole {
	ctx := context.Background()
	user, err := um.userRepo.GetByName(ctx, username)
	if err != nil {
		// 用户不存在，返回普通用户（默认）
		return config.RoleNormalUser
	}

	if user.Role == "" {
		// 如果没有设置角色，根据用户名判断：
		// - admin 用户默认为超级管理员
		// - 其他用户默认为普通用户
		if username == "admin" {
			return config.RoleSuperAdmin
		}
		return config.RoleNormalUser
	}
	return user.Role
}

// GetUser 获取用户信息
func (um *UserManager) GetUser(username string) (*userrepo.UserDAO, error) {
	ctx := context.Background()
	return um.userRepo.GetByName(ctx, username)
}
