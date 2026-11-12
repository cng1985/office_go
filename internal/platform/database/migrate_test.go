package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"

	"yunjian/internal/platform/config"
	"yunjian/migrations"
)

func TestMigrateIsIdempotentAndEnforcesForeignKeys(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "app.db")
	db, err := OpenAndMigrateSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	if err := Migrate(ctx, db, "sqlite"); err != nil {
		t.Fatal(err)
	}
	var fk int
	if err := db.Raw("PRAGMA foreign_keys").Scan(&fk).Error; err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d", fk)
	}
	err = db.Exec("INSERT INTO office_document_version (document_id, version_no, title, kind, content, content_sha, byte_size, note, created_by, created_at) VALUES (99, 1, 't', 'word', '{}', 'abc', 2, '', 1, '2026-01-01T00:00:00Z')").Error
	if err == nil {
		t.Fatal("foreign key violation was not reported")
	}
}

func TestFolderMigrationUpgradesExistingDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")
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
	db, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	payload, err := migrations.Files.ReadFile("0001_init.sqlite.sql")
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range splitSQL(string(payload)) {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := sqlDB.Exec(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at DATETIME NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES ('0001_init', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db, "sqlite"); err != nil {
		t.Fatal(err)
	}
	var columns int
	if err := db.Raw(`SELECT COUNT(*) FROM pragma_table_info('office_document') WHERE name = 'folder_id'`).Scan(&columns).Error; err != nil || columns != 1 {
		t.Fatalf("folder_id columns = %d err=%v", columns, err)
	}
	var tables int
	if err := db.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'office_folder'`).Scan(&tables).Error; err != nil || tables != 1 {
		t.Fatalf("folder table = %d err=%v", tables, err)
	}
	if err := Migrate(ctx, db, "sqlite"); err != nil {
		t.Fatal(err)
	}
}

func TestBackupSQLiteRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "app.db")
	db, err := OpenAndMigrateSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO account_user (username, display_name, password_hash, created_at, updated_at) VALUES ('ada', 'Ada', 'hash', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')").Error; err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "backup.db")
	if err := BackupSQLite(ctx, db, dest); err != nil {
		t.Fatal(err)
	}
	backup, err := OpenAndMigrateSQLite(ctx, dest)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, item := range []*gorm.DB{db, backup} {
			sqlDB, _ := item.DB()
			if sqlDB != nil {
				_ = sqlDB.Close()
			}
		}
	})
	var count int64
	if err := backup.Raw("SELECT COUNT(*) FROM account_user").Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("backup count = %d", count)
	}
}
