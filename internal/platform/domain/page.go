package domain

import "errors"

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 200
	MaxPageIndex    = 100000
)

// ErrInvalidPage 表示页码或页大小不合法。
var ErrInvalidPage = errors.New("分页参数无效")

// NormalizePage 把省略的页码当成默认值。
// JSON 数字无法区分省略和 0，因此 0 视为未提供；负数和超限是参数错误。
func NormalizePage(page, pageSize int) (int, int, error) {
	if page < 0 || pageSize < 0 {
		return 0, 0, ErrInvalidPage
	}
	if page == 0 {
		page = DefaultPage
	}
	if pageSize == 0 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		return 0, 0, ErrInvalidPage
	}
	if int64(page-1) > int64(MaxPageIndex)/int64(pageSize) {
		return 0, 0, ErrInvalidPage
	}
	return page, pageSize, nil
}
