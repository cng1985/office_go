// Package migrations 嵌入版本化 SQL。业务启动只通过 bootstrap 执行。
package migrations

import "embed"

// Files 包含各方言的迁移脚本。
//
//go:embed *.sql
var Files embed.FS
