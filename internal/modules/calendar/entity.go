package calendar

import "time"

// Event 是当前用户的一条日程。全天日程用 StartOn 和 EndOn 保存闭区间日期，
// EndsAt 是结束日的下一天零点，只供时间范围查询。定时日程的日期字段为空。
type Event struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	OwnerID   int64     `gorm:"not null;index:idx_calendar_event_owner_time,priority:1;index:idx_calendar_event_owner_day,priority:1"`
	Title     string    `gorm:"size:80;not null"`
	AllDay    bool      `gorm:"not null"`
	StartsAt  time.Time `gorm:"not null;index:idx_calendar_event_owner_time,priority:2"`
	EndsAt    time.Time `gorm:"not null;index:idx_calendar_event_owner_time,priority:3"`
	StartOn   string    `gorm:"size:10;not null;index:idx_calendar_event_owner_day,priority:2"`
	EndOn     string    `gorm:"size:10;not null;index:idx_calendar_event_owner_day,priority:3"`
	Location  string    `gorm:"size:80;not null"`
	Notes     string    `gorm:"size:2000;not null"`
	Color     string    `gorm:"size:7;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Event) TableName() string { return "calendar_event" }
