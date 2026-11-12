package calendar

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"yunjian/internal/platform/domain"
	apperrors "yunjian/internal/platform/errors"
	"yunjian/internal/platform/response"
)

const (
	maxTitle     = 80
	maxLocation  = 80
	maxNotes     = 2000
	maxAllDay    = 31
	maxTimed     = 14 * 24 * time.Hour
	maxSearch    = 70 * 24 * time.Hour
	defaultColor = "#1a73e8"
)

var eventColors = map[string]struct{}{
	"#1a73e8": {},
	"#188038": {},
	"#f9ab00": {},
	"#d93025": {},
	"#a142f4": {},
	"#12b5cb": {},
	"#e37400": {},
	"#5f6368": {},
}

// Resource 校验日程规则，并只操作当前登录用户的数据。
type Resource struct {
	repo *Repository
}

func NewResource(repo *Repository) *Resource {
	return &Resource{repo: repo}
}

func (r *Resource) Create(ctx context.Context, request *EventWriteRequest) (response.Response[EventResponse], error) {
	var zero response.Response[EventResponse]
	ownerID, err := ownerID(ctx)
	if err != nil {
		return zero, err
	}
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if request.ID != 0 {
		return zero, apperrors.BadRequest(400, "创建日程不能指定标识")
	}
	event, err := normalize(request)
	if err != nil {
		return zero, err
	}
	now := time.Now().UTC()
	event.OwnerID = ownerID
	event.CreatedAt = now
	event.UpdatedAt = now
	if err := r.repo.Create(ctx, &event); err != nil {
		return zero, err
	}
	return response.OK(toResponse(event)), nil
}

func (r *Resource) Update(ctx context.Context, request *EventWriteRequest) (response.Response[EventResponse], error) {
	var zero response.Response[EventResponse]
	ownerID, err := ownerID(ctx)
	if err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	event, err := normalize(request)
	if err != nil {
		return zero, err
	}
	event.ID = request.ID
	event.OwnerID = ownerID
	event.UpdatedAt = time.Now().UTC()
	if err := r.repo.Update(ctx, ownerID, &event); err != nil {
		return zero, err
	}
	return response.OK(toResponse(event)), nil
}

func (r *Resource) Delete(ctx context.Context, request *EventIDRequest) (response.Response[EventResponse], error) {
	var zero response.Response[EventResponse]
	ownerID, err := ownerID(ctx)
	if err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := r.repo.Delete(ctx, ownerID, request.ID); err != nil {
		return zero, err
	}
	return response.OK(EventResponse{ID: request.ID}), nil
}

func (r *Resource) Search(ctx context.Context, request *EventSearchRequest) (response.Response[EventList], error) {
	var zero response.Response[EventList]
	ownerID, err := ownerID(ctx)
	if err != nil {
		return zero, err
	}
	query, err := normalizeSearch(request)
	if err != nil {
		return zero, err
	}
	rows, err := r.repo.Search(ctx, ownerID, query)
	if err != nil {
		return zero, err
	}
	events := make([]EventResponse, 0, len(rows))
	for _, row := range rows {
		events = append(events, toResponse(row))
	}
	return response.OK(EventList{Events: events}), nil
}

func ownerID(ctx context.Context) (int64, error) {
	scope, ok := domain.ScopeFrom(ctx)
	if !ok || scope.ActorID <= 0 {
		return 0, apperrors.Unauthorized(401, "请先登录")
	}
	return scope.ActorID, nil
}

func normalize(request *EventWriteRequest) (Event, error) {
	title, err := cleanLine(request.Title, maxTitle, "日程标题过长")
	if err != nil {
		return Event{}, err
	}
	if title == "" {
		return Event{}, apperrors.BadRequest(400, "请填写日程标题")
	}
	location, err := cleanLine(request.Location, maxLocation, "地点过长")
	if err != nil {
		return Event{}, err
	}
	notes, err := cleanNotes(request.Notes)
	if err != nil {
		return Event{}, err
	}
	color, err := normalizeColor(request.Color)
	if err != nil {
		return Event{}, err
	}
	event := Event{Title: title, AllDay: request.AllDay, Location: location, Notes: notes, Color: color}
	if request.AllDay {
		start, end, err := dateSpan(request.StartOn, request.EndOn, maxAllDay)
		if err != nil {
			return Event{}, err
		}
		event.StartOn = start.Format("2006-01-02")
		event.EndOn = end.Format("2006-01-02")
		event.StartsAt = start
		event.EndsAt = end.AddDate(0, 0, 1)
		return event, nil
	}
	starts := request.StartsAt.UTC()
	ends := request.EndsAt.UTC()
	if starts.IsZero() || ends.IsZero() || !ends.After(starts) {
		return Event{}, apperrors.BadRequest(400, "结束时间需要晚于开始时间")
	}
	if ends.Sub(starts) > maxTimed {
		return Event{}, apperrors.BadRequest(400, "日程最长 14 天")
	}
	event.StartsAt = starts
	event.EndsAt = ends
	return event, nil
}

func normalizeSearch(request *EventSearchRequest) (SearchQuery, error) {
	if request == nil || request.From.IsZero() || request.To.IsZero() || strings.TrimSpace(request.StartOn) == "" || strings.TrimSpace(request.EndOn) == "" {
		return SearchQuery{}, apperrors.BadRequest(400, "请选择查询范围")
	}
	from := request.From.UTC()
	to := request.To.UTC()
	if !to.After(from) {
		return SearchQuery{}, apperrors.BadRequest(400, "结束时间需要晚于开始时间")
	}
	if to.Sub(from) > maxSearch {
		return SearchQuery{}, apperrors.BadRequest(400, "查询范围过长")
	}
	start, end, err := dateSpan(request.StartOn, request.EndOn, 70)
	if err != nil {
		return SearchQuery{}, err
	}
	return SearchQuery{From: from, To: to, StartOn: start.Format("2006-01-02"), EndOn: end.Format("2006-01-02")}, nil
}

func dateSpan(startOn, endOn string, maxDays int) (time.Time, time.Time, error) {
	start, err := parseDay(startOn)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := parseDay(endOn)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, apperrors.BadRequest(400, "结束日期不能早于开始日期")
	}
	if int(end.Sub(start)/(24*time.Hour))+1 > maxDays {
		if maxDays == maxAllDay {
			return time.Time{}, time.Time{}, apperrors.BadRequest(400, "全天日程最长 31 天")
		}
		return time.Time{}, time.Time{}, apperrors.BadRequest(400, "查询范围过长")
	}
	return start, end, nil
}

func parseDay(value string) (time.Time, error) {
	day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.UTC)
	if err != nil {
		return time.Time{}, apperrors.BadRequest(400, "日程日期无效")
	}
	return day, nil
}

func cleanLine(value string, limit int, tooLong string) (string, error) {
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) > limit {
		return "", apperrors.BadRequest(400, tooLong)
	}
	return value, nil
}

func cleanNotes(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\r\n", "\n"))
	if utf8.RuneCountInString(value) > maxNotes {
		return "", apperrors.BadRequest(400, "备注过长")
	}
	return value, nil
}

func normalizeColor(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return defaultColor, nil
	}
	if _, ok := eventColors[value]; !ok {
		return "", apperrors.BadRequest(400, "日程颜色无效")
	}
	return value, nil
}

func toResponse(event Event) EventResponse {
	item := EventResponse{
		ID:        event.ID,
		Title:     event.Title,
		AllDay:    event.AllDay,
		StartsAt:  event.StartsAt.UTC(),
		EndsAt:    event.EndsAt.UTC(),
		Location:  event.Location,
		Notes:     event.Notes,
		Color:     event.Color,
		UpdatedAt: event.UpdatedAt.UTC(),
	}
	if event.AllDay {
		item.StartOn = event.StartOn
		item.EndOn = event.EndOn
	}
	return item
}
