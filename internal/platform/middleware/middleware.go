// Package middleware 提供 Gin 的请求标识、安全头、恢复、超时和认证拦截。
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"yunjian/internal/platform/auth"
	"yunjian/internal/platform/config"
	"yunjian/internal/platform/domain"
	apperrors "yunjian/internal/platform/errors"
	"yunjian/internal/platform/response"
)

type GlobalOut struct {
	fx.Out
	Handler gin.HandlerFunc `name:"global"`
}

type AuthOut struct {
	fx.Out
	Handler gin.HandlerFunc `name:"auth"`
}

func NewGlobal(cfg *config.Config, logger *slog.Logger) GlobalOut {
	return GlobalOut{Handler: func(c *gin.Context) {
		requestID := newRequestID()
		c.Header("X-Request-ID", requestID)
		c.Request = c.Request.WithContext(domain.WithRequestID(c.Request.Context(), requestID))
		ip := clientIP(c, len(cfg.Server.TrustedProxies) > 0)
		c.Request = c.Request.WithContext(domain.WithClientIP(c.Request.Context(), ip))
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Permitted-Cross-Domain-Policies", "none")
		c.Header("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'")
		start := time.Now()
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic recovered",
					"request_id", requestID,
					"panic", fmt.Sprint(rec),
					"stack", string(debug.Stack()),
				)
				if !c.Writer.Written() {
					response.HandleError(c, apperrors.WrapInternal(fmt.Errorf("panic")))
				}
				c.Abort()
			}
		}()
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.Server.MaxBodyBytes)
			if err := checkOrigin(c); err != nil {
				response.HandleError(c, err)
				logRequest(logger, c, requestID, start)
				return
			}
			ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.Server.RequestTimeout)
			defer cancel()
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
		logRequest(logger, c, requestID, start)
	}}
}

func NewAuth(authenticator auth.Authenticator) AuthOut {
	return AuthOut{Handler: func(c *gin.Context) {
		token := sessionToken(c)
		if token == "" {
			response.HandleError(c, apperrors.Unauthorized(401, "请先登录"))
			return
		}
		scope, err := authenticator.Authenticate(c.Request.Context(), token)
		if err != nil {
			response.HandleError(c, err)
			return
		}
		scope.RequestID = domain.RequestID(c.Request.Context())
		ctx := domain.WithScope(c.Request.Context(), scope)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}}
}

func sessionToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	cookie, err := c.Cookie(auth.SessionCookie)
	if err != nil {
		return ""
	}
	return cookie
}

func checkOrigin(c *gin.Context) error {
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
		return nil
	}
	origin := c.GetHeader("Origin")
	if origin == "" {
		return nil
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return apperrors.Forbidden(403, "来源不被允许")
	}
	if !strings.EqualFold(parsed.Host, c.Request.Host) {
		return apperrors.Forbidden(403, "来源不被允许")
	}
	return nil
}

func clientIP(c *gin.Context, trusted bool) string {
	if trusted {
		return c.ClientIP()
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}

func logRequest(logger *slog.Logger, c *gin.Context, requestID string, start time.Time) {
	appCode := 200
	if value, ok := c.Get(response.ContextAppCode); ok {
		if code, ok := value.(int); ok {
			appCode = code
		}
	}
	var actorID int64
	if scope, ok := domain.ScopeFrom(c.Request.Context()); ok {
		actorID = scope.ActorID
	}
	route := c.FullPath()
	if route == "" {
		route = c.Request.URL.Path
	}
	args := []any{
		"request_id", requestID,
		"actor_id", actorID,
		"method", c.Request.Method,
		"route", route,
		"status", c.Writer.Status(),
		"app_code", appCode,
		"duration_ms", time.Since(start).Milliseconds(),
	}
	if kind, ok := c.Get(response.ContextErrKind); ok {
		args = append(args, "err_kind", kind)
	}
	if appCode >= 500 {
		if errValue, ok := c.Get(response.ContextError); ok {
			logger.Error("request", append(args, "err", fmt.Sprint(errValue))...)
			return
		}
	}
	logger.Info("request", args...)
}

func newRequestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format("150405.000000")))
	}
	return hex.EncodeToString(buf)
}
