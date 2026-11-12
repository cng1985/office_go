package account

import "time"

// User 是账号表。
type User struct {
	ID           int64  `gorm:"primaryKey;autoIncrement"`
	Username     string `gorm:"size:32;not null;uniqueIndex:uk_account_user_username"`
	DisplayName  string `gorm:"size:64;not null"`
	PasswordHash string `gorm:"size:100;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (User) TableName() string { return "account_user" }

// Session 是服务端会话。只保存令牌摘要。
type Session struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	UserID    int64     `gorm:"not null;index:idx_account_session_user"`
	TokenHash string    `gorm:"size:64;not null;uniqueIndex:uk_account_session_token"`
	ExpiresAt time.Time `gorm:"not null"`
	RevokedAt *time.Time
	CreatedAt time.Time
}

func (Session) TableName() string { return "account_session" }

// Team 是协作空间。文档和文件夹通过 team_id 挂在这里，不在本表级联删除。
type Team struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	OwnerID   int64  `gorm:"not null;index:idx_account_team_owner"`
	Name      string `gorm:"size:40;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Team) TableName() string { return "account_team" }

// TeamMember 记录用户在团队中的角色。所有者同时是一名成员。
type TeamMember struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	TeamID    int64  `gorm:"not null;uniqueIndex:uk_account_team_member,priority:1"`
	UserID    int64  `gorm:"not null;uniqueIndex:uk_account_team_member,priority:2;index:idx_account_team_member_user"`
	Role      string `gorm:"size:16;not null"`
	CreatedAt time.Time
}

func (TeamMember) TableName() string { return "account_team_member" }
