// Package server 组装 Gin 引擎、健康检查和 HTTP 生命周期。
package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"gorm.io/gorm"
	"yunjian/internal/platform/config"
	"yunjian/internal/platform/database"
	"yunjian/internal/platform/routing"
)

// Readiness 表示迁移完成且数据库可连接。
type Readiness struct {
	mu    sync.RWMutex
	ready bool
	db    *gorm.DB
}

func NewReadiness(db *gorm.DB) *Readiness {
	return &Readiness{db: db}
}

func (r *Readiness) MarkReady() {
	r.mu.Lock()
	r.ready = true
	r.mu.Unlock()
}

func (r *Readiness) Check(ctx context.Context) error {
	r.mu.RLock()
	ready := r.ready
	r.mu.RUnlock()
	if !ready {
		return errors.New("migration pending")
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return database.Ping(pingCtx, r.db)
}

type EngineParams struct {
	fx.In
	Config     *config.Config
	Logger     *slog.Logger
	Global     gin.HandlerFunc     `name:"global"`
	Auth       gin.HandlerFunc     `name:"auth"`
	Registrars []routing.Registrar `group:"registrars"`
	Ready      *Readiness
}

func NewEngine(params EngineParams) (*gin.Engine, error) {
	if params.Config.Logging.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}
	engine := gin.New()
	if err := engine.SetTrustedProxies(params.Config.Server.TrustedProxies); err != nil {
		return nil, err
	}
	engine.Use(params.Global)
	engine.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "live"})
	})
	engine.GET("/health/ready", func(c *gin.Context) {
		if err := params.Ready.Check(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	api := engine.Group("/api")
	public := api.Group("")
	protected := api.Group("")
	protected.Use(params.Auth)
	for _, registrar := range params.Registrars {
		registrar.RegisterPublic(public)
		registrar.RegisterProtected(protected)
	}
	return engine, nil
}

type StartParams struct {
	fx.In
	Config     *config.Config
	Engine     *gin.Engine
	Logger     *slog.Logger
	Shutdowner fx.Shutdowner
	Lifecycle  fx.Lifecycle
}

func Start(params StartParams) {
	srv := &http.Server{
		Addr:              params.Config.ListenAddr(),
		Handler:           params.Engine,
		ReadHeaderTimeout: params.Config.Server.ReadHeaderTimeout,
		IdleTimeout:       params.Config.Server.IdleTimeout,
		MaxHeaderBytes:    params.Config.Server.MaxHeaderBytes,
	}
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}
			go func() {
				serveErr := srv.Serve(ln)
				if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
					params.Logger.Error("http server stopped", "err", serveErr)
					_ = params.Shutdowner.Shutdown()
				}
			}()
			params.Logger.Info("http listening", "addr", srv.Addr)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			params.Logger.Info("http shutting down")
			return srv.Shutdown(ctx)
		},
	})
}
