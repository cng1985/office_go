// Package document 管理云端文稿、表格、演示及其版本。
// 三种文件是同一文档聚合的类型，版本历史属于该聚合，不单独拆模块。
package document

import (
	"encoding/json"
	"time"
)

const (
	KindWord  = "word"
	KindExcel = "excel"
	KindPPT   = "ppt"
)

// DocumentCreateRequest 创建空文档。客户端不能指定主键和正文。
type DocumentCreateRequest struct {
	ID    int64  `json:"id,string"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
}

// DocumentUpdateRequest 只修改标题，不产生内容版本。
type DocumentUpdateRequest struct {
	ID       int64  `json:"id,string"`
	Revision int64  `json:"revision"`
	Title    string `json:"title"`
}

// DocumentIDRequest 用于详情和删除。
type DocumentIDRequest struct {
	ID int64 `json:"id,string"`
}

// DocumentSearch 是文档库筛选条件。
type DocumentSearch struct {
	Title    string `json:"title"`
	Kind     string `json:"kind"`
	Sort     string `json:"sort"`
	Order    string `json:"order"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// DocumentSaveRequest 保存当前编辑内容，并按变化决定是否追加版本。
type DocumentSaveRequest struct {
	ID       int64           `json:"id,string"`
	Revision int64           `json:"revision"`
	Title    string          `json:"title"`
	Note     string          `json:"note"`
	Content  json.RawMessage `json:"content"`
}

// DocumentVersionSearch 分页读取某个文档的历史版本。
type DocumentVersionSearch struct {
	DocumentID int64 `json:"document_id,string"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
}

// DocumentVersionRequest 读取一个历史版本的正文。
type DocumentVersionRequest struct {
	DocumentID int64 `json:"document_id,string"`
	VersionID  int64 `json:"version_id,string"`
}

// DocumentRestoreRequest 把历史版本恢复为新的当前版本，不删除后续历史。
type DocumentRestoreRequest struct {
	ID        int64  `json:"id,string"`
	VersionID int64  `json:"version_id,string"`
	Revision  int64  `json:"revision"`
	Note      string `json:"note"`
}

// DocumentSimple 是文档库列表项。
type DocumentSimple struct {
	ID        int64     `json:"id,string"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Revision  int64     `json:"revision"`
	VersionNo int64     `json:"version_no"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DocumentResponse 是文档详情。Content 是已规范化的 JSON。
type DocumentResponse struct {
	ID        int64           `json:"id,string"`
	Kind      string          `json:"kind"`
	Title     string          `json:"title"`
	Revision  int64           `json:"revision"`
	VersionID int64           `json:"version_id,string"`
	VersionNo int64           `json:"version_no"`
	Content   json.RawMessage `json:"content,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// DocumentSaveResponse 说明这次保存有没有产生新版本。
type DocumentSaveResponse struct {
	Document   DocumentResponse `json:"document"`
	NewVersion bool             `json:"new_version"`
}

// DocumentVersionSimple 是版本列表项，不包含正文。
type DocumentVersionSimple struct {
	ID         int64     `json:"id,string"`
	DocumentID int64     `json:"document_id,string"`
	VersionNo  int64     `json:"version_no"`
	Title      string    `json:"title"`
	Note       string    `json:"note"`
	ByteSize   int64     `json:"byte_size"`
	CreatedAt  time.Time `json:"created_at"`
}

// DocumentImportRequest 用文件内容创建一份云端文档。
type DocumentImportRequest struct {
	Filename      string `json:"filename"`
	ContentBase64 string `json:"content_base64"`
}

// DocumentExportRequest 导出当前版本。
type DocumentExportRequest struct {
	ID int64 `json:"id,string"`
}

// DocumentFile 是可下载的文件。
type DocumentFile struct {
	Filename      string `json:"filename"`
	MediaType     string `json:"media_type"`
	ContentBase64 string `json:"content_base64"`
}

// DocumentVersionResponse 是历史版本详情。
type DocumentVersionResponse struct {
	ID         int64           `json:"id,string"`
	DocumentID int64           `json:"document_id,string"`
	VersionNo  int64           `json:"version_no"`
	Title      string          `json:"title"`
	Kind       string          `json:"kind"`
	Note       string          `json:"note"`
	Content    json.RawMessage `json:"content"`
	ByteSize   int64           `json:"byte_size"`
	CreatedAt  time.Time       `json:"created_at"`
}
