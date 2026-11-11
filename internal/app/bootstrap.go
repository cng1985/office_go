package app

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"yunjian/internal/platform/config"
	"yunjian/internal/platform/database"
	"yunjian/internal/platform/server"
)

// Run 在 HTTP 开始监听前执行迁移，并标记服务就绪。
func bootstrap(cfg *config.Config, db *gorm.DB, ready *server.Readiness, logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		return err
	}
	ready.MarkReady()
	logger.Info("application ready",
		"profile", cfg.App.Profile,
		"base_dir", cfg.App.BaseDir,
		"config_dir", cfg.App.ConfigDir,
		"data_dir", cfg.App.DataDir,
		"driver", cfg.Database.Driver,
		"sqlite", cfg.SQLiteFile(),
		"listen", cfg.ListenAddr(),
	)
	return nil
}
