// Package account 负责注册、登录、会话、个人资料和团队。
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

// AccountProfileRequest 修改当前用户的显示名称。用户名创建后不可改。
type AccountProfileRequest struct {
	DisplayName string `json:"display_name"`
}

// AccountPasswordRequest 校验当前口令后更换口令。当前会话保持有效。
type AccountPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// TeamCreateRequest 创建一个由当前用户所有的团队。
type TeamCreateRequest struct {
	Name string `json:"name"`
}

// TeamUpdateRequest 重命名团队。只有所有者可以修改。
type TeamUpdateRequest struct {
	ID   int64  `json:"id,string"`
	Name string `json:"name"`
}

// TeamIDRequest 读取或删除一个团队。
type TeamIDRequest struct {
	ID int64 `json:"id,string"`
}

// TeamSearch 列出当前用户加入的团队。
type TeamSearch struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// TeamInviteRequest 按用户名邀请成员。
type TeamInviteRequest struct {
	TeamID   int64  `json:"team_id,string"`
	Username string `json:"username"`
}

// TeamMemberRemoveRequest 由所有者移出一名成员。
type TeamMemberRemoveRequest struct {
	TeamID int64 `json:"team_id,string"`
	UserID int64 `json:"user_id,string"`
}

// TeamLeaveRequest 由成员退出团队。所有者需要改为删除团队。
type TeamLeaveRequest struct {
	TeamID int64 `json:"team_id,string"`
}

// TeamResponse 是团队列表项，Role 是当前用户在该团队中的角色。
type TeamResponse struct {
	ID        int64     `json:"id,string"`
	Name      string    `json:"name"`
	OwnerID   int64     `json:"owner_id,string"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// TeamMemberResponse 是团队成员。
type TeamMemberResponse struct {
	UserID      int64     `json:"user_id,string"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

// TeamDetail 带有成员名单。
type TeamDetail struct {
	TeamResponse
	Members []TeamMemberResponse `json:"members"`
}
