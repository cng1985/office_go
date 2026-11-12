package account

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"yunjian/internal/platform/domain"
	apperrors "yunjian/internal/platform/errors"
	"yunjian/internal/platform/response"
)

const (
	roleOwner      = "owner"
	roleMember     = "member"
	maxOwnedTeams  = 20
	maxTeamMembers = 50
)

func (r *Repository) CreateTeam(ctx context.Context, name string) (response.Response[TeamDetail], error) {
	var result response.Response[TeamDetail]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var owned int64
		if err := tx.Model(&Team{}).Where("owner_id = ?", scope.ActorID).Count(&owned).Error; err != nil {
			return apperrors.WrapInternal(err)
		}
		if owned >= maxOwnedTeams {
			return apperrors.BadRequest(400, "创建的团队数量已达上限")
		}
		now := time.Now().UTC()
		team := &Team{OwnerID: scope.ActorID, Name: name, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(team).Error; err != nil {
			return apperrors.WrapInternal(err)
		}
		member := &TeamMember{TeamID: team.ID, UserID: scope.ActorID, Role: roleOwner, CreatedAt: now}
		if err := tx.Create(member).Error; err != nil {
			return apperrors.WrapInternal(err)
		}
		detail, err := teamDetail(tx, team, member)
		if err != nil {
			return err
		}
		result = response.OK(*detail)
		return nil
	})
	if err != nil {
		return response.Response[TeamDetail]{}, err
	}
	return result, nil
}

func (r *Repository) UpdateTeam(ctx context.Context, request *TeamUpdateRequest) (response.Response[TeamDetail], error) {
	var result response.Response[TeamDetail]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		team, member, err := loadTeam(tx, request.ID, scope.ActorID)
		if err != nil {
			return err
		}
		if member.Role != roleOwner {
			return apperrors.Forbidden(403, "只有团队所有者可以修改团队")
		}
		now := time.Now().UTC()
		res := tx.Model(&Team{}).Where("id = ? AND owner_id = ?", team.ID, scope.ActorID).Updates(map[string]any{
			"name":       request.Name,
			"updated_at": now,
		})
		if res.Error != nil {
			return apperrors.WrapInternal(res.Error)
		}
		if res.RowsAffected != 1 {
			return apperrors.NotFound(404, "团队不存在")
		}
		team.Name = request.Name
		team.UpdatedAt = now
		detail, err := teamDetail(tx, team, member)
		if err != nil {
			return err
		}
		result = response.OK(*detail)
		return nil
	})
	if err != nil {
		return response.Response[TeamDetail]{}, err
	}
	return result, nil
}

func (r *Repository) DeleteTeam(ctx context.Context, teamID int64) (response.Response[TeamResponse], error) {
	var result response.Response[TeamResponse]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		team, member, err := loadTeam(tx, teamID, scope.ActorID)
		if err != nil {
			return err
		}
		if member.Role != roleOwner {
			return apperrors.Forbidden(403, "只有团队所有者可以修改团队")
		}
		busy, err := teamHasFiles(tx, team.ID)
		if err != nil {
			return err
		}
		if busy {
			return apperrors.BadRequest(400, "请先清空团队里的文档和文件夹")
		}
		if err := tx.Where("team_id = ?", team.ID).Delete(&TeamMember{}).Error; err != nil {
			return apperrors.WrapInternal(err)
		}
		res := tx.Where("id = ? AND owner_id = ?", team.ID, scope.ActorID).Delete(&Team{})
		if res.Error != nil {
			return apperrors.WrapInternal(res.Error)
		}
		if res.RowsAffected != 1 {
			return apperrors.NotFound(404, "团队不存在")
		}
		result = response.OK(teamResponse(team, member.Role))
		return nil
	})
	if err != nil {
		return response.Response[TeamResponse]{}, err
	}
	return result, nil
}

func (r *Repository) SearchTeams(ctx context.Context, page, pageSize int) (response.PageResponse[TeamResponse], error) {
	var result response.PageResponse[TeamResponse]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	filtered := func() *gorm.DB {
		return r.db.WithContext(ctx).Table("account_team AS t").
			Joins("JOIN account_team_member AS m ON m.team_id = t.id AND m.user_id = ?", scope.ActorID)
	}
	var total int64
	if err := filtered().Count(&total).Error; err != nil {
		return result, apperrors.WrapInternal(err)
	}
	rows := make([]teamListRow, 0)
	offset := (page - 1) * pageSize
	err = filtered().
		Select("t.id, t.owner_id, t.name, t.created_at, m.role").
		Order("t.name ASC, t.id ASC").
		Offset(offset).Limit(pageSize).
		Scan(&rows).Error
	if err != nil {
		return result, apperrors.WrapInternal(err)
	}
	items := make([]TeamResponse, 0, len(rows))
	for i := range rows {
		items = append(items, TeamResponse{
			ID:        rows[i].ID,
			Name:      rows[i].Name,
			OwnerID:   rows[i].OwnerID,
			Role:      rows[i].Role,
			CreatedAt: rows[i].CreatedAt,
		})
	}
	return response.PageResponse[TeamResponse]{
		Data:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (r *Repository) TeamOne(ctx context.Context, teamID int64) (response.Response[TeamDetail], error) {
	var result response.Response[TeamDetail]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	team, member, err := loadTeam(r.db.WithContext(ctx), teamID, scope.ActorID)
	if err != nil {
		return result, err
	}
	detail, err := teamDetail(r.db.WithContext(ctx), team, member)
	if err != nil {
		return result, err
	}
	return response.OK(*detail), nil
}

func (r *Repository) AddTeamMember(ctx context.Context, teamID int64, username string) (response.Response[TeamDetail], error) {
	var result response.Response[TeamDetail]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		team, member, err := loadTeam(tx, teamID, scope.ActorID)
		if err != nil {
			return err
		}
		if member.Role != roleOwner {
			return apperrors.Forbidden(403, "只有团队所有者可以修改团队")
		}
		var user User
		err = tx.Where("username = ?", username).Take(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.BadRequest(400, "用户不存在")
		}
		if err != nil {
			return apperrors.WrapInternal(err)
		}
		var existing int64
		if err := tx.Model(&TeamMember{}).Where("team_id = ? AND user_id = ?", team.ID, user.ID).Count(&existing).Error; err != nil {
			return apperrors.WrapInternal(err)
		}
		if existing > 0 {
			return apperrors.BadRequest(400, "已经是团队成员")
		}
		var total int64
		if err := tx.Model(&TeamMember{}).Where("team_id = ?", team.ID).Count(&total).Error; err != nil {
			return apperrors.WrapInternal(err)
		}
		if total >= maxTeamMembers {
			return apperrors.BadRequest(400, "团队成员已达上限")
		}
		now := time.Now().UTC()
		if err := tx.Create(&TeamMember{TeamID: team.ID, UserID: user.ID, Role: roleMember, CreatedAt: now}).Error; err != nil {
			return mapWriteError(err)
		}
		detail, err := teamDetail(tx, team, member)
		if err != nil {
			return err
		}
		result = response.OK(*detail)
		return nil
	})
	if err != nil {
		return response.Response[TeamDetail]{}, err
	}
	return result, nil
}

func (r *Repository) RemoveTeamMember(ctx context.Context, teamID, userID int64) (response.Response[TeamDetail], error) {
	var result response.Response[TeamDetail]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		team, member, err := loadTeam(tx, teamID, scope.ActorID)
		if err != nil {
			return err
		}
		if member.Role != roleOwner {
			return apperrors.Forbidden(403, "只有团队所有者可以修改团队")
		}
		if userID == team.OwnerID {
			return apperrors.BadRequest(400, "不能移出团队所有者")
		}
		res := tx.Where("team_id = ? AND user_id = ? AND role <> ?", team.ID, userID, roleOwner).Delete(&TeamMember{})
		if res.Error != nil {
			return apperrors.WrapInternal(res.Error)
		}
		if res.RowsAffected != 1 {
			return apperrors.BadRequest(400, "不是团队成员")
		}
		detail, err := teamDetail(tx, team, member)
		if err != nil {
			return err
		}
		result = response.OK(*detail)
		return nil
	})
	if err != nil {
		return response.Response[TeamDetail]{}, err
	}
	return result, nil
}

func (r *Repository) LeaveTeam(ctx context.Context, teamID int64) (response.Response[TeamResponse], error) {
	var result response.Response[TeamResponse]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		team, member, err := loadTeam(tx, teamID, scope.ActorID)
		if err != nil {
			return err
		}
		if member.Role == roleOwner {
			return apperrors.BadRequest(400, "所有者不能退出，请删除团队")
		}
		res := tx.Where("team_id = ? AND user_id = ? AND role <> ?", team.ID, scope.ActorID, roleOwner).Delete(&TeamMember{})
		if res.Error != nil {
			return apperrors.WrapInternal(res.Error)
		}
		if res.RowsAffected != 1 {
			return apperrors.NotFound(404, "团队不存在")
		}
		result = response.OK(teamResponse(team, member.Role))
		return nil
	})
	if err != nil {
		return response.Response[TeamResponse]{}, err
	}
	return result, nil
}

type teamListRow struct {
	ID        int64
	OwnerID   int64
	Name      string
	Role      string
	CreatedAt time.Time
}

type memberRow struct {
	UserID      int64
	Username    string
	DisplayName string
	Role        string
	CreatedAt   time.Time
}

func loadTeam(tx *gorm.DB, teamID, actorID int64) (*Team, *TeamMember, error) {
	if teamID <= 0 {
		return nil, nil, apperrors.NotFound(404, "团队不存在")
	}
	var team Team
	err := tx.Where("id = ?", teamID).Take(&team).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, apperrors.NotFound(404, "团队不存在")
	}
	if err != nil {
		return nil, nil, apperrors.WrapInternal(err)
	}
	var member TeamMember
	err = tx.Where("team_id = ? AND user_id = ?", team.ID, actorID).Take(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, apperrors.NotFound(404, "团队不存在")
	}
	if err != nil {
		return nil, nil, apperrors.WrapInternal(err)
	}
	return &team, &member, nil
}

func teamHasFiles(tx *gorm.DB, teamID int64) (bool, error) {
	var documents int64
	if err := tx.Table("office_document").Where("team_id = ?", teamID).Count(&documents).Error; err != nil {
		return false, apperrors.WrapInternal(err)
	}
	var folders int64
	if err := tx.Table("office_folder").Where("team_id = ?", teamID).Count(&folders).Error; err != nil {
		return false, apperrors.WrapInternal(err)
	}
	return documents+folders > 0, nil
}

func teamDetail(tx *gorm.DB, team *Team, actor *TeamMember) (*TeamDetail, error) {
	members, err := listMembers(tx, team.ID)
	if err != nil {
		return nil, err
	}
	detail := &TeamDetail{TeamResponse: teamResponse(team, actor.Role), Members: members}
	return detail, nil
}

func teamResponse(team *Team, role string) TeamResponse {
	return TeamResponse{
		ID:        team.ID,
		Name:      team.Name,
		OwnerID:   team.OwnerID,
		Role:      role,
		CreatedAt: team.CreatedAt,
	}
}

func listMembers(tx *gorm.DB, teamID int64) ([]TeamMemberResponse, error) {
	rows := make([]memberRow, 0)
	err := tx.Table("account_team_member AS m").
		Select("m.user_id, u.username, u.display_name, m.role, m.created_at").
		Joins("JOIN account_user AS u ON u.id = m.user_id").
		Where("m.team_id = ?", teamID).
		Order("CASE m.role WHEN 'owner' THEN 0 ELSE 1 END, u.username ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	items := make([]TeamMemberResponse, 0, len(rows))
	for i := range rows {
		items = append(items, TeamMemberResponse{
			UserID:      rows[i].UserID,
			Username:    rows[i].Username,
			DisplayName: rows[i].DisplayName,
			Role:        rows[i].Role,
			CreatedAt:   rows[i].CreatedAt,
		})
	}
	return items, nil
}

func normalizeTeamPage(page, pageSize int) (int, int, error) {
	normalizedPage, normalizedSize, err := domain.NormalizePage(page, pageSize)
	if err != nil {
		return 0, 0, apperrors.BadRequest(400, "分页参数无效")
	}
	return normalizedPage, normalizedSize, nil
}
