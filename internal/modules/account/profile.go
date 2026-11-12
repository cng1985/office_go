package account

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"yunjian/internal/platform/auth"
	apperrors "yunjian/internal/platform/errors"
	"yunjian/internal/platform/response"
)

func (r *Repository) UpdateProfile(ctx context.Context, displayName string) (response.Response[AccountResponse], error) {
	var result response.Response[AccountResponse]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", scope.ActorID).Updates(map[string]any{
		"display_name": displayName,
		"updated_at":   now,
	})
	if res.Error != nil {
		return result, apperrors.WrapInternal(res.Error)
	}
	if res.RowsAffected != 1 {
		return result, apperrors.Unauthorized(401, "请先登录")
	}
	var user User
	if err := r.db.WithContext(ctx).Where("id = ?", scope.ActorID).Take(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return result, apperrors.Unauthorized(401, "请先登录")
		}
		return result, apperrors.WrapInternal(err)
	}
	return response.OK(*UserEntityToResponse(&user)), nil
}

func (r *Repository) ChangePassword(ctx context.Context, current, next string) (response.Response[AccountResponse], error) {
	var result response.Response[AccountResponse]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	var user User
	err = r.db.WithContext(ctx).Where("id = ?", scope.ActorID).Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, apperrors.Unauthorized(401, "请先登录")
	}
	if err != nil {
		return result, apperrors.WrapInternal(err)
	}
	if !auth.CheckPassword(user.PasswordHash, current) {
		return result, apperrors.Unauthorized(401, "当前密码不正确")
	}
	hash, err := auth.HashPassword(next)
	if err != nil {
		return result, apperrors.WrapInternal(err)
	}
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", user.ID).Updates(map[string]any{
		"password_hash": hash,
		"updated_at":    now,
	})
	if res.Error != nil {
		return result, apperrors.WrapInternal(res.Error)
	}
	if res.RowsAffected != 1 {
		return result, apperrors.Unauthorized(401, "请先登录")
	}
	user.PasswordHash = ""
	return response.OK(*UserEntityToResponse(&user)), nil
}
