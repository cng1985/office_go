package document

import (
	"go.uber.org/fx"

	"yunjian/internal/platform/routing"
)

var Module = fx.Module(
	"document",
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
