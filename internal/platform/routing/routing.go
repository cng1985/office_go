// Package routing 定义公开路由和认证路由的注册契约。
package routing

import "github.com/gin-gonic/gin"

// Registrar 由业务 Controller 实现。server 只依赖该契约。
type Registrar interface {
	RegisterPublic(group *gin.RouterGroup)
	RegisterProtected(group *gin.RouterGroup)
}
