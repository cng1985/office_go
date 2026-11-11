// Package auth 提供口令、会话令牌和认证契约，不依赖 Gin。
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

const SessionCookie = "yunjian_session"

// HashToken 保存令牌摘要。数据库和日志都不存放原始会话令牌。
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewToken 生成浏览器 cookie 和调用方持有的原始会话令牌。
func NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
