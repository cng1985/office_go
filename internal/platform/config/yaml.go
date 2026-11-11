package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type schemaNode struct {
	children map[string]*schemaNode
}

func schemaRoot() *schemaNode {
	return &schemaNode{children: map[string]*schemaNode{
		"app": {children: map[string]*schemaNode{
			"name": {}, "profile": {}, "base_dir": {}, "config_dir": {}, "data_dir": {},
		}},
		"server": {children: map[string]*schemaNode{
			"host": {}, "port": {}, "read_header_timeout": {}, "idle_timeout": {},
			"max_header_bytes": {}, "shutdown_timeout": {}, "request_timeout": {},
			"sse_timeout": {}, "max_body_bytes": {}, "trusted_proxies": {},
		}},
		"database": {children: map[string]*schemaNode{
			"enabled": {}, "driver": {}, "dsn": {}, "max_open_conns": {}, "max_idle_conns": {},
			"conn_max_lifetime": {}, "batch_size": {},
			"sqlite": {children: map[string]*schemaNode{
				"path": {}, "foreign_keys": {}, "busy_timeout": {}, "journal_mode": {},
			}},
		}},
		"auth": {children: map[string]*schemaNode{
			"enabled": {}, "session_ttl": {}, "cookie_secure": {},
		}},
		"logging": {children: map[string]*schemaNode{
			"level": {}, "format": {},
		}},
	}}
}

func readYAMLFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, err
	}
	value, err := decodeNode(&node)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if value == nil {
		return map[string]any{}, nil
	}
	mapped, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s 的根节点必须是对象", path)
	}
	return mapped, nil
}

func decodeNode(node *yaml.Node) (any, error) {
	if node == nil {
		return nil, nil
	}
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return map[string]any{}, nil
		}
		return decodeNode(node.Content[0])
	}
	switch node.Kind {
	case yaml.MappingNode:
		out := map[string]any{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i].Value
			value, err := decodeNode(node.Content[i+1])
			if err != nil {
				return nil, err
			}
			out[key] = value
		}
		return out, nil
	case yaml.SequenceNode:
		out := make([]any, 0, len(node.Content))
		for _, child := range node.Content {
			value, err := decodeNode(child)
			if err != nil {
				return nil, err
			}
			out = append(out, value)
		}
		return out, nil
	case yaml.ScalarNode:
		var scalar any
		if err := node.Decode(&scalar); err != nil {
			return nil, err
		}
		return scalar, nil
	default:
		return nil, fmt.Errorf("不支持的 YAML 节点")
	}
}

func mergeMaps(dst, src map[string]any, schema *schemaNode) error {
	for key, value := range src {
		child := schema.children[key]
		if child == nil {
			return fmt.Errorf("未知配置项 %s", key)
		}
		if child.children != nil {
			srcMap, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("配置项 %s 必须是对象", key)
			}
			dstMap, _ := dst[key].(map[string]any)
			if dstMap == nil {
				dstMap = map[string]any{}
				dst[key] = dstMap
			}
			if err := mergeMaps(dstMap, srcMap, child); err != nil {
				return err
			}
			continue
		}
		dst[key] = value
	}
	return nil
}
