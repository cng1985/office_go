package calendar

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"yunjian/internal/platform/database"
	"yunjian/internal/platform/domain"
)

func TestCalendarLifecycle(t *testing.T) {
	resource, owner, other := newCalendarResource(t)
	ctx := asUser(owner)
	if _, err := resource.Create(context.Background(), &EventWriteRequest{Title: "未登录", AllDay: true, StartOn: "2026-11-12", EndOn: "2026-11-12"}); err == nil || !strings.Contains(err.Error(), "请先登录") {
		t.Fatalf("anonymous create = %v", err)
	}
	if _, err := resource.Create(ctx, &EventWriteRequest{ID: 9, Title: "指定", AllDay: true, StartOn: "2026-11-12", EndOn: "2026-11-12"}); err == nil || !strings.Contains(err.Error(), "不能指定标识") {
		t.Fatalf("create with id = %v", err)
	}
	if _, err := resource.Create(ctx, &EventWriteRequest{Title: "   ", AllDay: true, StartOn: "2026-11-12", EndOn: "2026-11-12"}); err == nil || !strings.Contains(err.Error(), "请填写日程标题") {
		t.Fatalf("empty title = %v", err)
	}
	if _, err := resource.Create(ctx, &EventWriteRequest{Title: "颜色", AllDay: true, StartOn: "2026-11-12", EndOn: "2026-11-12", Color: "#ffffff"}); err == nil || !strings.Contains(err.Error(), "颜色无效") {
		t.Fatalf("bad color = %v", err)
	}
	if _, err := resource.Create(ctx, &EventWriteRequest{Title: "倒序", AllDay: true, StartOn: "2026-11-13", EndOn: "2026-11-12"}); err == nil || !strings.Contains(err.Error(), "结束日期") {
		t.Fatalf("reversed dates = %v", err)
	}
	if _, err := resource.Create(ctx, &EventWriteRequest{Title: "太长", AllDay: true, StartOn: "2026-11-01", EndOn: "2026-12-02"}); err == nil || !strings.Contains(err.Error(), "31") {
		t.Fatalf("32 day span = %v", err)
	}
	long, err := resource.Create(ctx, &EventWriteRequest{Title: "季度", AllDay: true, StartOn: "2027-01-01", EndOn: "2027-01-31"})
	if err != nil || long.Data.StartOn != "2027-01-01" || long.Data.EndOn != "2027-01-31" {
		t.Fatalf("31 day span = %+v err=%v", long, err)
	}
	if _, err := resource.Delete(ctx, &EventIDRequest{ID: long.Data.ID}); err != nil {
		t.Fatal(err)
	}

	created, err := resource.Create(ctx, &EventWriteRequest{
		Title:    "  周会  ",
		AllDay:   true,
		StartOn:  "2026-11-12",
		EndOn:    "2026-11-13",
		Color:    "#1A73E8",
		Location: "三楼",
		Notes:    "带上纪要",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Data.Title != "周会" || created.Data.Color != "#1a73e8" || !created.Data.AllDay || created.Data.StartOn != "2026-11-12" || created.Data.EndOn != "2026-11-13" {
		t.Fatalf("created = %+v", created.Data)
	}
	if !created.Data.EndsAt.Equal(time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("all-day end = %s", created.Data.EndsAt)
	}
	assertDays(t, resource, ctx, "2026-11-12", "2026-11-12", 1)
	assertDays(t, resource, ctx, "2026-11-13", "2026-11-13", 1)
	assertDays(t, resource, ctx, "2026-11-14", "2026-11-14", 0)
	assertDays(t, resource, asUser(other), "2026-11-12", "2026-11-13", 0)
	if _, err := resource.Update(asUser(other), &EventWriteRequest{ID: created.Data.ID, Title: "别人的", AllDay: true, StartOn: "2026-11-12", EndOn: "2026-11-12"}); err == nil || !strings.Contains(err.Error(), "日程不存在") {
		t.Fatalf("other update = %v", err)
	}
	if _, err := resource.Delete(asUser(other), &EventIDRequest{ID: created.Data.ID}); err == nil || !strings.Contains(err.Error(), "日程不存在") {
		t.Fatalf("other delete = %v", err)
	}

	updated, err := resource.Update(ctx, &EventWriteRequest{
		ID:      created.Data.ID,
		Title:   "周会纪要",
		AllDay:  true,
		StartOn: "2026-11-12",
		EndOn:   "2026-11-12",
		Notes:   "已改期",
	})
	if err != nil || updated.Data.Title != "周会纪要" || updated.Data.EndOn != "2026-11-12" || updated.Data.Location != "" || updated.Data.Notes != "已改期" {
		t.Fatalf("updated = %+v err=%v", updated.Data, err)
	}
	assertDays(t, resource, ctx, "2026-11-13", "2026-11-13", 0)

	start := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	timed, err := resource.Create(ctx, &EventWriteRequest{Title: "评审", StartsAt: start, EndsAt: end, Location: "线上"})
	if err != nil || timed.Data.AllDay || !timed.Data.StartsAt.Equal(start) || timed.Data.StartOn != "" {
		t.Fatalf("timed = %+v err=%v", timed.Data, err)
	}
	if _, err := resource.Create(ctx, &EventWriteRequest{Title: "倒序时刻", StartsAt: end, EndsAt: start}); err == nil {
		t.Fatal("reversed times should fail")
	}
	if _, err := resource.Create(ctx, &EventWriteRequest{Title: "太久", StartsAt: start, EndsAt: start.Add(maxTimed + time.Second)}); err == nil || !strings.Contains(err.Error(), "14") {
		t.Fatalf("long timed = %v", err)
	}
	assertOverlap(t, resource, ctx, start.Add(time.Hour), start.Add(2*time.Hour), 1)
	assertOverlap(t, resource, ctx, start.Add(30*time.Minute), start.Add(45*time.Minute), 2)
	assertOverlap(t, resource, ctx, start.Add(-time.Hour), start, 1)

	if _, err := resource.Delete(ctx, &EventIDRequest{ID: created.Data.ID}); err != nil {
		t.Fatal(err)
	}
	assertOverlap(t, resource, ctx, start, end, 1)
	if _, err := resource.Delete(ctx, &EventIDRequest{ID: timed.Data.ID}); err != nil {
		t.Fatal(err)
	}
	assertOverlap(t, resource, ctx, start, end, 0)
	if _, err := resource.Delete(ctx, &EventIDRequest{ID: timed.Data.ID}); err == nil {
		t.Fatal("second delete should fail")
	}
}

func TestCalendarSearchCap(t *testing.T) {
	resource, owner, _ := newCalendarResource(t)
	ctx := asUser(owner)
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < maxSearchRows; i++ {
		_, err := resource.Create(ctx, &EventWriteRequest{
			Title:    fmt.Sprintf("事项%d", i),
			StartsAt: start.Add(time.Duration(i) * time.Minute),
			EndsAt:   start.Add(time.Duration(i+1) * time.Minute),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	found, err := resource.Search(ctx, &EventSearchRequest{
		From: start, To: start.Add(48 * time.Hour), StartOn: "2026-06-01", EndOn: "2026-06-02",
	})
	if err != nil || len(found.Data.Events) != maxSearchRows {
		t.Fatalf("search 400 = %d err=%v", len(found.Data.Events), err)
	}
	if _, err := resource.Create(ctx, &EventWriteRequest{
		Title: "再一条", StartsAt: start.Add(48 * time.Hour), EndsAt: start.Add(49 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := resource.Search(ctx, &EventSearchRequest{
		From: start, To: start.Add(70 * time.Hour), StartOn: "2026-06-01", EndOn: "2026-06-03",
	}); err == nil || !strings.Contains(err.Error(), "过多") {
		t.Fatalf("over cap = %v", err)
	}
}

func assertDays(t *testing.T, resource *Resource, ctx context.Context, startOn, endOn string, want int) {
	t.Helper()
	start, err := time.Parse(time.RFC3339, startOn+"T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	found, err := resource.Search(ctx, &EventSearchRequest{
		From: start, To: start.Add(24 * time.Hour), StartOn: startOn, EndOn: endOn,
	})
	if err != nil || len(found.Data.Events) != want {
		t.Fatalf("days %s..%s = %+v err=%v", startOn, endOn, found.Data.Events, err)
	}
}

func assertOverlap(t *testing.T, resource *Resource, ctx context.Context, from, to time.Time, want int) {
	t.Helper()
	found, err := resource.Search(ctx, &EventSearchRequest{
		From: from, To: to, StartOn: "2026-11-12", EndOn: "2026-11-13",
	})
	if err != nil || len(found.Data.Events) != want {
		t.Fatalf("overlap %s..%s = %+v err=%v", from, to, found.Data.Events, err)
	}
}

func newCalendarResource(t *testing.T) (*Resource, int64, int64) {
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
	return NewResource(NewRepository(db)), seedUser(t, db, "ada"), seedUser(t, db, "grace")
}

func seedUser(t *testing.T, db *gorm.DB, name string) int64 {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	if err := db.Exec(
		`INSERT INTO account_user (username, display_name, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		name, name, "x", now, now,
	).Error; err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := db.Raw(`SELECT id FROM account_user WHERE username = ?`, name).Scan(&id).Error; err != nil || id == 0 {
		t.Fatalf("user id = %d err=%v", id, err)
	}
	return id
}

func asUser(id int64) context.Context {
	return domain.WithScope(context.Background(), domain.Scope{ActorID: id})
}
