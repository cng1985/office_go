package account

import (
	"go.uber.org/fx"

	"yunjian/internal/platform/auth"
	"yunjian/internal/platform/routing"
)

var Module = fx.Module(
	"account",
	fx.Provide(NewRepository),
	fx.Provide(NewResource),
	fx.Provide(NewController),
	fx.Provide(func(repo *Repository) auth.Authenticator { return repo }),
	fx.Provide(provideRegistrar),
)

type registrarOut struct {
	fx.Out
	Registrar routing.Registrar `group:"registrars"`
}

func provideRegistrar(controller *Controller) registrarOut {
	return registrarOut{Registrar: controller}
}
