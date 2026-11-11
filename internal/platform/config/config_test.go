package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaultsAndEnvOverride(t *testing.T) {
	root := t.TempDir()
	writeConfigs(t, root, "server:\n  port: 8080\nlogging:\n  format: text\n", "logging:\n  level: debug\n")
	t.Setenv("APP_SERVER_PORT", "9090")
	t.Setenv("APP_DATABASE_SQLITE_FOREIGN_KEYS", "false")
	t.Setenv("APP_PROFILE", "dev")
	cfg, err := Load([]string{"--base-dir", root, "--config-dir", "configs", "--data-dir", "data"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 9090 {
		t.Fatalf("port = %d", cfg.Server.Port)
	}
	if cfg.Database.SQLite.ForeignKeys {
		t.Fatal("false env did not override foreign keys")
	}
	if cfg.Logging.Level != "debug" || cfg.App.Profile != "dev" {
		t.Fatalf("profile config = %+v", cfg.Logging)
	}
	if !filepath.IsAbs(cfg.App.DataDir) || !filepath.IsAbs(cfg.SQLiteFile()) {
		t.Fatalf("paths not absolute: %s %s", cfg.App.DataDir, cfg.SQLiteFile())
	}
	if cfg.Server.MaxBodyBytes != 2097152 {
		t.Fatalf("default body = %d", cfg.Server.MaxBodyBytes)
	}
	if cfg.Auth.SessionTTL != 8*time.Hour {
		t.Fatalf("ttl = %s", cfg.Auth.SessionTTL)
	}
}

func TestUnknownKeyAndMissingProfile(t *testing.T) {
	root := t.TempDir()
	writeConfigs(t, root, "server:\n  nope: 1\n", "")
	if _, err := Load([]string{"--base-dir", root}); err == nil {
		t.Fatal("unknown key should fail")
	}
	writeConfigs(t, root, "app:\n  name: yunjian\n", "")
	if _, err := Load([]string{"--base-dir", root, "--profile", "pro"}); err == nil {
		t.Fatal("missing profile file should fail")
	}
	if _, err := Load([]string{"--base-dir", root, "--profile", "staging"}); err == nil {
		t.Fatal("unknown profile should fail")
	}
}

func TestExplicitEmptySecretFails(t *testing.T) {
	root := t.TempDir()
	writeConfigs(t, root, "database:\n  driver: mysql\n", "")
	t.Setenv("APP_DATABASE_DSN", "")
	if _, err := Load([]string{"--base-dir", root}); err == nil {
		t.Fatal("empty dsn should fail")
	}
}

func TestLocatorKeysRejected(t *testing.T) {
	root := t.TempDir()
	writeConfigs(t, root, "app:\n  data_dir: ./data\n", "")
	if _, err := Load([]string{"--base-dir", root}); err == nil {
		t.Fatal("yaml data dir should fail")
	}
}

func writeConfigs(t *testing.T, root, base, dev string) {
	t.Helper()
	dir := filepath.Join(root, "configs")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "application.yaml"), []byte(base), 0o640); err != nil {
		t.Fatal(err)
	}
	if dev != "" {
		if err := os.WriteFile(filepath.Join(dir, "application-dev.yaml"), []byte(dev), 0o640); err != nil {
			t.Fatal(err)
		}
	}
}
