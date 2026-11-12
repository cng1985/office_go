package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"yunjian/migrations"
)

// migrationVersions 按顺序执行。已记录的版本会跳过，方便已有数据库补上后续变更。
var migrationVersions = []string{"0001_init", "0002_folder", "0003_team", "0004_calendar"}

type migrationStep struct {
	version string
	script  string
}

// Migrate 执行尚未记录的迁移。重复调用不会重放已完成版本。
func Migrate(ctx context.Context, db *gorm.DB, driver string) error {
	normalized := NormalizeDriver(driver)
	steps, err := loadMigrations(normalized)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return migrateOnConn(ctx, conn, normalized, steps)
}

func loadMigrations(driver string) ([]migrationStep, error) {
	steps := make([]migrationStep, 0, len(migrationVersions))
	for _, version := range migrationVersions {
		name := version + "." + driver + ".sql"
		payload, err := migrations.Files.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("缺少迁移脚本 %s", name)
		}
		steps = append(steps, migrationStep{version: version, script: string(payload)})
	}
	return steps, nil
}

func migrateOnConn(ctx context.Context, conn *sql.Conn, driver string, steps []migrationStep) error {
	switch driver {
	case "sqlite":
		return migrateSQLite(ctx, conn, steps)
	case "mysql":
		return migrateLocked(ctx, conn, steps, driver, lockMySQL, unlockMySQL)
	case "postgres":
		return migrateLocked(ctx, conn, steps, driver, lockPostgres, unlockPostgres)
	case "sqlserver":
		return migrateLocked(ctx, conn, steps, driver, lockSQLServer, unlockSQLServer)
	default:
		return fmt.Errorf("未知 database.driver %s", driver)
	}
}

func migrateSQLite(ctx context.Context, conn *sql.Conn, steps []migrationStep) error {
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("开始迁移事务失败: %w", err)
	}
	if err := applyPending(ctx, conn, "sqlite", steps); err != nil {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		return fmt.Errorf("提交迁移失败: %w", err)
	}
	return nil
}

type lockFunc func(context.Context, *sql.Conn) error

func migrateLocked(ctx context.Context, conn *sql.Conn, steps []migrationStep, driver string, lock, unlock lockFunc) error {
	if err := lock(ctx, conn); err != nil {
		return err
	}
	defer unlock(context.Background(), conn)
	if driver == "mysql" {
		return applyPending(ctx, conn, driver, steps)
	}
	begin := "BEGIN"
	if driver == "sqlserver" {
		begin = "BEGIN TRANSACTION"
	}
	if _, err := conn.ExecContext(ctx, begin); err != nil {
		return err
	}
	if err := applyPending(ctx, conn, driver, steps); err != nil {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		return err
	}
	return nil
}

func applyPending(ctx context.Context, conn *sql.Conn, driver string, steps []migrationStep) error {
	if _, err := conn.ExecContext(ctx, schemaTableSQL(driver)); err != nil {
		return fmt.Errorf("创建迁移记录表失败: %w", err)
	}
	query := "SELECT version FROM schema_migrations WHERE version = " + placeholder(driver, 1)
	insert := "INSERT INTO schema_migrations (version, applied_at) VALUES (" + placeholder(driver, 1) + ", " + placeholder(driver, 2) + ")"
	for _, step := range steps {
		var version string
		err := conn.QueryRowContext(ctx, query, step.version).Scan(&version)
		if err == nil {
			continue
		}
		if err != sql.ErrNoRows {
			return err
		}
		for _, stmt := range splitSQL(step.script) {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("执行迁移 %s 失败: %w", step.version, err)
			}
		}
		if _, err := conn.ExecContext(ctx, insert, step.version, time.Now().UTC()); err != nil {
			return fmt.Errorf("记录迁移版本失败: %w", err)
		}
	}
	return nil
}

func placeholder(driver string, index int) string {
	switch driver {
	case "postgres":
		return fmt.Sprintf("$%d", index)
	case "sqlserver":
		return fmt.Sprintf("@p%d", index)
	default:
		return "?"
	}
}

func schemaTableSQL(driver string) string {
	switch driver {
	case "mysql":
		return `CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(64) NOT NULL PRIMARY KEY,
			applied_at DATETIME(3) NOT NULL
		)`
	case "postgres":
		return `CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(64) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL
		)`
	case "sqlserver":
		return `IF OBJECT_ID(N'schema_migrations', N'U') IS NULL
			CREATE TABLE schema_migrations (
				version NVARCHAR(64) NOT NULL PRIMARY KEY,
				applied_at DATETIMEOFFSET NOT NULL
			)`
	default:
		return `CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL
		)`
	}
}

func splitSQL(script string) []string {
	parts := strings.Split(script, ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		stmt := strings.TrimSpace(part)
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}
	return statements
}

func lockMySQL(ctx context.Context, conn *sql.Conn) error {
	var locked sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK('yunjian_migrate', 30)").Scan(&locked); err != nil {
		return fmt.Errorf("获取迁移锁失败: %w", err)
	}
	if !locked.Valid || locked.Int64 != 1 {
		return fmt.Errorf("获取迁移锁失败")
	}
	return nil
}

func unlockMySQL(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, "SELECT RELEASE_LOCK('yunjian_migrate')")
	return err
}

func lockPostgres(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock(86421001)")
	if err != nil {
		return fmt.Errorf("获取迁移锁失败: %w", err)
	}
	return nil
}

func unlockPostgres(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, "SELECT pg_advisory_unlock(86421001)")
	return err
}

func lockSQLServer(ctx context.Context, conn *sql.Conn) error {
	var result int
	err := conn.QueryRowContext(ctx, "DECLARE @result int; EXEC @result = sp_getapplock @Resource = N'yunjian_migrate', @LockMode = 'Exclusive', @LockOwner = 'Session', @LockTimeout = 30000; SELECT @result").Scan(&result)
	if err != nil {
		return fmt.Errorf("获取迁移锁失败: %w", err)
	}
	if result < 0 {
		return fmt.Errorf("获取迁移锁失败")
	}
	return nil
}

func unlockSQLServer(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, "EXEC sp_releaseapplock @Resource = N'yunjian_migrate', @LockOwner = 'Session'")
	return err
}
