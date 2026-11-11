package account

import (
	"context"
	"time"

	"yunjian/internal/platform/auth"
	"yunjian/internal/platform/config"
	"yunjian/internal/platform/domain"
	apperrors "yunjian/internal/platform/errors"
	"yunjian/internal/platform/response"
)

// Resource 编排注册、登录、退出和当前用户。
type Resource struct {
	repo       *Repository
	sessionTTL time.Duration
	loginLimit *Limiter
	dummyHash  string
}

func NewResource(repo *Repository, cfg *config.Config) (*Resource, error) {
	dummy, err := auth.HashPassword("yunjian-dummy-password")
	if err != nil {
		return nil, err
	}
	return &Resource{
		repo:       repo,
		sessionTTL: cfg.Auth.SessionTTL,
		loginLimit: NewLimiter(20, 10*time.Minute, 10000),
		dummyHash:  dummy,
	}, nil
}

func (r *Resource) Register(ctx context.Context, request *AccountRegisterRequest) (response.Response[AccountSessionResponse], error) {
	var zero response.Response[AccountSessionResponse]
	if err := normalizeRegister(request); err != nil {
		return zero, err
	}
	if !r.allow("register", domain.ClientIP(ctx)) {
		return zero, apperrors.RateLimited(429, "尝试次数过多，请稍后再试")
	}
	hash, err := auth.HashPassword(request.Password)
	if err != nil {
		return zero, apperrors.WrapInternal(err)
	}
	request.Password = ""
	request.PasswordHash = hash
	request.ExpiresAt = time.Now().UTC().Add(r.sessionTTL)
	return r.repo.Register(ctx, request)
}

func (r *Resource) Login(ctx context.Context, request *AccountLoginRequest) (response.Response[AccountSessionResponse], error) {
	var zero response.Response[AccountSessionResponse]
	if err := normalizeLogin(request); err != nil {
		return zero, err
	}
	if !r.allow("login", domain.ClientIP(ctx)) {
		return zero, apperrors.RateLimited(429, "尝试次数过多，请稍后再试")
	}
	user, err := r.repo.findByUsername(ctx, request.Username)
	if err != nil {
		return zero, err
	}
	hash := r.dummyHash
	if user != nil {
		hash = user.PasswordHash
	}
	if !auth.CheckPassword(hash, request.Password) || user == nil {
		return zero, apperrors.Unauthorized(401, "用户名或密码错误")
	}
	request.Password = ""
	return r.repo.OpenSession(ctx, &AccountSessionRequest{
		UserID:    user.ID,
		ExpiresAt: time.Now().UTC().Add(r.sessionTTL),
	})
}

func (r *Resource) Logout(ctx context.Context, request *AccountLogoutRequest) (response.Response[AccountResponse], error) {
	if request == nil {
		request = &AccountLogoutRequest{}
	}
	return r.repo.Logout(ctx, request)
}

func (r *Resource) Me(ctx context.Context, request *AccountMeRequest) (response.Response[AccountResponse], error) {
	if request == nil {
		request = &AccountMeRequest{}
	}
	return r.repo.Me(ctx, request)
}

func (r *Resource) allow(action, ip string) bool {
	if ip == "" {
		ip = "unknown"
	}
	return r.loginLimit.Allow(action + ":" + ip)
}
