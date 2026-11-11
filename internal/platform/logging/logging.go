// Package logging 提供结构化日志。
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"yunjian/internal/platform/buildinfo"
	"yunjian/internal/platform/config"
)

func New(cfg *config.Config) *slog.Logger {
	level := parseLevel(cfg.Logging.Level)
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if strings.EqualFold(cfg.Logging.Format, "text") {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(handler).With(
		"service", cfg.App.Name,
		"version", buildinfo.Version,
	)
}

func NewDiscard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
