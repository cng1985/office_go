// Package errors 定义不依赖 Gin、GORM 和具体业务的错误分类。
package errors

import stderrors "errors"

// Kind 是错误分类，用于日志和调用方判断，不是给客户端的文案。
type Kind string

const (
	KindBadRequest   Kind = "bad_request"
	KindUnauthorized Kind = "unauthorized"
	KindForbidden    Kind = "forbidden"
	KindNotFound     Kind = "not_found"
	KindConflict     Kind = "conflict"
	KindTooLarge     Kind = "too_large"
	KindRateLimit    Kind = "rate_limit"
	KindInternal     Kind = "internal"
	KindUnavailable  Kind = "unavailable"
)

// Error 是应用错误。Message 可对客户端公开；Cause 只进入服务端日志。
type Error struct {
	Kind    Kind
	Code    int
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

// PublicMessage 返回可公开的应用码和文案。未知错误隐藏内部细节。
func PublicMessage(err error) (int, string, Kind) {
	var appErr *Error
	if stderrors.As(err, &appErr) {
		msg := appErr.Message
		if msg == "" {
			msg = "服务暂时不可用"
		}
		return appErr.Code, msg, appErr.Kind
	}
	return 500, "服务暂时不可用", KindInternal
}

func BadRequest(code int, message string) *Error {
	return &Error{Kind: KindBadRequest, Code: code, Message: message}
}

func Unauthorized(code int, message string) *Error {
	return &Error{Kind: KindUnauthorized, Code: code, Message: message}
}

func Forbidden(code int, message string) *Error {
	return &Error{Kind: KindForbidden, Code: code, Message: message}
}

func NotFound(code int, message string) *Error {
	return &Error{Kind: KindNotFound, Code: code, Message: message}
}

func Conflict(code int, message string) *Error {
	return &Error{Kind: KindConflict, Code: code, Message: message}
}

func TooLarge(code int, message string) *Error {
	return &Error{Kind: KindTooLarge, Code: code, Message: message}
}

func RateLimited(code int, message string) *Error {
	return &Error{Kind: KindRateLimit, Code: code, Message: message}
}

func Unavailable(message string, cause error) *Error {
	if message == "" {
		message = "服务暂时不可用"
	}
	return &Error{Kind: KindUnavailable, Code: 503, Message: message, Cause: cause}
}

func WrapInternal(cause error) *Error {
	return &Error{Kind: KindInternal, Code: 500, Message: "服务暂时不可用", Cause: cause}
}
