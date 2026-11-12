package document

import (
	"errors"

	"gorm.io/gorm"

	apperrors "yunjian/internal/platform/errors"
)

// 团队成员表由账号模块维护。文档模块只按表名核对成员，避免两个业务包互相引用。
func requireTeam(tx *gorm.DB, teamID, actorID int64) error {
	if teamID == 0 {
		return nil
	}
	if teamID < 0 {
		return apperrors.NotFound(404, "团队不存在")
	}
	var total int64
	err := tx.Table("account_team_member").Where("team_id = ? AND user_id = ?", teamID, actorID).Count(&total).Error
	if err != nil {
		return apperrors.WrapInternal(err)
	}
	if total != 1 {
		return apperrors.NotFound(404, "团队不存在")
	}
	return nil
}

func applySpace(query *gorm.DB, alias string, actorID, teamID int64) *gorm.DB {
	ownerCol := "owner_id"
	teamCol := "team_id"
	if alias != "" {
		ownerCol = alias + ".owner_id"
		teamCol = alias + ".team_id"
	}
	if teamID == 0 {
		return query.Where(ownerCol+" = ? AND "+teamCol+" IS NULL", actorID)
	}
	return query.Where(teamCol+" = ?", teamID)
}

func allowDocument(tx *gorm.DB, doc *Document, actorID int64) error {
	if doc.TeamID == nil || *doc.TeamID == 0 {
		if doc.OwnerID != actorID {
			return apperrors.NotFound(404, "文档不存在")
		}
		return nil
	}
	err := requireTeam(tx, *doc.TeamID, actorID)
	if err == nil {
		return nil
	}
	var appErr *apperrors.Error
	if errors.As(err, &appErr) && appErr.Kind == apperrors.KindNotFound {
		return apperrors.NotFound(404, "文档不存在")
	}
	return err
}

func allowFolder(tx *gorm.DB, folder *Folder, actorID int64) error {
	if folder.TeamID == nil || *folder.TeamID == 0 {
		if folder.OwnerID != actorID {
			return apperrors.NotFound(404, "文件夹不存在")
		}
		return nil
	}
	err := requireTeam(tx, *folder.TeamID, actorID)
	if err == nil {
		return nil
	}
	var appErr *apperrors.Error
	if errors.As(err, &appErr) && appErr.Kind == apperrors.KindNotFound {
		return apperrors.NotFound(404, "文件夹不存在")
	}
	return err
}

func guardDocument(tx *gorm.DB, doc *Document, actorID int64) *gorm.DB {
	query := tx.Model(&Document{}).Where("id = ?", doc.ID)
	if doc.TeamID == nil || *doc.TeamID == 0 {
		return query.Where("owner_id = ? AND team_id IS NULL", doc.OwnerID)
	}
	return query.Where(
		"team_id = ? AND EXISTS (SELECT 1 FROM account_team_member WHERE team_id = ? AND user_id = ?)",
		*doc.TeamID, *doc.TeamID, actorID,
	)
}

func guardFolder(tx *gorm.DB, folder *Folder, actorID int64) *gorm.DB {
	query := tx.Model(&Folder{}).Where("id = ?", folder.ID)
	if folder.TeamID == nil || *folder.TeamID == 0 {
		return query.Where("owner_id = ? AND team_id IS NULL", folder.OwnerID)
	}
	return query.Where(
		"team_id = ? AND EXISTS (SELECT 1 FROM account_team_member WHERE team_id = ? AND user_id = ?)",
		*folder.TeamID, *folder.TeamID, actorID,
	)
}
