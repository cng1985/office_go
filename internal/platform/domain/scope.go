// Package domain 存放不依赖 Gin 的可信上下文和分页参数。
package domain

import "context"

// Scope 是认证通过后写入 context 的不可变身份。
// 客户端提交的用户 ID 不能当作 Scope。
type Scope struct {
	ActorID   int64
	SessionID int64
	RequestID string
}

type scopeKey struct{}
type requestIDKey struct{}
type clientIPKey struct{}

func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

func ScopeFrom(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(scopeKey{}).(Scope)
	if !ok || scope.ActorID <= 0 {
		return Scope{}, false
	}
	return scope, true
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey{}, ip)
}

func ClientIP(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)
	return ip
}
