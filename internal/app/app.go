package app

import (
	"fmt"
	"log/slog"
	"os"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"yunjian/internal/platform/buildinfo"
	"yunjian/internal/platform/config"
	"yunjian/internal/platform/logging"
)

// Run 加载配置并启动应用。命令行只负责定位参数和版本信息。
func Run() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-version") {
		fmt.Printf("yunjian %s %s %s\n", buildinfo.Version, buildinfo.Commit, buildinfo.BuildTime)
		return
	}
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	logger := logging.New(cfg)
	slog.SetDefault(logger)
	application := fx.New(
		fx.Supply(cfg, logger),
		Module,
		fx.StopTimeout(cfg.Server.ShutdownTimeout),
		fx.WithLogger(func() fxevent.Logger {
			return &fxevent.SlogLogger{Logger: logger}
		}),
	)
	application.Run()
}
