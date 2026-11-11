package auth

import (
	"context"

	"yunjian/internal/platform/domain"
)

// Authenticator 由账号模块实现，中间件只依赖这个窄接口。
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (domain.Scope, error)
}
