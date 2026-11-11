package document

import (
	"context"
	"unicode/utf8"

	apperrors "yunjian/internal/platform/errors"
	"yunjian/internal/platform/response"
)

// Resource 执行文档权限、内容规则和版本规则。
type Resource struct {
	repo *Repository
}

func NewResource(repo *Repository) *Resource {
	return &Resource{repo: repo}
}

func (r *Resource) Create(ctx context.Context, request *DocumentCreateRequest) (response.Response[DocumentResponse], error) {
	var zero response.Response[DocumentResponse]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if request.ID != 0 {
		return zero, apperrors.BadRequest(400, "创建文档不能指定标识")
	}
	if !validKind(request.Kind) {
		return zero, apperrors.BadRequest(400, "文档类型无效")
	}
	title, err := normalizeTitle(request.Title)
	if err != nil {
		return zero, err
	}
	request.Title = title
	return r.repo.Create(ctx, request)
}

func (r *Resource) Update(ctx context.Context, request *DocumentUpdateRequest) (response.Response[DocumentResponse], error) {
	var zero response.Response[DocumentResponse]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireID(request.ID); err != nil {
		return zero, err
	}
	if err := requireRevision(request.Revision); err != nil {
		return zero, err
	}
	title, err := normalizeTitle(request.Title)
	if err != nil {
		return zero, err
	}
	request.Title = title
	return r.repo.Update(ctx, request)
}

func (r *Resource) One(ctx context.Context, request *DocumentIDRequest) (response.Response[DocumentResponse], error) {
	var zero response.Response[DocumentResponse]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireID(request.ID); err != nil {
		return zero, err
	}
	return r.repo.One(ctx, request)
}

func (r *Resource) Search(ctx context.Context, search *DocumentSearch) (response.PageResponse[DocumentSimple], error) {
	var zero response.PageResponse[DocumentSimple]
	if search == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	search.Title = trimSpace(search.Title)
	if utf8.RuneCountInString(search.Title) > 100 {
		return zero, apperrors.BadRequest(400, "搜索词过长")
	}
	if search.Kind != "" && !validKind(search.Kind) {
		return zero, apperrors.BadRequest(400, "文档类型无效")
	}
	page, pageSize, err := normalizePage(search.Page, search.PageSize)
	if err != nil {
		return zero, err
	}
	if _, _, err := normalizeSort(search.Sort, search.Order); err != nil {
		return zero, err
	}
	search.Page = page
	search.PageSize = pageSize
	return r.repo.Search(ctx, search)
}

func (r *Resource) Delete(ctx context.Context, request *DocumentIDRequest) (response.Response[DocumentResponse], error) {
	var zero response.Response[DocumentResponse]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireID(request.ID); err != nil {
		return zero, err
	}
	return r.repo.Delete(ctx, request)
}

func (r *Resource) Save(ctx context.Context, request *DocumentSaveRequest) (response.Response[DocumentSaveResponse], error) {
	var zero response.Response[DocumentSaveResponse]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireID(request.ID); err != nil {
		return zero, err
	}
	if err := requireRevision(request.Revision); err != nil {
		return zero, err
	}
	title, err := normalizeTitle(request.Title)
	if err != nil {
		return zero, err
	}
	note, err := normalizeNote(request.Note)
	if err != nil {
		return zero, err
	}
	request.Title = title
	request.Note = note
	return r.repo.Save(ctx, request)
}

func (r *Resource) VersionSearch(ctx context.Context, search *DocumentVersionSearch) (response.PageResponse[DocumentVersionSimple], error) {
	var zero response.PageResponse[DocumentVersionSimple]
	if search == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireID(search.DocumentID); err != nil {
		return zero, err
	}
	page, pageSize, err := normalizePage(search.Page, search.PageSize)
	if err != nil {
		return zero, err
	}
	search.Page = page
	search.PageSize = pageSize
	return r.repo.VersionSearch(ctx, search)
}

func (r *Resource) VersionOne(ctx context.Context, request *DocumentVersionRequest) (response.Response[DocumentVersionResponse], error) {
	var zero response.Response[DocumentVersionResponse]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireID(request.DocumentID); err != nil {
		return zero, err
	}
	if request.VersionID <= 0 {
		return zero, apperrors.BadRequest(400, "版本标识无效")
	}
	return r.repo.VersionOne(ctx, request)
}

func (r *Resource) Restore(ctx context.Context, request *DocumentRestoreRequest) (response.Response[DocumentSaveResponse], error) {
	var zero response.Response[DocumentSaveResponse]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireID(request.ID); err != nil {
		return zero, err
	}
	if request.VersionID <= 0 {
		return zero, apperrors.BadRequest(400, "版本标识无效")
	}
	if err := requireRevision(request.Revision); err != nil {
		return zero, err
	}
	note, err := normalizeNote(request.Note)
	if err != nil {
		return zero, err
	}
	request.Note = note
	return r.repo.Restore(ctx, request)
}
