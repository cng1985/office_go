package document

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"yunjian/internal/platform/domain"
	apperrors "yunjian/internal/platform/errors"
	"yunjian/internal/platform/response"
)

// Repository 持久化文档聚合。一次保存或恢复只在一个事务里提交。
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type detailRow struct {
	ID        int64
	OwnerID   int64
	TeamID    *int64
	FolderID  *int64
	Kind      string
	Title     string
	Revision  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	VersionID int64
	VersionNo int64
	Content   string
	ByteSize  int64
	Note      string
}

func (r *Repository) Create(ctx context.Context, request *DocumentCreateRequest) (response.Response[DocumentResponse], error) {
	var zero response.Response[DocumentResponse]
	content, err := Blank(request.Kind)
	if err != nil {
		return zero, err
	}
	return r.insertNew(ctx, request.Kind, request.Title, request.FolderID, request.TeamID, content, "创建文档")
}

func (r *Repository) Import(ctx context.Context, kind, title string, folderID, teamID int64, content []byte) (response.Response[DocumentResponse], error) {
	return r.insertNew(ctx, kind, title, folderID, teamID, content, "上传文件")
}

func (r *Repository) insertNew(ctx context.Context, kind, title string, folderID, teamID int64, content []byte, note string) (response.Response[DocumentResponse], error) {
	var result response.Response[DocumentResponse]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := requireTeam(tx, teamID, scope.ActorID); err != nil {
			return err
		}
		if err := ensureFolderInSpace(tx, scope.ActorID, teamID, folderID); err != nil {
			return err
		}
		entity := &Document{
			OwnerID:   scope.ActorID,
			Kind:      kind,
			Title:     title,
			Revision:  1,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if folderID != 0 {
			entity.FolderID = &folderID
		}
		if teamID != 0 {
			entity.TeamID = &teamID
		}
		if err := tx.Omit(clause.Associations).Create(entity).Error; err != nil {
			return mapWriteError(err)
		}
		if _, err := insertVersion(tx, entity, content, note, scope.ActorID, now); err != nil {
			return err
		}
		row, err := loadDetail(tx, entity.ID, scope.ActorID)
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

func (r *Repository) Update(ctx context.Context, request *DocumentUpdateRequest) (response.Response[DocumentResponse], error) {
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
		now := time.Now().UTC()
		res := guardDocument(tx, doc, scope.ActorID).
			Where("revision = ?", doc.Revision).
			Updates(map[string]any{
				"title":      request.Title,
				"revision":   doc.Revision + 1,
				"updated_at": now,
			})
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

func (r *Repository) One(ctx context.Context, request *DocumentIDRequest) (response.Response[DocumentResponse], error) {
	var result response.Response[DocumentResponse]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	row, err := loadDetail(r.db.WithContext(ctx), request.ID, scope.ActorID)
	if err != nil {
		return result, err
	}
	return response.OK(*documentToResponse(row)), nil
}

func (r *Repository) Search(ctx context.Context, search *DocumentSearch) (response.PageResponse[DocumentSimple], error) {
	var result response.PageResponse[DocumentSimple]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	page, pageSize, err := normalizePage(search.Page, search.PageSize)
	if err != nil {
		return result, err
	}
	column, direction, err := normalizeSort(search.Sort, search.Order)
	if err != nil {
		return result, err
	}
	if err := requireTeam(r.db.WithContext(ctx), search.TeamID, scope.ActorID); err != nil {
		return result, err
	}
	filtered := func() *gorm.DB {
		query := applySpace(r.db.WithContext(ctx).Table("office_document AS d"), "d", scope.ActorID, search.TeamID)
		if search.Kind != "" {
			query = query.Where("d.kind = ?", search.Kind)
		}
		if search.Title != "" {
			query = query.Where("d.title LIKE ? ESCAPE ?", likeContains(search.Title), `\`)
		}
		if search.FolderID != nil {
			if *search.FolderID == 0 {
				query = query.Where("d.folder_id IS NULL")
			} else {
				query = query.Where("d.folder_id = ?", *search.FolderID)
			}
		}
		return query
	}
	var total int64
	if err := filtered().Count(&total).Error; err != nil {
		return result, apperrors.WrapInternal(err)
	}
	rows := make([]detailRow, 0)
	offset := (page - 1) * pageSize
	err = filtered().
		Select("d.id, d.folder_id, d.kind, d.title, d.revision, d.updated_at, v.version_no").
		Joins("JOIN office_document_version AS v ON v.id = d.current_version_id").
		Order(column + " " + direction + ", d.id DESC").
		Offset(offset).Limit(pageSize).
		Scan(&rows).Error
	if err != nil {
		return result, apperrors.WrapInternal(err)
	}
	items := make([]DocumentSimple, 0, len(rows))
	for i := range rows {
		items = append(items, *documentToSimple(&rows[i]))
	}
	return response.PageResponse[DocumentSimple]{
		Data:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (r *Repository) Delete(ctx context.Context, request *DocumentIDRequest) (response.Response[DocumentResponse], error) {
	var result response.Response[DocumentResponse]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := loadDetail(tx, request.ID, scope.ActorID)
		if err != nil {
			return err
		}
		if err := tx.Where("document_id = ?", row.ID).Delete(&DocumentVersion{}).Error; err != nil {
			return apperrors.WrapInternal(err)
		}
		res := guardDocument(tx, &Document{ID: row.ID, OwnerID: row.OwnerID, TeamID: row.TeamID}, scope.ActorID).Delete(&Document{})
		if res.Error != nil {
			return apperrors.WrapInternal(res.Error)
		}
		if res.RowsAffected != 1 {
			return apperrors.NotFound(404, "文档不存在")
		}
		saved := *documentToResponse(row)
		saved.Content = nil
		result = response.OK(saved)
		return nil
	})
	if err != nil {
		return response.Response[DocumentResponse]{}, err
	}
	return result, nil
}

func (r *Repository) Save(ctx context.Context, request *DocumentSaveRequest) (response.Response[DocumentSaveResponse], error) {
	var result response.Response[DocumentSaveResponse]
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
		canonical, err := Canonical(doc.Kind, request.Content)
		if err != nil {
			return err
		}
		var current DocumentVersion
		if err := tx.Where("id = ? AND document_id = ?", doc.CurrentVersionID, doc.ID).Take(&current).Error; err != nil {
			return apperrors.WrapInternal(err)
		}
		digest := hashContent(canonical)
		if digest == current.ContentSHA && doc.Title == request.Title && request.Note == "" {
			row, err := loadDetail(tx, doc.ID, scope.ActorID)
			if err != nil {
				return err
			}
			result = response.OK(DocumentSaveResponse{Document: *documentToResponse(row), NewVersion: false})
			return nil
		}
		now := time.Now().UTC()
		res := guardDocument(tx, doc, scope.ActorID).
			Where("revision = ?", doc.Revision).
			Updates(map[string]any{
				"title":      request.Title,
				"revision":   doc.Revision + 1,
				"updated_at": now,
			})
		if res.Error != nil {
			return apperrors.WrapInternal(res.Error)
		}
		if res.RowsAffected != 1 {
			return apperrors.Conflict(10004, "文档已被其他编辑更新，请刷新后重试")
		}
		doc.Title = request.Title
		doc.Revision++
		if _, err := insertVersion(tx, doc, canonical, request.Note, scope.ActorID, now); err != nil {
			return err
		}
		row, err := loadDetail(tx, doc.ID, scope.ActorID)
		if err != nil {
			return err
		}
		result = response.OK(DocumentSaveResponse{Document: *documentToResponse(row), NewVersion: true})
		return nil
	})
	if err != nil {
		return response.Response[DocumentSaveResponse]{}, err
	}
	return result, nil
}

func (r *Repository) VersionSearch(ctx context.Context, search *DocumentVersionSearch) (response.PageResponse[DocumentVersionSimple], error) {
	var result response.PageResponse[DocumentVersionSimple]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	page, pageSize, err := normalizePage(search.Page, search.PageSize)
	if err != nil {
		return result, err
	}
	if _, err := loadOwned(r.db.WithContext(ctx), search.DocumentID, scope.ActorID); err != nil {
		return result, err
	}
	base := func() *gorm.DB {
		return r.db.WithContext(ctx).Model(&DocumentVersion{}).Where("document_id = ?", search.DocumentID)
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return result, apperrors.WrapInternal(err)
	}
	rows := make([]DocumentVersion, 0)
	offset := (page - 1) * pageSize
	err = base().Order("version_no DESC, id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error
	if err != nil {
		return result, apperrors.WrapInternal(err)
	}
	items := make([]DocumentVersionSimple, 0, len(rows))
	for i := range rows {
		items = append(items, *versionToSimple(&rows[i]))
	}
	return response.PageResponse[DocumentVersionSimple]{
		Data:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (r *Repository) VersionOne(ctx context.Context, request *DocumentVersionRequest) (response.Response[DocumentVersionResponse], error) {
	var result response.Response[DocumentVersionResponse]
	scope, err := requireScope(ctx)
	if err != nil {
		return result, err
	}
	if _, err := loadOwned(r.db.WithContext(ctx), request.DocumentID, scope.ActorID); err != nil {
		return result, err
	}
	var version DocumentVersion
	err = r.db.WithContext(ctx).
		Where("id = ? AND document_id = ?", request.VersionID, request.DocumentID).
		Take(&version).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, apperrors.NotFound(404, "版本不存在")
	}
	if err != nil {
		return result, apperrors.WrapInternal(err)
	}
	return response.OK(*versionToResponse(&version)), nil
}

func (r *Repository) Restore(ctx context.Context, request *DocumentRestoreRequest) (response.Response[DocumentSaveResponse], error) {
	var result response.Response[DocumentSaveResponse]
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
		var source DocumentVersion
		err = tx.Where("id = ? AND document_id = ?", request.VersionID, doc.ID).Take(&source).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.NotFound(404, "版本不存在")
		}
		if err != nil {
			return apperrors.WrapInternal(err)
		}
		note := request.Note
		if note == "" {
			note = fmt.Sprintf("恢复至版本 %d", source.VersionNo)
		}
		now := time.Now().UTC()
		res := guardDocument(tx, doc, scope.ActorID).
			Where("revision = ?", doc.Revision).
			Updates(map[string]any{
				"title":      source.Title,
				"revision":   doc.Revision + 1,
				"updated_at": now,
			})
		if res.Error != nil {
			return apperrors.WrapInternal(res.Error)
		}
		if res.RowsAffected != 1 {
			return apperrors.Conflict(10004, "文档已被其他编辑更新，请刷新后重试")
		}
		doc.Title = source.Title
		doc.Revision++
		if _, err := insertVersion(tx, doc, []byte(source.Content), note, scope.ActorID, now); err != nil {
			return err
		}
		row, err := loadDetail(tx, doc.ID, scope.ActorID)
		if err != nil {
			return err
		}
		result = response.OK(DocumentSaveResponse{Document: *documentToResponse(row), NewVersion: true})
		return nil
	})
	if err != nil {
		return response.Response[DocumentSaveResponse]{}, err
	}
	return result, nil
}

func insertVersion(tx *gorm.DB, doc *Document, content []byte, note string, actorID int64, now time.Time) (*DocumentVersion, error) {
	var maxRow struct {
		Value int64 `gorm:"column:value"`
	}
	if err := tx.Raw("SELECT COALESCE(MAX(version_no), 0) AS value FROM office_document_version WHERE document_id = ?", doc.ID).Scan(&maxRow).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	maxNo := maxRow.Value
	version := &DocumentVersion{
		DocumentID: doc.ID,
		VersionNo:  maxNo + 1,
		Title:      doc.Title,
		Kind:       doc.Kind,
		Content:    string(content),
		ContentSHA: hashContent(content),
		ByteSize:   int64(len(content)),
		Note:       note,
		CreatedBy:  actorID,
		CreatedAt:  now,
	}
	if err := tx.Omit(clause.Associations).Create(version).Error; err != nil {
		return nil, mapWriteError(err)
	}
	res := tx.Model(&Document{}).
		Where("id = ? AND owner_id = ?", doc.ID, doc.OwnerID).
		Update("current_version_id", version.ID)
	if res.Error != nil {
		return nil, apperrors.WrapInternal(res.Error)
	}
	if res.RowsAffected != 1 {
		return nil, apperrors.Conflict(10004, "文档已被其他编辑更新，请刷新后重试")
	}
	doc.CurrentVersionID = version.ID
	return version, nil
}

func loadOwned(tx *gorm.DB, id, actorID int64) (*Document, error) {
	var doc Document
	err := tx.Where("id = ?", id).Take(&doc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.NotFound(404, "文档不存在")
	}
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	if err := allowDocument(tx, &doc, actorID); err != nil {
		return nil, err
	}
	return &doc, nil
}

func loadDetail(tx *gorm.DB, id, actorID int64) (*detailRow, error) {
	var row detailRow
	err := tx.Table("office_document AS d").
		Select("d.id, d.owner_id, d.team_id, d.folder_id, d.kind, d.title, d.revision, d.created_at, d.updated_at, v.id AS version_id, v.version_no, v.content, v.byte_size, v.note").
		Joins("JOIN office_document_version AS v ON v.id = d.current_version_id").
		Where("d.id = ?", id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.NotFound(404, "文档不存在")
	}
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	if err := allowDocument(tx, &Document{ID: row.ID, OwnerID: row.OwnerID, TeamID: row.TeamID}, actorID); err != nil {
		return nil, err
	}
	return &row, nil
}

func likeContains(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(value) + "%"
}

func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "unique") || strings.Contains(text, "duplicate") {
		return apperrors.Conflict(10004, "文档已被其他编辑更新，请刷新后重试")
	}
	return apperrors.WrapInternal(err)
}

func requireScope(ctx context.Context) (domain.Scope, error) {
	scope, ok := domain.ScopeFrom(ctx)
	if !ok || scope.ActorID <= 0 {
		return domain.Scope{}, apperrors.Unauthorized(401, "请先登录")
	}
	return scope, nil
}
