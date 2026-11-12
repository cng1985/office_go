package document

import (
	"strings"
	"testing"
	"time"

	"yunjian/internal/modules/account"
	"yunjian/internal/platform/config"
)

func TestTeamLibraryKeepsPersonalDocumentsPrivate(t *testing.T) {
	db := openDB(t)
	resource := NewResource(NewRepository(db))
	cfg := &config.Config{}
	cfg.Auth.SessionTTL = time.Hour
	accounts := account.NewRepository(db)
	accountResource, err := account.NewResource(accounts, cfg)
	if err != nil {
		t.Fatal(err)
	}
	owner := scopeOf(t, db, "ada")
	member := scopeOf(t, db, "bob")
	stranger := scopeOf(t, db, "cara")

	createdTeam, err := accountResource.CreateTeam(owner, &account.TeamCreateRequest{Name: "设计组"})
	if err != nil {
		t.Fatal(err)
	}
	teamID := createdTeam.Data.ID
	if _, err := accountResource.AddTeamMember(owner, &account.TeamInviteRequest{TeamID: teamID, Username: "bob"}); err != nil {
		t.Fatal(err)
	}

	personal, err := resource.Create(owner, &DocumentCreateRequest{Kind: KindWord, Title: "私人"})
	if err != nil {
		t.Fatal(err)
	}
	shared, err := resource.Create(owner, &DocumentCreateRequest{Kind: KindWord, Title: "共享", TeamID: teamID})
	if err != nil || shared.Data.TeamID != teamID {
		t.Fatalf("shared = %+v err=%v", shared.Data, err)
	}
	if _, err := resource.One(member, &DocumentIDRequest{ID: personal.Data.ID}); err == nil {
		t.Fatal("team member should not read a personal document")
	}
	if _, err := resource.One(stranger, &DocumentIDRequest{ID: shared.Data.ID}); err == nil {
		t.Fatal("outsider should not read a team document")
	}
	seen, err := resource.One(member, &DocumentIDRequest{ID: shared.Data.ID})
	if err != nil || seen.Data.Title != "共享" {
		t.Fatalf("member one = %+v err=%v", seen.Data, err)
	}
	updated, err := resource.Update(member, &DocumentUpdateRequest{ID: shared.Data.ID, Revision: shared.Data.Revision, Title: "共享纪要"})
	if err != nil || updated.Data.Title != "共享纪要" {
		t.Fatalf("member update = %+v err=%v", updated.Data, err)
	}
	saved, err := resource.Save(member, &DocumentSaveRequest{
		ID:       updated.Data.ID,
		Revision: updated.Data.Revision,
		Title:    "共享纪要",
		Note:     "成员补充",
		Content:  updated.Data.Content,
	})
	if err != nil || !saved.Data.NewVersion {
		t.Fatalf("member save = %+v err=%v", saved.Data, err)
	}

	ownerDocs, err := resource.Search(owner, &DocumentSearch{})
	if err != nil || !hasTitle(ownerDocs.Data, "私人") || hasTitle(ownerDocs.Data, "共享纪要") {
		t.Fatalf("owner personal search = %+v err=%v", titles(ownerDocs.Data), err)
	}
	memberDocs, err := resource.Search(member, &DocumentSearch{})
	if err != nil || hasTitle(memberDocs.Data, "共享纪要") || hasTitle(memberDocs.Data, "私人") {
		t.Fatalf("member personal search = %+v err=%v", titles(memberDocs.Data), err)
	}
	teamDocs, err := resource.Search(member, &DocumentSearch{TeamID: teamID})
	if err != nil || !hasTitle(teamDocs.Data, "共享纪要") || hasTitle(teamDocs.Data, "私人") {
		t.Fatalf("team search = %+v err=%v", titles(teamDocs.Data), err)
	}
	if _, err := resource.Search(stranger, &DocumentSearch{TeamID: teamID}); err == nil || !strings.Contains(err.Error(), "团队不存在") {
		t.Fatalf("outsider team search err=%v", err)
	}
	if _, err := resource.Create(stranger, &DocumentCreateRequest{Kind: KindWord, Title: "闯入", TeamID: teamID}); err == nil {
		t.Fatal("outsider should not create in the team")
	}

	teamFolder, err := resource.CreateFolder(member, &FolderCreateRequest{Name: "素材", TeamID: teamID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resource.CreateFolder(owner, &FolderCreateRequest{Name: "素材", TeamID: teamID}); err == nil {
		t.Fatal("duplicate team folder name should fail")
	}
	personalFolder, err := resource.CreateFolder(owner, &FolderCreateRequest{Name: "素材"})
	if err != nil {
		t.Fatal(err)
	}
	root := int64(0)
	teamFolders, err := resource.SearchFolders(owner, &FolderSearch{TeamID: teamID, ParentID: &root})
	if err != nil || len(teamFolders.Data) != 1 || teamFolders.Data[0].Name != "素材" {
		t.Fatalf("team folders = %+v err=%v", teamFolders.Data, err)
	}
	ownFolders, err := resource.SearchFolders(owner, &FolderSearch{ParentID: &root})
	if err != nil || len(ownFolders.Data) != 1 || ownFolders.Data[0].ID != personalFolder.Data.ID {
		t.Fatalf("personal folders = %+v err=%v", ownFolders.Data, err)
	}
	if _, err := resource.Create(owner, &DocumentCreateRequest{Kind: KindWord, Title: "放错", FolderID: teamFolder.Data.ID}); err == nil {
		t.Fatal("personal document should stay out of a team folder")
	}
	if _, err := resource.Move(owner, &DocumentMoveRequest{ID: saved.Data.Document.ID, Revision: saved.Data.Document.Revision, FolderID: personalFolder.Data.ID}); err == nil {
		t.Fatal("team document should not move into a personal folder")
	}
	moved, err := resource.Move(member, &DocumentMoveRequest{ID: saved.Data.Document.ID, Revision: saved.Data.Document.Revision, FolderID: teamFolder.Data.ID})
	if err != nil || moved.Data.FolderID != teamFolder.Data.ID {
		t.Fatalf("move = %+v err=%v", moved.Data, err)
	}
	if _, err := accountResource.DeleteTeam(owner, &account.TeamIDRequest{ID: teamID}); err == nil || !strings.Contains(err.Error(), "请先清空") {
		t.Fatalf("delete busy team err=%v", err)
	}

	memberAccount, err := accountResource.Me(member, &account.AccountMeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accountResource.RemoveTeamMember(owner, &account.TeamMemberRemoveRequest{TeamID: teamID, UserID: memberAccount.Data.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := resource.One(member, &DocumentIDRequest{ID: shared.Data.ID}); err == nil {
		t.Fatal("removed member should not read the team document")
	}
	if _, err := accountResource.AddTeamMember(owner, &account.TeamInviteRequest{TeamID: teamID, Username: "bob"}); err != nil {
		t.Fatal(err)
	}
	if _, err := accountResource.LeaveTeam(member, &account.TeamLeaveRequest{TeamID: teamID}); err != nil {
		t.Fatal(err)
	}
	if _, err := resource.Search(member, &DocumentSearch{TeamID: teamID}); err == nil {
		t.Fatal("member who left should not search the team")
	}
	if _, err := accountResource.LeaveTeam(owner, &account.TeamLeaveRequest{TeamID: teamID}); err == nil {
		t.Fatal("owner should not leave")
	}
	ownerAccount, err := accountResource.Me(owner, &account.AccountMeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accountResource.RemoveTeamMember(owner, &account.TeamMemberRemoveRequest{TeamID: teamID, UserID: ownerAccount.Data.ID}); err == nil {
		t.Fatal("owner should not be removed")
	}
	if _, err := resource.Delete(owner, &DocumentIDRequest{ID: shared.Data.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := resource.DeleteFolder(owner, &FolderIDRequest{ID: teamFolder.Data.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := accountResource.DeleteTeam(owner, &account.TeamIDRequest{ID: teamID}); err != nil {
		t.Fatal(err)
	}
}

func hasTitle(items []DocumentSimple, title string) bool {
	for _, item := range items {
		if item.Title == title {
			return true
		}
	}
	return false
}

func titles(items []DocumentSimple) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Title)
	}
	return out
}
