package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// JWTSecret JWT 密钥（应该从配置文件读取，这里先使用默认值）
	JWTSecret = []byte("iarnet-secret-key-change-in-production")
	// TokenExpiration token 过期时间（24小时）
	TokenExpiration = 24 * time.Hour
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
