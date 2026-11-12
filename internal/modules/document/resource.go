package document

import (
	"context"
	"encoding/base64"
	"strings"
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
	if request.FolderID < 0 {
		return zero, apperrors.BadRequest(400, "文件夹不存在")
	}
	if err := requireNonNegativeTeam(request.TeamID); err != nil {
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
	if search.FolderID != nil && *search.FolderID < 0 {
		return zero, apperrors.BadRequest(400, "文件夹不存在")
	}
	if err := requireNonNegativeTeam(search.TeamID); err != nil {
		return zero, err
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

func (r *Resource) Import(ctx context.Context, request *DocumentImportRequest) (response.Response[DocumentResponse], error) {
	var zero response.Response[DocumentResponse]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	filename := strings.TrimSpace(request.Filename)
	if filename == "" || utf8.RuneCountInString(filename) > 255 {
		return zero, apperrors.BadRequest(400, "文件名无效")
	}
	raw, err := decodeUpload(request.ContentBase64)
	if err != nil {
		return zero, err
	}
	kind, title, content, err := ImportFile(filename, raw)
	if err != nil {
		return zero, err
	}
	title, err = normalizeTitle(title)
	if err != nil {
		return zero, err
	}
	if request.FolderID < 0 {
		return zero, apperrors.BadRequest(400, "文件夹不存在")
	}
	if err := requireNonNegativeTeam(request.TeamID); err != nil {
		return zero, err
	}
	return r.repo.Import(ctx, kind, title, request.FolderID, request.TeamID, content)
}

func (r *Resource) Move(ctx context.Context, request *DocumentMoveRequest) (response.Response[DocumentResponse], error) {
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
	if request.FolderID < 0 {
		return zero, apperrors.BadRequest(400, "文件夹不存在")
	}
	return r.repo.Move(ctx, request)
}

func (r *Resource) CreateFolder(ctx context.Context, request *FolderCreateRequest) (response.Response[FolderDetail], error) {
	var zero response.Response[FolderDetail]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if request.ParentID < 0 {
		return zero, apperrors.BadRequest(400, "文件夹不存在")
	}
	if err := requireNonNegativeTeam(request.TeamID); err != nil {
		return zero, err
	}
	name, err := normalizeFolderName(request.Name)
	if err != nil {
		return zero, err
	}
	request.Name = name
	return r.repo.CreateFolder(ctx, request)
}

func (r *Resource) UpdateFolder(ctx context.Context, request *FolderUpdateRequest) (response.Response[FolderDetail], error) {
	var zero response.Response[FolderDetail]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireID(request.ID); err != nil {
		return zero, apperrors.BadRequest(400, "文件夹不存在")
	}
	name, err := normalizeFolderName(request.Name)
	if err != nil {
		return zero, err
	}
	request.Name = name
	return r.repo.UpdateFolder(ctx, request)
}

func (r *Resource) DeleteFolder(ctx context.Context, request *FolderIDRequest) (response.Response[FolderSimple], error) {
	var zero response.Response[FolderSimple]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireID(request.ID); err != nil {
		return zero, apperrors.BadRequest(400, "文件夹不存在")
	}
	return r.repo.DeleteFolder(ctx, request)
}

func (r *Resource) FolderOne(ctx context.Context, request *FolderIDRequest) (response.Response[FolderDetail], error) {
	var zero response.Response[FolderDetail]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireID(request.ID); err != nil {
		return zero, apperrors.BadRequest(400, "文件夹不存在")
	}
	return r.repo.FolderOne(ctx, request)
}

func (r *Resource) SearchFolders(ctx context.Context, search *FolderSearch) (response.PageResponse[FolderSimple], error) {
	var zero response.PageResponse[FolderSimple]
	if search == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	search.Name = trimSpace(search.Name)
	if utf8.RuneCountInString(search.Name) > 100 {
		return zero, apperrors.BadRequest(400, "搜索词过长")
	}
	if search.ParentID != nil && *search.ParentID < 0 {
		return zero, apperrors.BadRequest(400, "文件夹不存在")
	}
	if err := requireNonNegativeTeam(search.TeamID); err != nil {
		return zero, err
	}
	page, pageSize, err := normalizePage(search.Page, search.PageSize)
	if err != nil {
		return zero, err
	}
	search.Page = page
	search.PageSize = pageSize
	return r.repo.SearchFolders(ctx, search)
}

func (r *Resource) Export(ctx context.Context, request *DocumentExportRequest) (response.Response[DocumentFile], error) {
	var zero response.Response[DocumentFile]
	if request == nil {
		return zero, apperrors.BadRequest(400, "请求格式不正确")
	}
	if err := requireID(request.ID); err != nil {
		return zero, err
	}
	current, err := r.repo.One(ctx, &DocumentIDRequest{ID: request.ID})
	if err != nil {
		return zero, err
	}
	file, err := ExportFile(current.Data.Kind, current.Data.Title, current.Data.Content)
	if err != nil {
		return zero, err
	}
	return response.OK(DocumentFile{
		Filename:      file.Filename,
		MediaType:     file.MediaType,
		ContentBase64: base64.StdEncoding.EncodeToString(file.Body),
	}), nil
}

func decodeUpload(encoded string) ([]byte, error) {
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\r', '\t':
			return -1
		default:
			return r
		}
	}, strings.TrimSpace(encoded))
	if cleaned == "" {
		return nil, apperrors.BadRequest(400, "文件内容为空")
	}
	raw, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(cleaned, "="))
	}
	if err != nil {
		return nil, apperrors.BadRequest(400, "文件内容无法读取")
	}
	if len(raw) == 0 {
		return nil, apperrors.BadRequest(400, "文件内容为空")
	}
	return raw, nil
}
