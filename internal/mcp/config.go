package mcp

import (
	"fmt"

	"github.com/user/goharness/internal/config"
)

// LoadServerConfigs normalizes MCP server config from settings map.
func LoadServerConfigs(settings config.Settings) map[string]ServerConfig {
	return LoadServerConfigsFromMap(settings.MCPServers)
}

// LoadServerConfigsFromMap normalizes MCP server config from a raw map.
func LoadServerConfigsFromMap(rawMap map[string]interface{}) map[string]ServerConfig {
	out := map[string]ServerConfig{}
	for name, raw := range rawMap {
		if cfg, ok := toServerConfig(raw); ok {
			out[name] = cfg
		}
	}
	return out
}

// PrefixPluginServerName applies plugin_name:server naming to avoid collisions.
func PrefixPluginServerName(pluginName, serverName string) string {
	if pluginName == "" {
		return serverName
	}
	return pluginName + ":" + serverName
}

func toServerConfig(raw any) (ServerConfig, bool) {
	m, ok := raw.(map[string]any)
	if !ok {
		return ServerConfig{}, false
	}
	cfg := ServerConfig{Type: asString(m["type"])}
	if cfg.Type == "" {
		cfg.Type = "stdio"
	}
	cfg.Command = asString(m["command"])
	cfg.CWD = asString(m["cwd"])
	cfg.URL = asString(m["url"])
	cfg.Args = asStringSlice(m["args"])
	cfg.Env = asStringMap(m["env"])
	cfg.Headers = asStringMap(m["headers"])
	return cfg, true
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asStringSlice(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		if raw2, ok2 := v.([]string); ok2 {
			return append([]string{}, raw2...)
		}
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		out = append(out, fmt.Sprintf("%v", item))
	}
	return out
}

func asStringMap(v any) map[string]string {
	raw, ok := v.(map[string]any)
	if !ok {
		if typed, ok2 := v.(map[string]string); ok2 {
			cp := map[string]string{}
			for k, val := range typed {
				cp[k] = val
			}
			return cp
		}
		return nil
	}
	out := map[string]string{}
	for k, val := range raw {
		out[k] = fmt.Sprintf("%v", val)
	}
	return out
}
