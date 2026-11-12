package document

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	apperrors "yunjian/internal/platform/errors"
	"yunjian/internal/platform/response"
)

const (
	maxFolders     = 200
	maxFolderDepth = 8
)

func (r *Repository) CreateFolder(ctx context.Context, request *FolderCreateRequest) (response.Response[FolderDetail], error) {
	var result response.Response[FolderDetail]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if request.ParentID != 0 {
			depth, err := folderDepth(tx, scope.ActorID, request.ParentID)
			if err != nil {
				return err
			}
			if depth >= maxFolderDepth {
				return apperrors.BadRequest(400, "文件夹层级过深")
			}
		}
		var total int64
		if err := tx.Model(&Folder{}).Where("owner_id = ?", scope.ActorID).Count(&total).Error; err != nil {
			return apperrors.WrapInternal(err)
		}
		if total >= maxFolders {
			return apperrors.BadRequest(400, "文件夹数量已达上限")
		}
		taken, err := folderNameTaken(tx, scope.ActorID, request.ParentID, request.Name, 0)
		if err != nil {
			return err
		}
		if taken {
			return apperrors.BadRequest(400, "同一位置已有同名文件夹")
		}
		now := time.Now().UTC()
		folder := &Folder{OwnerID: scope.ActorID, Name: request.Name, CreatedAt: now, UpdatedAt: now}
		if request.ParentID != 0 {
			parent := request.ParentID
			folder.ParentID = &parent
		}
		if err := tx.Create(folder).Error; err != nil {
			return apperrors.WrapInternal(err)
		}
		detail, err := folderDetail(tx, scope.ActorID, folder.ID)
		if err != nil {
			return err
		}
		result = response.OK(*detail)
		return nil
	})
	if err != nil {
		return response.Response[FolderDetail]{}, err
	}
	return result, nil
}

func (r *Repository) UpdateFolder(ctx context.Context, request *FolderUpdateRequest) (response.Response[FolderDetail], error) {
	var result response.Response[FolderDetail]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		folder, err := loadOwnedFolder(tx, request.ID, scope.ActorID)
		if err != nil {
			return err
		}
		parentID := valueOrZero(folder.ParentID)
		taken, err := folderNameTaken(tx, scope.ActorID, parentID, request.Name, folder.ID)
		if err != nil {
			return err
		}
		if taken {
			return apperrors.BadRequest(400, "同一位置已有同名文件夹")
		}
		now := time.Now().UTC()
		res := tx.Model(&Folder{}).Where("id = ? AND owner_id = ?", folder.ID, scope.ActorID).Updates(map[string]any{
			"name":       request.Name,
			"updated_at": now,
		})
		if res.Error != nil {
			return apperrors.WrapInternal(res.Error)
		}
		if res.RowsAffected != 1 {
			return apperrors.NotFound(404, "文件夹不存在")
		}
		detail, err := folderDetail(tx, scope.ActorID, folder.ID)
		if err != nil {
			return err
		}
		result = response.OK(*detail)
		return nil
	})
	if err != nil {
		return response.Response[FolderDetail]{}, err
	}
	return result, nil
}

func (r *Repository) DeleteFolder(ctx context.Context, request *FolderIDRequest) (response.Response[FolderSimple], error) {
	var result response.Response[FolderSimple]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		folder, err := loadOwnedFolder(tx, request.ID, scope.ActorID)
		if err != nil {
			return err
		}
		var documents int64
		if err := tx.Model(&Document{}).Where("owner_id = ? AND folder_id = ?", scope.ActorID, folder.ID).Count(&documents).Error; err != nil {
			return apperrors.WrapInternal(err)
		}
		var children int64
		if err := tx.Model(&Folder{}).Where("owner_id = ? AND parent_id = ?", scope.ActorID, folder.ID).Count(&children).Error; err != nil {
			return apperrors.WrapInternal(err)
		}
		if documents > 0 || children > 0 {
			return apperrors.BadRequest(400, "请先移走文件夹里的内容")
		}
		res := tx.Where("id = ? AND owner_id = ?", folder.ID, scope.ActorID).Delete(&Folder{})
		if res.Error != nil {
			return apperrors.WrapInternal(res.Error)
		}
		if res.RowsAffected != 1 {
			return apperrors.NotFound(404, "文件夹不存在")
		}
		result = response.OK(*folderToSimple(folder))
		return nil
	})
	if err != nil {
		return response.Response[FolderSimple]{}, err
	}
	return result, nil
}

func (r *Repository) FolderOne(ctx context.Context, request *FolderIDRequest) (response.Response[FolderDetail], error) {
	var result response.Response[FolderDetail]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	detail, err := folderDetail(r.db.WithContext(ctx), scope.ActorID, request.ID)
	if err != nil {
		return result, err
	}
	return response.OK(*detail), nil
}

func (r *Repository) SearchFolders(ctx context.Context, search *FolderSearch) (response.PageResponse[FolderSimple], error) {
	var result response.PageResponse[FolderSimple]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	page, pageSize, err := normalizePage(search.Page, search.PageSize)
	if err != nil {
		return result, err
	}
	filtered := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&Folder{}).Where("owner_id = ?", scope.ActorID)
		if search.ParentID != nil {
			if *search.ParentID == 0 {
				query = query.Where("parent_id IS NULL")
			} else {
				query = query.Where("parent_id = ?", *search.ParentID)
			}
		}
		if search.Name != "" {
			query = query.Where("name LIKE ? ESCAPE ?", likeContains(search.Name), `\`)
		}
		return query
	}
	var total int64
	if err := filtered().Count(&total).Error; err != nil {
		return result, apperrors.WrapInternal(err)
	}
	rows := make([]Folder, 0)
	offset := (page - 1) * pageSize
	err = filtered().Order("name ASC, id ASC").Offset(offset).Limit(pageSize).Find(&rows).Error
	if err != nil {
		return result, apperrors.WrapInternal(err)
	}
	items := make([]FolderSimple, 0, len(rows))
	for i := range rows {
		items = append(items, *folderToSimple(&rows[i]))
	}
	return response.PageResponse[FolderSimple]{
		Data:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (r *Repository) Move(ctx context.Context, request *DocumentMoveRequest) (response.Response[DocumentResponse], error) {
	var result response.Response[DocumentResponse]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		doc, err := loadOwned(tx, request.ID, scope.ActorID)
		if err != nil {
			return err
		}
		if doc.Revision != request.Revision {
			return apperrors.Conflict(10004, "文档已被其他编辑更新，请刷新后重试")
		}
		if err := ensureFolder(tx, scope.ActorID, request.FolderID); err != nil {
			return err
		}
		if valueOrZero(doc.FolderID) == request.FolderID {
			row, err := loadDetail(tx, doc.ID, scope.ActorID)
			if err != nil {
				return err
			}
			result = response.OK(*documentToResponse(row))
			return nil
		}
		now := time.Now().UTC()
		updates := map[string]any{
			"revision":   doc.Revision + 1,
			"updated_at": now,
		}
		if request.FolderID == 0 {
			updates["folder_id"] = gorm.Expr("NULL")
		} else {
			updates["folder_id"] = request.FolderID
		}
		res := tx.Model(&Document{}).
			Where("id = ? AND owner_id = ? AND revision = ?", doc.ID, scope.ActorID, doc.Revision).
			Updates(updates)
		if res.Error != nil {
			return apperrors.WrapInternal(res.Error)
		}
		if res.RowsAffected != 1 {
			return apperrors.Conflict(10004, "文档已被其他编辑更新，请刷新后重试")
		}
		row, err := loadDetail(tx, doc.ID, scope.ActorID)
		if err != nil {
			return err
		}
		result = response.OK(*documentToResponse(row))
		return nil
	})
	if err != nil {
		return response.Response[DocumentResponse]{}, err
	}
	return result, nil
}

func ensureFolder(tx *gorm.DB, ownerID, folderID int64) error {
	if folderID == 0 {
		return nil
	}
	_, err := loadOwnedFolder(tx, folderID, ownerID)
	return err
}

func loadOwnedFolder(tx *gorm.DB, id, ownerID int64) (*Folder, error) {
	var folder Folder
	err := tx.Where("id = ? AND owner_id = ?", id, ownerID).Take(&folder).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.NotFound(404, "文件夹不存在")
	}
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return &folder, nil
}

func folderDepth(tx *gorm.DB, ownerID, id int64) (int, error) {
	depth := 0
	current := id
	seen := map[int64]struct{}{}
	for current != 0 {
		if _, ok := seen[current]; ok {
			return 0, apperrors.BadRequest(400, "文件夹层级无效")
		}
		seen[current] = struct{}{}
		folder, err := loadOwnedFolder(tx, current, ownerID)
		if err != nil {
			return 0, err
		}
		depth++
		if folder.ParentID == nil {
			break
		}
		current = *folder.ParentID
		if depth > maxFolderDepth+1 {
			break
		}
	}
	return depth, nil
}

func folderNameTaken(tx *gorm.DB, ownerID, parentID int64, name string, excludeID int64) (bool, error) {
	query := tx.Model(&Folder{}).Where("owner_id = ? AND name = ?", ownerID, name)
	if parentID == 0 {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", parentID)
	}
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return false, apperrors.WrapInternal(err)
	}
	return total > 0, nil
}

func folderDetail(tx *gorm.DB, ownerID, id int64) (*FolderDetail, error) {
	chain := make([]Folder, 0, maxFolderDepth)
	current := id
	seen := map[int64]struct{}{}
	for current != 0 {
		if _, ok := seen[current]; ok {
			return nil, apperrors.BadRequest(400, "文件夹层级无效")
		}
		seen[current] = struct{}{}
		folder, err := loadOwnedFolder(tx, current, ownerID)
		if err != nil {
			return nil, err
		}
		chain = append(chain, *folder)
		if folder.ParentID == nil || len(chain) > maxFolderDepth {
			break
		}
		current = *folder.ParentID
	}
	if len(chain) == 0 {
		return nil, apperrors.NotFound(404, "文件夹不存在")
	}
	ancestors := make([]FolderSimple, 0, len(chain)-1)
	for i := len(chain) - 1; i >= 1; i-- {
		ancestors = append(ancestors, *folderToSimple(&chain[i]))
	}
	detail := &FolderDetail{FolderSimple: *folderToSimple(&chain[0]), Ancestors: ancestors}
	return detail, nil
}

func folderToSimple(folder *Folder) *FolderSimple {
	return &FolderSimple{
		ID:        folder.ID,
		ParentID:  valueOrZero(folder.ParentID),
		Name:      folder.Name,
		UpdatedAt: folder.UpdatedAt,
	}
}
