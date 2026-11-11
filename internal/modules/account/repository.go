package account

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"yunjian/internal/platform/auth"
	"yunjian/internal/platform/domain"
	apperrors "yunjian/internal/platform/errors"
	"yunjian/internal/platform/response"
)

// Repository 保存用户和会话。
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Register(ctx context.Context, request *AccountRegisterRequest) (response.Response[AccountSessionResponse], error) {
	var result response.Response[AccountSessionResponse]
	if request == nil || request.PasswordHash == "" || request.ExpiresAt.IsZero() {
		return result, apperrors.BadRequest(400, "注册信息不完整")
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		user := &User{
			Username:     request.Username,
			DisplayName:  request.DisplayName,
			PasswordHash: request.PasswordHash,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := tx.Omit(clause.Associations).Create(user).Error; err != nil {
			return mapWriteError(err)
		}
		sessionResult, err := insertSession(tx, user, request.ExpiresAt)
		if err != nil {
			return err
		}
		result = sessionResult
		return nil
	})
	if err != nil {
		return response.Response[AccountSessionResponse]{}, err
	}
	return result, nil
}

func (r *Repository) OpenSession(ctx context.Context, request *AccountSessionRequest) (response.Response[AccountSessionResponse], error) {
	var result response.Response[AccountSessionResponse]
	if request == nil || request.UserID <= 0 || request.ExpiresAt.IsZero() {
		return result, apperrors.BadRequest(400, "会话参数无效")
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user User
		err := tx.Where("id = ?", request.UserID).Take(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.Unauthorized(401, "用户名或密码错误")
		}
		if err != nil {
			return apperrors.WrapInternal(err)
		}
		sessionResult, err := insertSession(tx, &user, request.ExpiresAt)
		if err != nil {
			return err
		}
		result = sessionResult
		return nil
	})
	if err != nil {
		return response.Response[AccountSessionResponse]{}, err
	}
	return result, nil
}

func insertSession(tx *gorm.DB, user *User, expiresAt time.Time) (response.Response[AccountSessionResponse], error) {
	var result response.Response[AccountSessionResponse]
	token, err := auth.NewToken()
	if err != nil {
		return result, apperrors.WrapInternal(err)
	}
	session := &Session{
		UserID:    user.ID,
		TokenHash: auth.HashToken(token),
		ExpiresAt: expiresAt.UTC(),
		CreatedAt: time.Now().UTC(),
	}
	if err := tx.Omit(clause.Associations).Create(session).Error; err != nil {
		return result, mapWriteError(err)
	}
	result = response.OK(AccountSessionResponse{
		Account:   *UserEntityToResponse(user),
		ExpiresAt: session.ExpiresAt,
		Token:     token,
	})
	return result, nil
}

func (r *Repository) Logout(ctx context.Context, _ *AccountLogoutRequest) (response.Response[AccountResponse], error) {
	var result response.Response[AccountResponse]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user User
		err := tx.Where("id = ?", scope.ActorID).Take(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.Unauthorized(401, "请先登录")
		}
		if err != nil {
			return apperrors.WrapInternal(err)
		}
		now := time.Now().UTC()
		res := tx.Model(&Session{}).
			Where("id = ? AND user_id = ? AND revoked_at IS NULL", scope.SessionID, scope.ActorID).
			Update("revoked_at", now)
		if res.Error != nil {
			return apperrors.WrapInternal(res.Error)
		}
		result = response.OK(*UserEntityToResponse(&user))
		return nil
	})
	if err != nil {
		return response.Response[AccountResponse]{}, err
	}
	return result, nil
}

func (r *Repository) Me(ctx context.Context, _ *AccountMeRequest) (response.Response[AccountResponse], error) {
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
	return response.OK(*UserEntityToResponse(&user)), nil
}

func (r *Repository) findByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("username = ?", username).Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return &user, nil
}

// Authenticate 校验原始会话令牌。它实现平台认证接口，不是业务 Response 契约。
func (r *Repository) Authenticate(ctx context.Context, token string) (domain.Scope, error) {
	if strings.TrimSpace(token) == "" {
		return domain.Scope{}, apperrors.Unauthorized(401, "请先登录")
	}
	var session Session
	err := r.db.WithContext(ctx).Where("token_hash = ?", auth.HashToken(token)).Take(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Scope{}, apperrors.Unauthorized(401, "登录已失效，请重新登录")
	}
	if err != nil {
		return domain.Scope{}, apperrors.WrapInternal(err)
	}
	if session.RevokedAt != nil || !session.ExpiresAt.After(time.Now().UTC()) {
		return domain.Scope{}, apperrors.Unauthorized(401, "登录已失效，请重新登录")
	}
	return domain.Scope{ActorID: session.UserID, SessionID: session.ID}, nil
}

func requireScope(ctx context.Context) (domain.Scope, error) {
	scope, ok := domain.ScopeFrom(ctx)
	if !ok || scope.ActorID <= 0 || scope.SessionID <= 0 {
		return domain.Scope{}, apperrors.Unauthorized(401, "请先登录")
	}
	return scope, nil
}

func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "unique") || strings.Contains(text, "duplicate") {
		if strings.Contains(text, "username") {
			return apperrors.Conflict(10001, "用户名已存在")
		}
		return apperrors.Conflict(409, "数据冲突，请刷新后重试")
	}
	return apperrors.WrapInternal(err)
}
