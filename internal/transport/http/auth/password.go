package auth

import (
	"crypto/sha256"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost bcrypt 加密成本（值越大越安全，但计算时间越长）
	// 推荐值：10-12，这里使用 10 作为平衡点
	BcryptCost = 10
)

// HashPassword 使用 bcrypt 加密密码
// 如果密码是 SHA-256 哈希值（64位十六进制），直接使用 bcrypt 加密
// 如果是明文，先进行 SHA-256 哈希，再用 bcrypt 加密
func HashPassword(password string) (string, error) {
	// 检查是否是 SHA-256 哈希值（64位十六进制）
	var passwordToHash string
	if len(password) == 64 && isHexString(password) {
		// 是哈希值，直接使用
		passwordToHash = password
	} else {
		// 是明文，先进行 SHA-256 哈希
		passwordToHash = hashSHA256(password)
	}

	// 使用 bcrypt 加密
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(passwordToHash), BcryptCost)
	if err != nil {
		return "", err
	}

	return string(hashedBytes), nil
}

// VerifyPassword 验证密码
// password: 前端发送的密码（可能是 SHA-256 哈希值或明文）
// hashedPassword: 数据库中存储的 bcrypt 哈希值
func VerifyPassword(password, hashedPassword string) bool {
	// 检查是否是 SHA-256 哈希值（64位十六进制）
	var passwordToVerify string
	if len(password) == 64 && isHexString(password) {
		// 是哈希值，直接使用
		passwordToVerify = password
	} else {
		// 是明文，先进行 SHA-256 哈希
		passwordToVerify = hashSHA256(password)
	}

	// 使用 bcrypt 验证
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(passwordToVerify))
	return err == nil
}

// hashSHA256 对字符串进行 SHA-256 哈希
// 导出此函数以供向后兼容使用
func hashSHA256(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

// isHexString 检查字符串是否为有效的十六进制字符串
// 导出此函数以供向后兼容使用
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
