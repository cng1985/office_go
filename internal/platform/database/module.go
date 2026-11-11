package database

import (
	"context"

	"go.uber.org/fx"
	"gorm.io/gorm"

	"yunjian/internal/platform/config"
)

var Module = fx.Module(
	"database",
	fx.Provide(New),
)

func New(cfg *config.Config, lc fx.Lifecycle) (*gorm.DB, error) {
	db, err := Open(cfg)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.Close()
		},
	})
	return db, nil
}
