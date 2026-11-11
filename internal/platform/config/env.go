package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

type envField struct {
	name string
	path []string
	kind string
}

func envFields() []envField {
	return []envField{
		{name: "APP_NAME", path: []string{"app", "name"}, kind: "string"},
		{name: "APP_SERVER_HOST", path: []string{"server", "host"}, kind: "string"},
		{name: "APP_SERVER_PORT", path: []string{"server", "port"}, kind: "int"},
		{name: "APP_SERVER_READ_HEADER_TIMEOUT", path: []string{"server", "read_header_timeout"}, kind: "duration"},
		{name: "APP_SERVER_IDLE_TIMEOUT", path: []string{"server", "idle_timeout"}, kind: "duration"},
		{name: "APP_SERVER_MAX_HEADER_BYTES", path: []string{"server", "max_header_bytes"}, kind: "int"},
		{name: "APP_SERVER_SHUTDOWN_TIMEOUT", path: []string{"server", "shutdown_timeout"}, kind: "duration"},
		{name: "APP_SERVER_REQUEST_TIMEOUT", path: []string{"server", "request_timeout"}, kind: "duration"},
		{name: "APP_SERVER_SSE_TIMEOUT", path: []string{"server", "sse_timeout"}, kind: "duration"},
		{name: "APP_SERVER_MAX_BODY_BYTES", path: []string{"server", "max_body_bytes"}, kind: "int64"},
		{name: "APP_SERVER_TRUSTED_PROXIES", path: []string{"server", "trusted_proxies"}, kind: "strings"},
		{name: "APP_DATABASE_ENABLED", path: []string{"database", "enabled"}, kind: "bool"},
		{name: "APP_DATABASE_DRIVER", path: []string{"database", "driver"}, kind: "string"},
		{name: "APP_DATABASE_DSN", path: []string{"database", "dsn"}, kind: "secret"},
		{name: "APP_DATABASE_MAX_OPEN_CONNS", path: []string{"database", "max_open_conns"}, kind: "int"},
		{name: "APP_DATABASE_MAX_IDLE_CONNS", path: []string{"database", "max_idle_conns"}, kind: "int"},
		{name: "APP_DATABASE_CONN_MAX_LIFETIME", path: []string{"database", "conn_max_lifetime"}, kind: "duration"},
		{name: "APP_DATABASE_BATCH_SIZE", path: []string{"database", "batch_size"}, kind: "int"},
		{name: "APP_DATABASE_SQLITE_PATH", path: []string{"database", "sqlite", "path"}, kind: "string"},
		{name: "APP_DATABASE_SQLITE_FOREIGN_KEYS", path: []string{"database", "sqlite", "foreign_keys"}, kind: "bool"},
		{name: "APP_DATABASE_SQLITE_BUSY_TIMEOUT", path: []string{"database", "sqlite", "busy_timeout"}, kind: "duration"},
		{name: "APP_DATABASE_SQLITE_JOURNAL_MODE", path: []string{"database", "sqlite", "journal_mode"}, kind: "string"},
		{name: "APP_AUTH_ENABLED", path: []string{"auth", "enabled"}, kind: "bool"},
		{name: "APP_AUTH_SESSION_TTL", path: []string{"auth", "session_ttl"}, kind: "duration"},
		{name: "APP_AUTH_COOKIE_SECURE", path: []string{"auth", "cookie_secure"}, kind: "bool"},
		{name: "APP_LOGGING_LEVEL", path: []string{"logging", "level"}, kind: "string"},
		{name: "APP_LOGGING_FORMAT", path: []string{"logging", "format"}, kind: "string"},
	}
}

func applyEnv(merged map[string]any) error {
	for _, field := range envFields() {
		value, ok := os.LookupEnv(field.name)
		if !ok {
			continue
		}
		parsed, err := parseEnv(field, value)
		if err != nil {
			return err
		}
		if err := putPath(merged, field.path, parsed); err != nil {
			return err
		}
	}
	return nil
}

func parseEnv(field envField, value string) (any, error) {
	if field.kind == "secret" && value == "" {
		return nil, fmt.Errorf("%s 已设置但为空", field.name)
	}
	switch field.kind {
	case "string", "secret":
		return value, nil
	case "bool":
		if value != "true" && value != "false" {
			return nil, fmt.Errorf("%s 只接受 true 或 false", field.name)
		}
		return value == "true", nil
	case "int":
		number, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("%s 不是整数", field.name)
		}
		return number, nil
	case "int64":
		number, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s 不是整数", field.name)
		}
		return number, nil
	case "duration":
		if _, err := time.ParseDuration(value); err != nil {
			return nil, fmt.Errorf("%s 不是带单位的时长", field.name)
		}
		return value, nil
	case "strings":
		if value == "" {
			return nil, fmt.Errorf("%s 已设置但为空，列表请使用 JSON 数组", field.name)
		}
		var items []string
		if err := json.Unmarshal([]byte(value), &items); err != nil {
			return nil, fmt.Errorf("%s 必须是 JSON 字符串数组", field.name)
		}
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out, nil
	default:
		return nil, fmt.Errorf("未知环境变量类型 %s", field.name)
	}
}

func putPath(root map[string]any, path []string, value any) error {
	current := root
	for i, key := range path {
		if i == len(path)-1 {
			current[key] = value
			return nil
		}
		next, ok := current[key].(map[string]any)
		if !ok {
			return fmt.Errorf("配置路径 %s 不存在", key)
		}
		current = next
	}
	return nil
}
