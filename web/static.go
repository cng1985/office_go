package web

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	apperrors "yunjian/internal/platform/errors"
	"yunjian/internal/platform/response"
)

// RegisterStatic 注册静态资源和单页回退，不启动 HTTP 服务。
func RegisterStatic(engine *gin.Engine) error {
	dist, err := fs.Sub(Dist, "dist")
	if err != nil {
		return err
	}
	engine.NoRoute(func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if requestPath == "/api" || strings.HasPrefix(requestPath, "/api/") || requestPath == "/health" || strings.HasPrefix(requestPath, "/health/") {
			response.HandleError(c, apperrors.NotFound(404, "接口不存在"))
			return
		}
		if requestPath != "/" && path.Ext(requestPath) != "" {
			if serveFile(c, dist, strings.TrimPrefix(requestPath, "/")) {
				return
			}
			c.Status(http.StatusNotFound)
			return
		}
		data, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
	return nil
}

func serveFile(c *gin.Context, dist fs.FS, name string) bool {
	file, err := dist.Open(name)
	if err != nil {
		return false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return false
	}
	data, err := io.ReadAll(file)
	if err != nil {
		c.Status(http.StatusNotFound)
		return true
	}
	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Data(http.StatusOK, contentType, data)
	return true
}
