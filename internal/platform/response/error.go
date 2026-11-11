package response

import (
	stderrors "errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	apperrors "yunjian/internal/platform/errors"
)

const (
	ContextAppCode = "app_code"
	ContextErrKind = "err_kind"
	ContextError   = "handler_error"
)

// HandleError 把错误映射为统一 JSON。应用错误的 HTTP 状态保持 200。
// 原始原因放进 Gin 上下文，由请求日志边界记录一次，避免下层重复打印。
func HandleError(c *gin.Context, err error) {
	if err == nil || c == nil {
		return
	}
	if c.Request != nil && c.Request.Context().Err() != nil && c.Writer.Written() {
		return
	}
	code, msg, kind := apperrors.PublicMessage(err)
	c.Set(ContextAppCode, code)
	c.Set(ContextErrKind, string(kind))
	c.Set(ContextError, err)
	c.Abort()
	c.JSON(http.StatusOK, Response[any]{Code: code, Msg: msg})
}

// BindJSON 绑定 JSON，并把超限和格式问题映射为分类错误。
func BindJSON(c *gin.Context, dst any) error {
	if err := c.ShouldBindJSON(dst); err != nil {
		var maxBytes *http.MaxBytesError
		if stderrors.As(err, &maxBytes) || strings.Contains(err.Error(), "request body too large") {
			return apperrors.TooLarge(413, "请求体过大")
		}
		slog.Debug("bind json failed", "err", err.Error())
		return apperrors.BadRequest(400, "请求格式不正确")
	}
	return nil
}
