package database

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gorm.io/gorm"
)

// BackupSQLite 使用 VACUUM INTO 生成一致性副本。目标文件不能已存在。
// 该实现只覆盖 SQLite；其他数据库应使用各自引擎的备份工具。
func BackupSQLite(ctx context.Context, db *gorm.DB, dest string) error {
	if db == nil {
		return fmt.Errorf("数据库不存在")
	}
	if strings.TrimSpace(dest) == "" {
		return fmt.Errorf("备份路径不能为空")
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("备份文件已存在")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	escaped := strings.ReplaceAll(dest, "'", "''")
	_, err = sqlDB.ExecContext(ctx, "VACUUM INTO '"+escaped+"'")
	if err != nil {
		return fmt.Errorf("备份 SQLite 失败: %w", err)
	}
	return nil
}
