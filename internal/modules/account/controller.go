package account

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"yunjian/internal/platform/auth"
	"yunjian/internal/platform/config"
	"yunjian/internal/platform/response"
)

// Controller 注册账号路由，并负责会话 Cookie。
type Controller struct {
	resource *Resource
	secure   bool
}

func NewController(resource *Resource, cfg *config.Config) *Controller {
	return &Controller{resource: resource, secure: cfg.Auth.CookieSecure}
}

func (c *Controller) RegisterPublic(group *gin.RouterGroup) {
	group.POST("/account/register", c.register)
	group.POST("/account/login", c.login)
}

func (c *Controller) RegisterProtected(group *gin.RouterGroup) {
	group.POST("/account/logout", c.logout)
	group.POST("/account/me", c.me)
	group.POST("/account/profile", c.profile)
	group.POST("/account/password", c.password)
	group.POST("/team/create", c.teamCreate)
	group.POST("/team/update", c.teamUpdate)
	group.POST("/team/delete", c.teamDelete)
	group.POST("/team/search", c.teamSearch)
	group.POST("/team/one", c.teamOne)
	group.POST("/team/member/add", c.teamMemberAdd)
	group.POST("/team/member/remove", c.teamMemberRemove)
	group.POST("/team/leave", c.teamLeave)
}

func (c *Controller) register(ctx *gin.Context) {
	var request AccountRegisterRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Register(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	c.writeSession(ctx, result.Data.Token, result.Data.ExpiresAt)
	result.Data.Token = ""
	response.SuccessResponse(ctx, result)
}

func (c *Controller) login(ctx *gin.Context) {
	var request AccountLoginRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Login(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	c.writeSession(ctx, result.Data.Token, result.Data.ExpiresAt)
	result.Data.Token = ""
	response.SuccessResponse(ctx, result)
}

func (c *Controller) logout(ctx *gin.Context) {
	result, err := c.resource.Logout(ctx.Request.Context(), &AccountLogoutRequest{})
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	c.clearSession(ctx)
	response.SuccessResponse(ctx, result)
}

func (c *Controller) me(ctx *gin.Context) {
	result, err := c.resource.Me(ctx.Request.Context(), &AccountMeRequest{})
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) writeSession(ctx *gin.Context, token string, expires time.Time) {
	maxAge := int(time.Until(expires).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     auth.SessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.secure,
		Expires:  expires,
		MaxAge:   maxAge,
	})
}

func (c *Controller) clearSession(ctx *gin.Context) {
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     auth.SessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
	})
}
