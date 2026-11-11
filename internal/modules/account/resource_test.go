package account

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"

	"yunjian/internal/platform/auth"
	"yunjian/internal/platform/config"
	"yunjian/internal/platform/database"
	"yunjian/internal/platform/domain"
)

func TestRegisterLoginLogout(t *testing.T) {
	resource, repo := newAccountResource(t)
	ctx := domain.WithClientIP(context.Background(), "203.0.113.10")
	registered, err := resource.Register(ctx, &AccountRegisterRequest{
		Username:    "Ada",
		Password:    "secret-pass",
		DisplayName: "艾达",
	})
	if err != nil {
		t.Fatal(err)
	}
	if registered.Code != 200 || registered.Data.Account.Username != "ada" || registered.Data.Token == "" {
		t.Fatalf("register = %+v", registered)
	}
	if _, err := resource.Register(ctx, &AccountRegisterRequest{Username: "ada", Password: "secret-pass"}); err == nil {
		t.Fatal("duplicate username should fail")
	}
	if _, err := resource.Login(ctx, &AccountLoginRequest{Username: "ada", Password: "wrong-password"}); err == nil {
		t.Fatal("bad password should fail")
	}
	logged, err := resource.Login(ctx, &AccountLoginRequest{Username: "ADA", Password: "secret-pass"})
	if err != nil {
		t.Fatal(err)
	}
	scope, err := repo.Authenticate(ctx, logged.Data.Token)
	if err != nil {
		t.Fatal(err)
	}
	me, err := resource.Me(domain.WithScope(ctx, scope), &AccountMeRequest{})
	if err != nil || me.Data.DisplayName != "艾达" {
		t.Fatalf("me = %+v err=%v", me, err)
	}
	if _, err := resource.Logout(domain.WithScope(ctx, scope), &AccountLogoutRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Authenticate(ctx, logged.Data.Token); err == nil {
		t.Fatal("revoked session should fail")
	}
	if _, err := resource.Register(ctx, &AccountRegisterRequest{Username: "ab", Password: "short"}); err == nil {
		t.Fatal("invalid username and password should fail")
	}
}

func newAccountResource(t *testing.T) (*Resource, *Repository) {
	t.Helper()
	db := openDB(t)
	repo := NewRepository(db)
	cfg := &config.Config{}
	cfg.Auth.SessionTTL = time.Hour
	resource, err := NewResource(repo, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return resource, repo
}

func openDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.OpenAndMigrateSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestPasswordHashNotComparedForMissingUser(t *testing.T) {
	resource, _ := newAccountResource(t)
	ctx := domain.WithClientIP(context.Background(), "203.0.113.11")
	_, err := resource.Login(ctx, &AccountLoginRequest{Username: "ghost", Password: "secret-pass"})
	if err == nil {
		t.Fatal("missing user should fail")
	}
	if auth.CheckPassword(resource.dummyHash, "secret-pass") {
		t.Fatal("dummy hash should not match a normal password")
	}
}
