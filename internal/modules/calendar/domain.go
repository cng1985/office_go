package calendar

import "time"

// EventWriteRequest 创建或更新一条日程。全天日程使用日期，定时日程使用时刻。
type EventWriteRequest struct {
	ID       int64     `json:"id,string"`
	Title    string    `json:"title"`
	AllDay   bool      `json:"all_day"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	StartOn  string    `json:"start_on"`
	EndOn    string    `json:"end_on"`
	Location string    `json:"location"`
	Notes    string    `json:"notes"`
	Color    string    `json:"color"`
}

// EventIDRequest 按标识删除日程。
type EventIDRequest struct {
	ID int64 `json:"id,string"`
}

// EventSearchRequest 查询与可见范围相交的日程。
// From、To 是定时日程的半开区间。StartOn、EndOn 是全天日程的闭区间日期。
type EventSearchRequest struct {
	From    time.Time `json:"from"`
	To      time.Time `json:"to"`
	StartOn string    `json:"start_on"`
	EndOn   string    `json:"end_on"`
}

// EventResponse 是返回给页面的日程。
type EventResponse struct {
	ID        int64     `json:"id,string"`
	Title     string    `json:"title"`
	AllDay    bool      `json:"all_day"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	StartOn   string    `json:"start_on,omitempty"`
	EndOn     string    `json:"end_on,omitempty"`
	Location  string    `json:"location"`
	Notes     string    `json:"notes"`
	Color     string    `json:"color"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EventList 是一次范围查询的结果。
type EventList struct {
	Events []EventResponse `json:"events"`
}
