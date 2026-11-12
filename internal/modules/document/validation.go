package document

import (
	"strings"
	"unicode/utf8"

	"yunjian/internal/platform/domain"
	apperrors "yunjian/internal/platform/errors"
)

func validKind(kind string) bool {
	return kind == KindWord || kind == KindExcel || kind == KindPPT || kind == KindPDF
}

func normalizeFolderName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 80 || hasControl(name, false) || strings.ContainsAny(name, `/\`) {
		return "", apperrors.BadRequest(400, "文件夹名称需要 1 到 80 个字符")
	}
	return name, nil
}

func normalizeTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > 200 || hasControl(title, false) {
		return "", apperrors.BadRequest(400, "标题需要 1 到 200 个字符")
	}
	return title, nil
}

func normalizeNote(note string) (string, error) {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > 200 || hasControl(note, false) {
		return "", apperrors.BadRequest(400, "版本说明不能超过 200 个字符")
	}
	return note, nil
}

func normalizePage(page, pageSize int) (int, int, error) {
	normalizedPage, normalizedSize, err := domain.NormalizePage(page, pageSize)
	if err != nil {
		return 0, 0, apperrors.BadRequest(400, "分页参数无效")
	}
	return normalizedPage, normalizedSize, nil
}

func normalizeSort(sort, order string) (string, string, error) {
	var column string
	switch sort {
	case "", "updated_at":
		column = "d.updated_at"
	case "created_at":
		column = "d.created_at"
	case "title":
		column = "d.title"
	default:
		return "", "", apperrors.BadRequest(400, "排序字段无效")
	}
	switch strings.ToLower(order) {
	case "", "desc":
		return column, "desc", nil
	case "asc":
		return column, "asc", nil
	default:
		return "", "", apperrors.BadRequest(400, "排序方向无效")
	}
}

func requireID(id int64) error {
	if id <= 0 {
		return apperrors.BadRequest(400, "文档标识无效")
	}
	return nil
}

func requireRevision(revision int64) error {
	if revision <= 0 {
		return apperrors.BadRequest(400, "文档修订号无效")
	}
	return nil
}

func requireNonNegativeTeam(id int64) error {
	if id < 0 {
		return apperrors.BadRequest(400, "团队不存在")
	}
	return nil
}

func hasControl(value string, allowNewline bool) bool {
	for _, r := range value {
		if allowNewline && r == '\n' {
			continue
		}
		if r < 32 || r == 127 {
			return true
		}
	}
	return false
}
