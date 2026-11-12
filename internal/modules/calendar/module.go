package calendar

import (
	"go.uber.org/fx"

	"yunjian/internal/platform/routing"
)

// Module 提供个人日历的仓储、规则和接口。
var Module = fx.Module(
	"calendar",
	fx.Provide(NewRepository),
	fx.Provide(NewResource),
	fx.Provide(NewController),
	fx.Provide(provideRegistrar),
)

type registrarOut struct {
	fx.Out
	Registrar routing.Registrar `group:"registrars"`
}

func provideRegistrar(controller *Controller) registrarOut {
	return registrarOut{Registrar: controller}
}
