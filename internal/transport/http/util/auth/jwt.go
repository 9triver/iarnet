package auth

import (
	"errors"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// JWTSecret JWT 密钥（应该从配置文件读取，这里先使用默认值）
	JWTSecret = []byte("iarnet-secret-key-change-in-production")
	// TokenExpiration token 过期时间（24小时）
	TokenExpiration = 24 * time.Hour
	// tokenBlacklist token 黑名单（存储已退出的 token）
	tokenBlacklist = make(map[string]time.Time)
	// blacklistMu 保护 tokenBlacklist 的互斥锁
	blacklistMu sync.RWMutex
)

// Claims JWT Claims 结构
type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"` // 用户角色
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT token
func GenerateToken(username string, role string) (string, error) {
	expirationTime := time.Now().Add(TokenExpiration)
	claims := &Claims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(JWTSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken 验证 JWT token
func ValidateToken(tokenString string) (*Claims, error) {
	// 首先检查 token 是否在黑名单中
	blacklistMu.RLock()
	expirationTime, isBlacklisted := tokenBlacklist[tokenString]
	blacklistMu.RUnlock()

	if isBlacklisted {
		// 如果 token 在黑名单中，检查是否已过期（超过24小时）
		// 如果已过期，从黑名单中移除（避免内存泄漏）
		if time.Now().After(expirationTime) {
			blacklistMu.Lock()
			delete(tokenBlacklist, tokenString)
			blacklistMu.Unlock()
		} else {
			// Token 在黑名单中且未过期，拒绝访问
			return nil, errors.New("token has been revoked")
		}
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return JWTSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// SetSecret 设置 JWT 密钥（从配置读取）
func SetSecret(secret string) {
	if secret != "" {
		JWTSecret = []byte(secret)
	}
}

// GetJWTSecret 获取 JWT 密钥（用于解析 token）
func GetJWTSecret() []byte {
	return JWTSecret
}

// RevokeToken 将 token 加入黑名单，使其立即失效
// expirationTime 是 token 的过期时间，用于自动清理黑名单
func RevokeToken(tokenString string, expirationTime time.Time) {
	blacklistMu.Lock()
	defer blacklistMu.Unlock()
	tokenBlacklist[tokenString] = expirationTime
}

// IsTokenRevoked 检查 token 是否已被撤销
func IsTokenRevoked(tokenString string) bool {
	blacklistMu.RLock()
	defer blacklistMu.RUnlock()
	_, exists := tokenBlacklist[tokenString]
	return exists
}

// CleanExpiredTokens 清理过期的 token（避免内存泄漏）
// 应该在后台定期调用，例如每小时清理一次
func CleanExpiredTokens() {
	blacklistMu.Lock()
	defer blacklistMu.Unlock()

	now := time.Now()
	for token, expirationTime := range tokenBlacklist {
		if now.After(expirationTime) {
			delete(tokenBlacklist, token)
		}
	}
}
