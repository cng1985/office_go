// Package database 负责四库连接、连接池和关闭，不承载业务查询。
package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	sqlite "github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"yunjian/internal/platform/config"
)

// Open 按配置打开数据库。未知驱动或连接失败直接返回错误，不回退到 SQLite。
func Open(cfg *config.Config) (*gorm.DB, error) {
	if cfg == nil || !cfg.Database.Enabled {
		return nil, fmt.Errorf("数据库未启用")
	}
	dialector, err := dialector(cfg)
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
		Logger:  logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}
	return db, nil
}

func dialector(cfg *config.Config) (gorm.Dialector, error) {
	switch NormalizeDriver(cfg.Database.Driver) {
	case "sqlite":
		if err := os.MkdirAll(filepath.Dir(cfg.Database.SQLite.Path), 0o750); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
		return sqlite.Open(sqliteDSN(cfg)), nil
	case "mysql":
		return mysql.Open(cfg.Database.DSN), nil
	case "postgres":
		return postgres.Open(cfg.Database.DSN), nil
	case "sqlserver":
		return sqlserver.Open(cfg.Database.DSN), nil
	default:
		return nil, fmt.Errorf("未知 database.driver %s", cfg.Database.Driver)
	}
}

func sqliteDSN(cfg *config.Config) string {
	fk := 0
	if cfg.Database.SQLite.ForeignKeys {
		fk = 1
	}
	return fmt.Sprintf("%s?_pragma=foreign_keys(%d)&_pragma=busy_timeout(%d)&_pragma=journal_mode(%s)",
		cfg.Database.SQLite.Path,
		fk,
		cfg.Database.SQLite.BusyTimeout.Milliseconds(),
		strings.ToUpper(cfg.Database.SQLite.JournalMode),
	)
}

// NormalizeDriver 把 pgsql 规范为 postgres，其余驱动保持原名。
func NormalizeDriver(driver string) string {
	if driver == "pgsql" {
		return "postgres"
	}
	return driver
}

// Ping 用于就绪检查。
func Ping(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("数据库不存在")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
