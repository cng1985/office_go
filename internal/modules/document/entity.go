package document

import "time"

// Folder 是用户自己的文档目录，可以嵌套。
type Folder struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	OwnerID   int64  `gorm:"not null;index:idx_office_folder_owner_parent,priority:1"`
	ParentID  *int64 `gorm:"index:idx_office_folder_owner_parent,priority:2"`
	Name      string `gorm:"size:80;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Folder) TableName() string { return "office_folder" }

// Document 是云端文档主表。删除策略为硬删除，并同时删除全部版本。
type Document struct {
	ID               int64  `gorm:"primaryKey;autoIncrement"`
	OwnerID          int64  `gorm:"not null;index:idx_office_document_owner_updated,priority:1"`
	FolderID         *int64 `gorm:"index:idx_office_document_owner_folder,priority:2"`
	Kind             string `gorm:"size:16;not null"`
	Title            string `gorm:"size:200;not null"`
	CurrentVersionID int64  `gorm:"not null"`
	Revision         int64  `gorm:"not null"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (Document) TableName() string { return "office_document" }

// DocumentVersion 是追加写入的内容版本。
type DocumentVersion struct {
	ID         int64  `gorm:"primaryKey;autoIncrement"`
	DocumentID int64  `gorm:"not null;uniqueIndex:uk_office_document_version_no,priority:1"`
	VersionNo  int64  `gorm:"not null;uniqueIndex:uk_office_document_version_no,priority:2"`
	Title      string `gorm:"size:200;not null"`
	Kind       string `gorm:"size:16;not null"`
	Content    string `gorm:"not null"`
	ContentSHA string `gorm:"size:64;not null"`
	ByteSize   int64  `gorm:"not null"`
	Note       string `gorm:"size:200;not null;default:''"`
	CreatedBy  int64  `gorm:"not null"`
	CreatedAt  time.Time
}

func (DocumentVersion) TableName() string { return "office_document_version" }
