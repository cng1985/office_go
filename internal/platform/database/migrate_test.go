package database

import (
	"context"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
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
