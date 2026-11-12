package account

import (
	"context"

	apperrors "yunjian/internal/platform/errors"
	"yunjian/internal/platform/response"
)

func (r *Resource) UpdateProfile(ctx context.Context, request *AccountProfileRequest) (response.Response[AccountResponse], error) {
	var zero response.Response[AccountResponse]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	name, err := normalizeDisplayName(request.DisplayName)
	if err != nil {
		return zero, err
	}
	return r.repo.UpdateProfile(ctx, name)
}

func (r *Resource) ChangePassword(ctx context.Context, request *AccountPasswordRequest) (response.Response[AccountResponse], error) {
	var zero response.Response[AccountResponse]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := validatePassword(request.NewPassword); err != nil {
		return zero, err
	}
	if request.CurrentPassword == "" {
		return zero, apperrors.Unauthorized(401, "当前密码不正确")
	}
	return r.repo.ChangePassword(ctx, request.CurrentPassword, request.NewPassword)
}

func (r *Resource) CreateTeam(ctx context.Context, request *TeamCreateRequest) (response.Response[TeamDetail], error) {
	var zero response.Response[TeamDetail]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	name, err := normalizeTeamName(request.Name)
	if err != nil {
		return zero, err
	}
	return r.repo.CreateTeam(ctx, name)
}

func (r *Resource) UpdateTeam(ctx context.Context, request *TeamUpdateRequest) (response.Response[TeamDetail], error) {
	var zero response.Response[TeamDetail]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireTeamID(request.ID); err != nil {
		return zero, err
	}
	name, err := normalizeTeamName(request.Name)
	if err != nil {
		return zero, err
	}
	request.Name = name
	return r.repo.UpdateTeam(ctx, request)
}

func (r *Resource) DeleteTeam(ctx context.Context, request *TeamIDRequest) (response.Response[TeamResponse], error) {
	var zero response.Response[TeamResponse]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireTeamID(request.ID); err != nil {
		return zero, err
	}
	return r.repo.DeleteTeam(ctx, request.ID)
}

func (r *Resource) SearchTeams(ctx context.Context, search *TeamSearch) (response.PageResponse[TeamResponse], error) {
	var zero response.PageResponse[TeamResponse]
	if search == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	page, pageSize, err := normalizeTeamPage(search.Page, search.PageSize)
	if err != nil {
		return zero, err
	}
	return r.repo.SearchTeams(ctx, page, pageSize)
}

func (r *Resource) TeamOne(ctx context.Context, request *TeamIDRequest) (response.Response[TeamDetail], error) {
	var zero response.Response[TeamDetail]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireTeamID(request.ID); err != nil {
		return zero, err
	}
	return r.repo.TeamOne(ctx, request.ID)
}

func (r *Resource) AddTeamMember(ctx context.Context, request *TeamInviteRequest) (response.Response[TeamDetail], error) {
	var zero response.Response[TeamDetail]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireTeamID(request.TeamID); err != nil {
		return zero, err
	}
	username, err := normalizeUsername(request.Username)
	if err != nil {
		return zero, err
	}
	return r.repo.AddTeamMember(ctx, request.TeamID, username)
}

func (r *Resource) RemoveTeamMember(ctx context.Context, request *TeamMemberRemoveRequest) (response.Response[TeamDetail], error) {
	var zero response.Response[TeamDetail]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireTeamID(request.TeamID); err != nil {
		return zero, err
	}
	if request.UserID <= 0 {
		return zero, apperrors.BadRequest(400, "不是团队成员")
	}
	return r.repo.RemoveTeamMember(ctx, request.TeamID, request.UserID)
}

func (r *Resource) LeaveTeam(ctx context.Context, request *TeamLeaveRequest) (response.Response[TeamResponse], error) {
	var zero response.Response[TeamResponse]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireTeamID(request.TeamID); err != nil {
		return zero, err
	}
	return r.repo.LeaveTeam(ctx, request.TeamID)
}

func requireTeamID(id int64) error {
	if id <= 0 {
		return apperrors.NotFound(404, "团队不存在")
	}
	return nil
}
