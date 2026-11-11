// Package config 按固定顺序加载 configs 目录中的 YAML，并用环境变量覆盖。
package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是进程有效配置。定位路径在加载 YAML 之前确定。
type Config struct {
	App      AppConfig      `yaml:"app"`
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Logging  LoggingConfig  `yaml:"logging"`
}

type AppConfig struct {
	Name      string `yaml:"name"`
	Profile   string `yaml:"profile"`
	BaseDir   string `yaml:"base_dir"`
	ConfigDir string `yaml:"config_dir"`
	DataDir   string `yaml:"data_dir"`
}

type ServerConfig struct {
	Host              string        `yaml:"host"`
	Port              int           `yaml:"port"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
	IdleTimeout       time.Duration `yaml:"idle_timeout"`
	MaxHeaderBytes    int           `yaml:"max_header_bytes"`
	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout"`
	RequestTimeout    time.Duration `yaml:"request_timeout"`
	SSETimeout        time.Duration `yaml:"sse_timeout"`
	MaxBodyBytes      int64         `yaml:"max_body_bytes"`
	TrustedProxies    []string      `yaml:"trusted_proxies"`
}

type DatabaseConfig struct {
	Enabled         bool          `yaml:"enabled"`
	Driver          string        `yaml:"driver"`
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	BatchSize       int           `yaml:"batch_size"`
	SQLite          SQLiteConfig  `yaml:"sqlite"`
}

type SQLiteConfig struct {
	Path        string        `yaml:"path"`
	ForeignKeys bool          `yaml:"foreign_keys"`
	BusyTimeout time.Duration `yaml:"busy_timeout"`
	JournalMode string        `yaml:"journal_mode"`
}

type AuthConfig struct {
	Enabled      bool          `yaml:"enabled"`
	SessionTTL   time.Duration `yaml:"session_ttl"`
	CookieSecure bool          `yaml:"cookie_secure"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type locators struct {
	baseDir   string
	configDir string
	dataDir   string
	profile   string
}

// Load 读取命令行、环境变量和 YAML。args 不包含程序名。
func Load(args []string) (*Config, error) {
	located, err := parseLocators(args)
	if err != nil {
		return nil, err
	}
	merged := defaultsMap()
	baseFile := filepath.Join(located.configDir, "application.yaml")
	baseMap, err := readYAMLFile(baseFile)
	if err != nil {
		return nil, fmt.Errorf("读取基础配置失败: %w", err)
	}
	if err := mergeMaps(merged, baseMap, schemaRoot()); err != nil {
		return nil, err
	}
	if located.profile != "" {
		profileFile := filepath.Join(located.configDir, "application-"+located.profile+".yaml")
		profileMap, err := readYAMLFile(profileFile)
		if err != nil {
			return nil, fmt.Errorf("读取环境配置失败: %w", err)
		}
		if err := mergeMaps(merged, profileMap, schemaRoot()); err != nil {
			return nil, err
		}
		if err := rejectLocatorKeys(profileMap); err != nil {
			return nil, err
		}
	}
	if err := rejectLocatorKeys(baseMap); err != nil {
		return nil, err
	}
	if err := applyEnv(merged); err != nil {
		return nil, err
	}
	setLocator(merged, located)
	encoded, err := yaml.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("整理配置失败: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(encoded, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	cfg.App.BaseDir = located.baseDir
	cfg.App.ConfigDir = located.configDir
	cfg.App.DataDir = located.dataDir
	cfg.App.Profile = located.profile
	cfg.Database.SQLite.Path = resolveSQLitePath(located.dataDir, cfg.Database.SQLite.Path)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func parseLocators(args []string) (locators, error) {
	fs := newFlagSet()
	if err := fs.Parse(args); err != nil {
		return locators{}, err
	}
	if fs.NArg() > 0 {
		return locators{}, fmt.Errorf("不能识别的参数 %s", fs.Arg(0))
	}
	base := firstSet(fs.baseDir, os.Getenv("APP_BASE_DIR"))
	if base == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return locators{}, fmt.Errorf("无法确定工作目录: %w", err)
		}
		base = cwd
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return locators{}, fmt.Errorf("无法解析 base-dir: %w", err)
	}
	configDir := firstSet(fs.configDir, os.Getenv("APP_CONFIG_DIR"))
	if configDir == "" {
		configDir = "configs"
	}
	dataDir := firstSet(fs.dataDir, os.Getenv("APP_DATA_DIR"))
	if dataDir == "" {
		dataDir = "./data"
	}
	profile := firstSet(fs.profile, os.Getenv("APP_PROFILE"))
	if profile != "" && profile != "dev" && profile != "pro" {
		return locators{}, fmt.Errorf("未知 profile %s", profile)
	}
	return locators{
		baseDir:   absBase,
		configDir: resolveAgainst(absBase, configDir),
		dataDir:   resolveAgainst(absBase, dataDir),
		profile:   profile,
	}, nil
}

func rejectLocatorKeys(yamlMap map[string]any) error {
	app, ok := yamlMap["app"].(map[string]any)
	if !ok {
		return nil
	}
	for _, key := range []string{"base_dir", "config_dir", "data_dir", "profile"} {
		if _, exists := app[key]; exists {
			return fmt.Errorf("app.%s 只能由启动参数或对应环境变量决定，不能写在 YAML 中", key)
		}
	}
	return nil
}

func setLocator(merged map[string]any, located locators) {
	app := merged["app"].(map[string]any)
	app["name"] = stringOr(app["name"], "yunjian")
	app["profile"] = located.profile
	app["base_dir"] = located.baseDir
	app["config_dir"] = located.configDir
	app["data_dir"] = located.dataDir
}

func resolveSQLitePath(dataDir, path string) string {
	if path == "" {
		path = "app.db"
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dataDir, path)
}

func resolveAgainst(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

func firstSet(flagValue *string, envValue string) string {
	if flagValue != nil && *flagValue != "" {
		return *flagValue
	}
	return envValue
}

func stringOr(value any, fallback string) string {
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return fallback
	}
	return text
}

// Validate 检查必填项、范围和本项目的启用约束。
func (c *Config) Validate() error {
	if strings.TrimSpace(c.App.Name) == "" {
		return fmt.Errorf("app.name 不能为空")
	}
	if c.App.Profile != "" && c.App.Profile != "dev" && c.App.Profile != "pro" {
		return fmt.Errorf("app.profile 仅允许 dev 或 pro")
	}
	if strings.TrimSpace(c.Server.Host) == "" {
		return fmt.Errorf("server.host 不能为空")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port 必须在 1 到 65535 之间")
	}
	if c.Server.ReadHeaderTimeout <= 0 || c.Server.IdleTimeout <= 0 || c.Server.ShutdownTimeout <= 0 || c.Server.RequestTimeout <= 0 || c.Server.SSETimeout <= 0 {
		return fmt.Errorf("server 超时必须大于 0")
	}
	if c.Server.MaxHeaderBytes <= 0 || c.Server.MaxBodyBytes <= 0 {
		return fmt.Errorf("server 报文限制必须大于 0")
	}
	for _, proxy := range c.Server.TrustedProxies {
		if ip := net.ParseIP(proxy); ip == nil {
			if _, _, err := net.ParseCIDR(proxy); err != nil {
				return fmt.Errorf("server.trusted_proxies 包含无法解析的地址 %s", proxy)
			}
		}
	}
	if !c.Database.Enabled {
		return fmt.Errorf("云笺需要数据库，database.enabled 不能为 false")
	}
	switch c.Database.Driver {
	case "sqlite", "mysql", "postgres", "pgsql", "sqlserver":
	default:
		return fmt.Errorf("未知 database.driver %s", c.Database.Driver)
	}
	if c.Database.Driver != "sqlite" && strings.TrimSpace(c.Database.DSN) == "" {
		return fmt.Errorf("网络数据库必须通过 APP_DATABASE_DSN 提供连接串")
	}
	if c.Database.MaxOpenConns < 1 || c.Database.MaxIdleConns < 0 || c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		return fmt.Errorf("数据库连接池参数无效")
	}
	if c.Database.ConnMaxLifetime <= 0 || c.Database.BatchSize < 1 {
		return fmt.Errorf("数据库连接生命周期或批大小无效")
	}
	if c.Database.Driver == "sqlite" {
		if strings.TrimSpace(c.Database.SQLite.Path) == "" {
			return fmt.Errorf("sqlite 路径不能为空")
		}
		switch strings.ToUpper(c.Database.SQLite.JournalMode) {
		case "DELETE", "TRUNCATE", "PERSIST", "MEMORY", "WAL", "OFF":
		default:
			return fmt.Errorf("不支持的 sqlite journal_mode")
		}
		if c.Database.SQLite.BusyTimeout <= 0 {
			return fmt.Errorf("sqlite busy_timeout 必须大于 0")
		}
	}
	if !c.Auth.Enabled {
		return fmt.Errorf("云笺需要登录，auth.enabled 不能为 false")
	}
	if c.Auth.SessionTTL <= 0 {
		return fmt.Errorf("auth.session_ttl 必须大于 0")
	}
	switch strings.ToLower(c.Logging.Level) {
	case "debug", "info", "warn", "warning", "error":
	default:
		return fmt.Errorf("未知 logging.level %s", c.Logging.Level)
	}
	switch strings.ToLower(c.Logging.Format) {
	case "json", "text":
	default:
		return fmt.Errorf("未知 logging.format %s", c.Logging.Format)
	}
	return nil
}

func defaultsMap() map[string]any {
	return map[string]any{
		"app": map[string]any{
			"name":       "yunjian",
			"profile":    "",
			"base_dir":   "",
			"config_dir": "configs",
			"data_dir":   "./data",
		},
		"server": map[string]any{
			"host":                "127.0.0.1",
			"port":                8080,
			"read_header_timeout": "5s",
			"idle_timeout":        "60s",
			"max_header_bytes":    1048576,
			"shutdown_timeout":    "30s",
			"request_timeout":     "30s",
			"sse_timeout":         "15m",
			"max_body_bytes":      2097152,
			"trusted_proxies":     []any{},
		},
		"database": map[string]any{
			"enabled":           true,
			"driver":            "sqlite",
			"dsn":               "",
			"max_open_conns":    1,
			"max_idle_conns":    1,
			"conn_max_lifetime": "30m",
			"batch_size":        100,
			"sqlite": map[string]any{
				"path":         "app.db",
				"foreign_keys": true,
				"busy_timeout": "5s",
				"journal_mode": "DELETE",
			},
		},
		"auth": map[string]any{
			"enabled":       true,
			"session_ttl":   "8h",
			"cookie_secure": false,
		},
		"logging": map[string]any{
			"level":  "info",
			"format": "json",
		},
	}
}

// SQLiteFile 返回已解析的 SQLite 文件路径。网络数据库返回空字符串。
func (c *Config) SQLiteFile() string {
	if c.Database.Driver != "sqlite" {
		return ""
	}
	return c.Database.SQLite.Path
}

// ListenAddr 返回 HTTP 监听地址。
func (c *Config) ListenAddr() string {
	return net.JoinHostPort(c.Server.Host, strconv.Itoa(c.Server.Port))
}
