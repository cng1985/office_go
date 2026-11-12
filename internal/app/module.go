package app

import (
	"go.uber.org/fx"

	"yunjian/internal/modules/account"
	"yunjian/internal/modules/calendar"
	"yunjian/internal/modules/document"
	"yunjian/internal/platform/database"
	"yunjian/internal/platform/middleware"
	"yunjian/internal/platform/server"
	"yunjian/web"
)

// Module 组合平台、业务和前端静态资源。
var Module = fx.Options(
	database.Module,
	middleware.Module,
	server.Module,
	account.Module,
	document.Module,
	calendar.Module,
	web.Module,
	fx.Invoke(bootstrap),
)
