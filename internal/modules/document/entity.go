package document

import "time"

// Document 是云端文档主表。删除策略为硬删除，并同时删除全部版本。
type Document struct {
	ID               int64  `gorm:"primaryKey;autoIncrement"`
	OwnerID          int64  `gorm:"not null;index:idx_office_document_owner_updated,priority:1"`
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
