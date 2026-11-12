package account

import (
	"context"
	"strings"
	"testing"
	"time"

	"yunjian/internal/platform/domain"
)

func TestProfileAndPassword(t *testing.T) {
	resource, repo := newAccountResource(t)
	ctx := domain.WithClientIP(context.Background(), "203.0.113.20")
	registered, err := resource.Register(ctx, &AccountRegisterRequest{
		Username:    "ada",
		Password:    "secret-pass",
		DisplayName: "艾达",
	})
	if err != nil {
		t.Fatal(err)
	}
	scope, err := repo.Authenticate(ctx, registered.Data.Token)
	if err != nil {
		t.Fatal(err)
	}
	authed := domain.WithScope(ctx, scope)
	updated, err := resource.UpdateProfile(authed, &AccountProfileRequest{DisplayName: " 艾达·设计 "})
	if err != nil || updated.Data.DisplayName != "艾达·设计" || updated.Data.Username != "ada" {
		t.Fatalf("profile = %+v err=%v", updated.Data, err)
	}
	if _, err := resource.UpdateProfile(authed, &AccountProfileRequest{DisplayName: "坏\n名称"}); err == nil {
		t.Fatal("control character in display name should fail")
	}
	me, err := resource.Me(authed, &AccountMeRequest{})
	if err != nil || me.Data.DisplayName != "艾达·设计" {
		t.Fatalf("me = %+v err=%v", me.Data, err)
	}
	if _, err := resource.ChangePassword(authed, &AccountPasswordRequest{CurrentPassword: "wrong-pass", NewPassword: "another-pass"}); err == nil || !strings.Contains(err.Error(), "当前密码不正确") {
		t.Fatalf("wrong current password err=%v", err)
	}
	changed, err := resource.ChangePassword(authed, &AccountPasswordRequest{CurrentPassword: "secret-pass", NewPassword: "another-pass"})
	if err != nil || changed.Data.Username != "ada" {
		t.Fatalf("password = %+v err=%v", changed.Data, err)
	}
	if _, err := repo.Authenticate(ctx, registered.Data.Token); err != nil {
		t.Fatal("password change should keep the current session")
	}
	if _, err := resource.Login(ctx, &AccountLoginRequest{Username: "ada", Password: "secret-pass"}); err == nil {
		t.Fatal("old password should fail")
	}
	if _, err := resource.Login(ctx, &AccountLoginRequest{Username: "ada", Password: "another-pass"}); err != nil {
		t.Fatal(err)
	}
}

func TestTeamMembership(t *testing.T) {
	resource, repo := newAccountResource(t)
	ctx := domain.WithClientIP(context.Background(), "203.0.113.21")
	owner := registerScope(t, resource, repo, ctx, "ada", "secret-pass")
	member := registerScope(t, resource, repo, ctx, "bob", "secret-pass")
	outsider := registerScope(t, resource, repo, ctx, "cara", "secret-pass")

	created, err := resource.CreateTeam(owner, &TeamCreateRequest{Name: " 设计组 "})
	if err != nil || created.Data.Name != "设计组" || created.Data.Role != roleOwner || len(created.Data.Members) != 1 {
		t.Fatalf("create = %+v err=%v", created.Data, err)
	}
	teamID := created.Data.ID
	if _, err := resource.AddTeamMember(owner, &TeamInviteRequest{TeamID: teamID, Username: "missing"}); err == nil || !strings.Contains(err.Error(), "用户不存在") {
		t.Fatalf("missing user err=%v", err)
	}
	if _, err := resource.AddTeamMember(member, &TeamInviteRequest{TeamID: teamID, Username: "cara"}); err == nil || !strings.Contains(err.Error(), "团队不存在") {
		t.Fatalf("outsider invite err=%v", err)
	}
	invited, err := resource.AddTeamMember(owner, &TeamInviteRequest{TeamID: teamID, Username: "Bob"})
	if err != nil || len(invited.Data.Members) != 2 || invited.Data.Members[1].Username != "bob" || invited.Data.Members[1].Role != roleMember {
		t.Fatalf("invite = %+v err=%v", invited.Data.Members, err)
	}
	if _, err := resource.AddTeamMember(owner, &TeamInviteRequest{TeamID: teamID, Username: "bob"}); err == nil || !strings.Contains(err.Error(), "已经是团队成员") {
		t.Fatalf("duplicate err=%v", err)
	}
	seen, err := resource.TeamOne(member, &TeamIDRequest{ID: teamID})
	if err != nil || seen.Data.Role != roleMember || seen.Data.Name != "设计组" {
		t.Fatalf("member one = %+v err=%v", seen.Data, err)
	}
	if _, err := resource.TeamOne(outsider, &TeamIDRequest{ID: teamID}); err == nil || !strings.Contains(err.Error(), "团队不存在") {
		t.Fatalf("outsider one err=%v", err)
	}
	if _, err := resource.UpdateTeam(member, &TeamUpdateRequest{ID: teamID, Name: "不能改"}); err == nil || !strings.Contains(err.Error(), "所有者") {
		t.Fatalf("member rename err=%v", err)
	}
	renamed, err := resource.UpdateTeam(owner, &TeamUpdateRequest{ID: teamID, Name: "设计一组"})
	if err != nil || renamed.Data.Name != "设计一组" {
		t.Fatalf("rename = %+v err=%v", renamed.Data, err)
	}
	listed, err := resource.SearchTeams(member, &TeamSearch{})
	if err != nil || listed.Total != 1 || listed.Data[0].Name != "设计一组" || listed.Data[0].Role != roleMember {
		t.Fatalf("search = %+v err=%v", listed, err)
	}
	ownerAccount, err := resource.Me(owner, &AccountMeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resource.RemoveTeamMember(owner, &TeamMemberRemoveRequest{TeamID: teamID, UserID: ownerAccount.Data.ID}); err == nil || !strings.Contains(err.Error(), "不能移出团队所有者") {
		t.Fatalf("remove owner err=%v", err)
	}
	if _, err := resource.LeaveTeam(owner, &TeamLeaveRequest{TeamID: teamID}); err == nil || !strings.Contains(err.Error(), "所有者不能退出") {
		t.Fatalf("owner leave err=%v", err)
	}
	memberAccount, err := resource.Me(member, &AccountMeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	removed, err := resource.RemoveTeamMember(owner, &TeamMemberRemoveRequest{TeamID: teamID, UserID: memberAccount.Data.ID})
	if err != nil || len(removed.Data.Members) != 1 {
		t.Fatalf("remove = %+v err=%v", removed.Data.Members, err)
	}
	if _, err := resource.TeamOne(member, &TeamIDRequest{ID: teamID}); err == nil {
		t.Fatal("removed member should not see the team")
	}
	if _, err := resource.AddTeamMember(owner, &TeamInviteRequest{TeamID: teamID, Username: "bob"}); err != nil {
		t.Fatal(err)
	}
	if _, err := resource.LeaveTeam(member, &TeamLeaveRequest{TeamID: teamID}); err != nil {
		t.Fatal(err)
	}
	if _, err := resource.TeamOne(member, &TeamIDRequest{ID: teamID}); err == nil {
		t.Fatal("member who left should not see the team")
	}
	deleted, err := resource.DeleteTeam(owner, &TeamIDRequest{ID: teamID})
	if err != nil || deleted.Data.ID != teamID {
		t.Fatalf("delete = %+v err=%v", deleted.Data, err)
	}
	if _, err := resource.TeamOne(owner, &TeamIDRequest{ID: teamID}); err == nil {
		t.Fatal("deleted team should be gone")
	}
}

func TestTeamDeleteRefusesFilesAndCreationLimit(t *testing.T) {
	resource, repo := newAccountResource(t)
	ctx := domain.WithClientIP(context.Background(), "203.0.113.22")
	owner := registerScope(t, resource, repo, ctx, "ada", "secret-pass")
	created, err := resource.CreateTeam(owner, &TeamCreateRequest{Name: "资料组"})
	if err != nil {
		t.Fatal(err)
	}
	me, err := resource.Me(owner, &AccountMeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := repo.db.Exec(
		`INSERT INTO office_document (owner_id, team_id, kind, title, current_version_id, revision, created_at, updated_at) VALUES (?, ?, 'word', '共享', 0, 1, ?, ?)`,
		me.Data.ID, created.Data.ID, now, now,
	).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := resource.DeleteTeam(owner, &TeamIDRequest{ID: created.Data.ID}); err == nil || !strings.Contains(err.Error(), "请先清空") {
		t.Fatalf("delete with files err=%v", err)
	}
	if err := repo.db.Exec(`DELETE FROM office_document WHERE team_id = ?`, created.Data.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := resource.DeleteTeam(owner, &TeamIDRequest{ID: created.Data.ID}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxOwnedTeams; i++ {
		if _, err := resource.CreateTeam(owner, &TeamCreateRequest{Name: "组"}); err != nil {
			t.Fatalf("team %d: %v", i, err)
		}
	}
	if _, err := resource.CreateTeam(owner, &TeamCreateRequest{Name: "超出"}); err == nil || !strings.Contains(err.Error(), "上限") {
		t.Fatalf("limit err=%v", err)
	}
}

func registerScope(t *testing.T, resource *Resource, repo *Repository, ctx context.Context, username, password string) context.Context {
	t.Helper()
	registered, err := resource.Register(ctx, &AccountRegisterRequest{Username: username, Password: password})
	if err != nil {
		t.Fatal(err)
	}
	scope, err := repo.Authenticate(ctx, registered.Data.Token)
	if err != nil {
		t.Fatal(err)
	}
	return domain.WithScope(ctx, scope)
}
