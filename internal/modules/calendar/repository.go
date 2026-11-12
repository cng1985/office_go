package calendar

import (
	"context"
	"time"

	"gorm.io/gorm"

	apperrors "yunjian/internal/platform/errors"
)

const maxSearchRows = 400

// Repository 读写当前用户的日程。
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, event *Event) error {
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, ownerID int64, event *Event) error {
	result := r.db.WithContext(ctx).Model(&Event{}).
		Where("id = ? AND owner_id = ?", event.ID, ownerID).
		Updates(map[string]any{
			"title":      event.Title,
			"all_day":    event.AllDay,
			"starts_at":  event.StartsAt,
			"ends_at":    event.EndsAt,
			"start_on":   event.StartOn,
			"end_on":     event.EndOn,
			"location":   event.Location,
			"notes":      event.Notes,
			"color":      event.Color,
			"updated_at": event.UpdatedAt,
		})
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.NotFound(404, "日程不存在")
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, ownerID, id int64) error {
	result := r.db.WithContext(ctx).Where("id = ? AND owner_id = ?", id, ownerID).Delete(&Event{})
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.NotFound(404, "日程不存在")
	}
	return nil
}

// SearchQuery 是已经校验过的查询窗口。
type SearchQuery struct {
	From    time.Time
	To      time.Time
	StartOn string
	EndOn   string
}

func (r *Repository) Search(ctx context.Context, ownerID int64, query SearchQuery) ([]Event, error) {
	var rows []Event
	err := r.db.WithContext(ctx).
		Where("owner_id = ? AND ((all_day = ? AND starts_at < ? AND ends_at > ?) OR (all_day = ? AND start_on <= ? AND end_on >= ?))",
			ownerID, false, query.To, query.From, true, query.EndOn, query.StartOn).
		Order("all_day DESC, start_on ASC, starts_at ASC, id ASC").
		Limit(maxSearchRows + 1).
		Find(&rows).Error
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	if len(rows) > maxSearchRows {
		return nil, apperrors.BadRequest(400, "这个范围内的日程过多")
	}
	if rows == nil {
		rows = []Event{}
	}
	return rows, nil
}
