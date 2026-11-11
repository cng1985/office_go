package server

import "go.uber.org/fx"

var Module = fx.Module(
	"server",
	fx.Provide(NewReadiness),
	fx.Provide(NewEngine),
	fx.Invoke(Start),
)
