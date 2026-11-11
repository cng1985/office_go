package document

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"time"
	"yunjian/internal/modules/account"
	"yunjian/internal/platform/config"
	"yunjian/internal/platform/database"
	"yunjian/internal/platform/domain"
	apperrors "yunjian/internal/platform/errors"
)

func TestDocumentVersionsAndIsolation(t *testing.T) {
	db := openDB(t)
	owner := scopeOf(t, db, "owner")
	other := scopeOf(t, db, "other")
	resource := NewResource(NewRepository(db))
	created, err := resource.Create(owner, &DocumentCreateRequest{Kind: KindWord, Title: "  项目纪要  "})
	if err != nil {
		t.Fatal(err)
	}
	if created.Data.Revision != 1 || created.Data.VersionNo != 1 || created.Data.Title != "项目纪要" {
		t.Fatalf("created = %+v", created.Data)
	}
	if _, err := resource.One(other, &DocumentIDRequest{ID: created.Data.ID}); err == nil {
		t.Fatal("other user should not read the document")
	}
	saved, err := resource.Save(owner, &DocumentSaveRequest{
		ID:       created.Data.ID,
		Revision: created.Data.Revision,
		Title:    "项目纪要",
		Note:     "补充结论",
		Content:  []byte(`{"blocks":[{"id":"b1","type":"heading1","align":"left","runs":[{"text":"结论","bold":true,"italic":false,"underline":false}]}]}`),
	})
	if err != nil || !saved.Data.NewVersion || saved.Data.Document.VersionNo != 2 {
		t.Fatalf("save = %+v err=%v", saved.Data, err)
	}
	again, err := resource.Save(owner, &DocumentSaveRequest{
		ID:       saved.Data.Document.ID,
		Revision: saved.Data.Document.Revision,
		Title:    "项目纪要",
		Content:  saved.Data.Document.Content,
	})
	if err != nil || again.Data.NewVersion || again.Data.Document.VersionNo != 2 {
		t.Fatalf("identical save = %+v err=%v", again.Data, err)
	}
	if _, err := resource.Save(owner, &DocumentSaveRequest{
		ID: saved.Data.Document.ID, Revision: 1, Title: "过期", Content: saved.Data.Document.Content,
	}); err == nil {
		t.Fatal("stale revision should conflict")
	}
	renamed, err := resource.Update(owner, &DocumentUpdateRequest{
		ID: saved.Data.Document.ID, Revision: again.Data.Document.Revision, Title: "纪要终稿",
	})
	if err != nil || renamed.Data.Title != "纪要终稿" {
		t.Fatalf("rename = %+v err=%v", renamed.Data, err)
	}
	versions, err := resource.VersionSearch(owner, &DocumentVersionSearch{DocumentID: created.Data.ID})
	if err != nil || versions.Total != 2 || len(versions.Data) != 2 {
		t.Fatalf("versions = %+v err=%v", versions, err)
	}
	first, err := resource.VersionOne(owner, &DocumentVersionRequest{DocumentID: created.Data.ID, VersionID: versions.Data[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := resource.Restore(owner, &DocumentRestoreRequest{
		ID: created.Data.ID, VersionID: first.Data.ID, Revision: renamed.Data.Revision,
	})
	if err != nil || restored.Data.Document.VersionNo != 3 || restored.Data.Document.Title != "项目纪要" {
		t.Fatalf("restore = %+v err=%v", restored.Data, err)
	}
	page, err := resource.Search(owner, &DocumentSearch{Title: "纪要", Kind: KindWord})
	if err != nil || page.Total != 1 {
		t.Fatalf("search = %+v err=%v", page, err)
	}
	empty, err := resource.Search(owner, &DocumentSearch{Title: "不存在"})
	if err != nil || empty.Total != 0 || empty.Data == nil {
		t.Fatalf("empty search = %+v err=%v", empty, err)
	}
	if _, err := resource.Search(owner, &DocumentSearch{Sort: "name"}); err == nil {
		t.Fatal("sort whitelist should reject name")
	}
	if _, err := resource.Delete(owner, &DocumentIDRequest{ID: created.Data.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := resource.Delete(owner, &DocumentIDRequest{ID: created.Data.ID}); err == nil {
		t.Fatal("repeat delete should be not found")
	}
}

func TestSaveRollsBackWhenVersionInsertFails(t *testing.T) {
	db := openDB(t)
	ctx := scopeOf(t, db, "writer")
	resource := NewResource(NewRepository(db))
	created, err := resource.Create(ctx, &DocumentCreateRequest{Kind: KindExcel, Title: "预算"})
	if err != nil {
		t.Fatal(err)
	}
	db.Callback().Create().Before("gorm:create").Register("test:fail-version", func(tx *gorm.DB) {
		if tx.Statement.Table == "office_document_version" {
			_ = tx.AddError(errors.New("forced version failure"))
		}
	})
	t.Cleanup(func() {
		db.Callback().Create().Remove("test:fail-version")
	})
	_, err = resource.Save(ctx, &DocumentSaveRequest{
		ID: created.Data.ID, Revision: created.Data.Revision, Title: "预算",
		Content: []byte(`{"sheet_name":"预算","row_count":30,"col_count":12,"cells":[{"row":1,"col":1,"input":"100","bold":false}]}`),
	})
	if err == nil {
		t.Fatal("forced failure should surface")
	}
	current, err := resource.One(ctx, &DocumentIDRequest{ID: created.Data.ID})
	if err != nil {
		t.Fatal(err)
	}
	if current.Data.Revision != created.Data.Revision || current.Data.VersionNo != 1 {
		t.Fatalf("rolled back state = %+v", current.Data)
	}
}

func TestExcelCanonicalAcceptsLegacySheet(t *testing.T) {
	got, err := Canonical(KindExcel, []byte(`{"sheet_name":"预算","row_count":30,"col_count":12,"cells":[{"row":2,"col":1,"input":"=A1","bold":false},{"row":1,"col":1,"input":"100","bold":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("sheet_name")) || bytes.Contains(got, []byte(`"bold":false`)) {
		t.Fatalf("legacy fields leaked: %s", got)
	}
	want := `{"sheets":[{"name":"预算","row_count":30,"col_count":12,"cells":[{"row":1,"col":1,"input":"100","bold":true},{"row":2,"col":1,"input":"=A1"}]}]}`
	if string(got) != want {
		t.Fatalf("canonical = %s", got)
	}
	formatted, err := Canonical(KindExcel, []byte(`{"active":1,"sheets":[{"name":"Sheet1","row_count":40,"col_count":16,"cells":[]},{"name":"明细","row_count":10,"col_count":4,"col_widths":[0,80,0,0],"cells":[{"row":1,"col":1,"input":"你好\n世界","italic":true,"align":"center","color":"#AABBCC","fill":"#112233","format":"text"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(formatted, []byte(`"active":1`)) || !bytes.Contains(formatted, []byte(`你好\n世界`)) || !bytes.Contains(formatted, []byte(`"color":"#aabbcc"`)) {
		t.Fatalf("formatted = %s", formatted)
	}
	if _, err := Canonical(KindExcel, []byte(`{"sheets":[{"name":"Sheet1","row_count":40,"col_count":16,"cells":[{"row":1,"col":1,"input":"1","extra":true}]}]}`)); err == nil {
		t.Fatal("unknown cell field should fail")
	}
	styled, err := Canonical(KindExcel, []byte(`{"sheets":[{"name":"工作表1","row_count":4,"col_count":3,"cells":[{"row":1,"col":1,"input":"12.5","format":"currency","decimals":0,"strike":true,"font":"sans","size":10,"border":"bottom","valign":"top","note":"备注","merge_rows":2,"merge_cols":2}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(styled, []byte(`"decimals":0`)) || !bytes.Contains(styled, []byte(`"format":"currency"`)) || !bytes.Contains(styled, []byte(`"merge_rows":2`)) {
		t.Fatalf("styled = %s", styled)
	}
	if _, err := Canonical(KindExcel, []byte(`{"sheets":[{"name":"工作表1","row_count":4,"col_count":3,"cells":[{"row":1,"col":1,"input":"1","decimals":9}]}]}`)); err == nil {
		t.Fatal("decimals should fail")
	}
}

func TestWordAndPPTFormatting(t *testing.T) {
	plain, err := Canonical(KindWord, []byte(`{"blocks":[{"id":"b1","type":"paragraph","align":"left","runs":[{"text":"甲","bold":false,"italic":false,"underline":false}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != `{"blocks":[{"id":"b1","type":"paragraph","align":"left","runs":[{"text":"甲","bold":false,"italic":false,"underline":false}]}]}` {
		t.Fatalf("plain = %s", plain)
	}
	styled, err := Canonical(KindWord, []byte(`{"blocks":[{"id":"b1","type":"heading3","align":"justify","indent":1,"spacing":"1.5","runs":[{"text":"标题","bold":true,"italic":false,"underline":false,"strike":true,"color":"#AABBCC","highlight":"#FFEEAA","size":18,"font":"serif"},{"text":"标题","bold":true,"italic":false,"underline":false,"strike":true,"color":"#aabbcc","highlight":"#ffeeaa","size":18,"font":"serif"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(styled, []byte(`"color":"#aabbcc"`)) || !bytes.Contains(styled, []byte(`"spacing":"1.5"`)) || bytes.Count(styled, []byte(`"text"`)) != 1 {
		t.Fatalf("styled = %s", styled)
	}
	if _, err := Canonical(KindWord, []byte(`{"blocks":[{"id":"b1","type":"paragraph","align":"left","indent":9,"runs":[{"text":"甲","bold":false,"italic":false,"underline":false}]}]}`)); err == nil {
		t.Fatal("indent should be rejected")
	}

	deck, err := Canonical(KindPPT, []byte(`{"slides":[{"id":"s1","background":"#FFFDF8","notes":"第一行\n第二行","elements":[{"id":"e1","kind":"shape","shape":"ellipse","x":80,"y":80,"w":240,"h":140,"text":"形状","font_size":18,"color":"#112233","bold":false,"align":"center","fill":"#AABBCC","italic":true}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(deck, []byte(`"shape":"ellipse"`)) || !bytes.Contains(deck, []byte(`"fill":"#aabbcc"`)) || !bytes.Contains(deck, []byte(`第一行\n第二行`)) {
		t.Fatalf("deck = %s", deck)
	}
	if _, err := Canonical(KindPPT, []byte(`{"slides":[{"id":"s1","background":"#fffdf8","elements":[{"id":"e1","kind":"text","shape":"rect","x":80,"y":80,"w":240,"h":140,"text":"","font_size":18,"color":"#112233","bold":false,"align":"left"}]}]}`)); err == nil {
		t.Fatal("text element should not carry a shape")
	}
}

func TestContentRejectsUnknownShape(t *testing.T) {
	if _, err := Canonical(KindPPT, []byte(`{"slides":[{"id":"s1","background":"red","elements":[]}],"extra":1}`)); err == nil {
		t.Fatal("unknown field and bad color should fail")
	}
	var appErr *apperrors.Error
	_, err := Canonical(KindWord, []byte(`{"blocks":[]}`))
	if !errors.As(err, &appErr) || appErr.Code != 400 {
		t.Fatalf("err = %v", err)
	}
}

func scopeOf(t *testing.T, db *gorm.DB, username string) context.Context {
	t.Helper()
	cfg := &config.Config{}
	cfg.Auth.SessionTTL = time.Hour
	repo := account.NewRepository(db)
	resource, err := account.NewResource(repo, cfg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := resource.Register(domain.WithClientIP(context.Background(), "198.51.100.8"), &account.AccountRegisterRequest{
		Username: username,
		Password: "secret-pass",
	})
	if err != nil {
		t.Fatal(err)
	}
	scope, err := repo.Authenticate(context.Background(), result.Data.Token)
	if err != nil {
		t.Fatal(err)
	}
	return domain.WithScope(context.Background(), scope)
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
