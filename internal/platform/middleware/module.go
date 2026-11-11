package middleware

import "go.uber.org/fx"

var Module = fx.Module(
	"middleware",
	fx.Provide(NewGlobal),
	fx.Provide(NewAuth),
)
