package database

import (
	"context"
	"path/filepath"
	"time"

	"gorm.io/gorm"

	"yunjian/internal/platform/config"
)

// OpenAndMigrateSQLite 打开一个独立的 SQLite 文件并执行迁移，供测试和本地检查使用。
func OpenAndMigrateSQLite(ctx context.Context, path string) (*gorm.DB, error) {
	cfg := &config.Config{}
	cfg.Database.Enabled = true
	cfg.Database.Driver = "sqlite"
	cfg.Database.MaxOpenConns = 1
	cfg.Database.MaxIdleConns = 1
	cfg.Database.ConnMaxLifetime = time.Minute
	cfg.Database.BatchSize = 100
	cfg.Database.SQLite.Path = path
	cfg.Database.SQLite.ForeignKeys = true
	cfg.Database.SQLite.BusyTimeout = 5 * time.Second
	cfg.Database.SQLite.JournalMode = "DELETE"
	if filepath.IsAbs(path) {
		cfg.App.DataDir = filepath.Dir(path)
	}
	db, err := Open(cfg)
	if err != nil {
		return nil, err
	}
	if err := Migrate(ctx, db, "sqlite"); err != nil {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}
	return db, nil
}
