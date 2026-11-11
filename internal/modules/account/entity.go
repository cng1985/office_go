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
