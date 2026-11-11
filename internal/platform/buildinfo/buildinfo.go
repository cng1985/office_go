// Package buildinfo 保存版本、提交号和构建时间，由发布脚本通过 ldflags 注入。
package buildinfo

var (
	Version   = "0.1.0"
	Commit    = "dev"
	BuildTime = "unknown"
)
