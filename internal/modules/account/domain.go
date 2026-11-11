// Package account 负责注册、登录、会话和当前用户。
package account

import "time"

// AccountRegisterRequest 是注册参数。口令摘要和过期时间由服务端写入，不来自客户端。
type AccountRegisterRequest struct {
	Username     string    `json:"username"`
	Password     string    `json:"password"`
	DisplayName  string    `json:"display_name"`
	PasswordHash string    `json:"-"`
	ExpiresAt    time.Time `json:"-"`
}

// AccountLoginRequest 是登录参数。
type AccountLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AccountSessionRequest 是已通过口令校验后的开会话参数。
type AccountSessionRequest struct {
	UserID    int64     `json:"-"`
	ExpiresAt time.Time `json:"-"`
}

// AccountLogoutRequest 不接受客户端指定的用户或会话。
type AccountLogoutRequest struct{}

// AccountMeRequest 读取当前可信身份。
type AccountMeRequest struct{}

// AccountResponse 是可公开的用户资料。
type AccountResponse struct {
	ID          int64     `json:"id,string"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// AccountSessionResponse 是登录或注册结果。Token 只用于设置 HttpOnly Cookie，不回给页面脚本。
type AccountSessionResponse struct {
	Account   AccountResponse `json:"account"`
	ExpiresAt time.Time       `json:"expires_at"`
	Token     string          `json:"token,omitempty"`
}
