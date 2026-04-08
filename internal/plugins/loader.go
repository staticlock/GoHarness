package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/user/goharness/internal/config"
	"github.com/user/goharness/internal/hooks"
	"github.com/user/goharness/internal/mcp"
)

type manifest struct {
	Name             string `json:"name"`
	EnabledByDefault bool   `json:"enabled_by_default"`
	HooksFile        string `json:"hooks_file"`
	MCPFile          string `json:"mcp_file"`
}

type mcpJSON struct {
	MCPServers map[string]any `json:"mcpServers"`
}

// LoadPluginExtensions discovers enabled plugins and returns their hook/mcp contributions.
func LoadPluginExtensions(settings config.Settings, cwd string) (map[string][]hooks.Definition, map[string]any) {
	hooksByEvent := map[string][]hooks.Definition{}
	mcpServers := map[string]any{}
	for _, path := range discoverPluginPaths(cwd) {
		name, enabled, pluginHooks, pluginMCP := loadPlugin(path, settings.EnabledPlugins)
		if !enabled {
			continue
		}
		for event, defs := range pluginHooks {
			hooksByEvent[event] = append(hooksByEvent[event], defs...)
		}
		for serverName, raw := range pluginMCP {
			prefixed := mcp.PrefixPluginServerName(name, serverName)
			mcpServers[prefixed] = raw
		}
	}
	return hooksByEvent, mcpServers
}

func discoverPluginPaths(cwd string) []string {
	paths := []string{}
	if cfgDir, err := config.ConfigDir(); err == nil {
		paths = append(paths, scanPluginRoot(filepath.Join(cfgDir, "plugins"))...)
	}
	paths = append(paths, scanPluginRoot(filepath.Join(cwd, ".openharness", "plugins"))...)
	return paths
}

func scanPluginRoot(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	out := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if findManifest(path) != "" {
			out = append(out, path)
		}
	}
	return out
}

func findManifest(pluginDir string) string {
	candidates := []string{
		filepath.Join(pluginDir, "plugin.json"),
		filepath.Join(pluginDir, ".claude-plugin", "plugin.json"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func loadPlugin(path string, enabledPlugins map[string]bool) (string, bool, map[string][]hooks.Definition, map[string]any) {
	manifestPath := findManifest(path)
	if manifestPath == "" {
		return "", false, nil, nil
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", false, nil, nil
	}
	m := manifest{HooksFile: "hooks.json", MCPFile: "mcp.json"}
	if err := json.Unmarshal(manifestBytes, &m); err != nil || m.Name == "" {
		return "", false, nil, nil
	}
	enabled, ok := enabledPlugins[m.Name]
	if !ok {
		enabled = m.EnabledByDefault
	}
	if !enabled {
		return m.Name, false, nil, nil
	}

	hooksMap := loadPluginHooks(filepath.Join(path, m.HooksFile))
	pluginMCP := loadPluginMCP(filepath.Join(path, m.MCPFile))
	if len(pluginMCP) == 0 {
		pluginMCP = loadPluginMCP(filepath.Join(path, ".mcp.json"))
	}
	return m.Name, true, hooksMap, pluginMCP
}

func loadPluginHooks(path string) map[string][]hooks.Definition {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string][]hooks.Definition{}
	}
	var raw map[string][]map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return map[string][]hooks.Definition{}
	}
	out := map[string][]hooks.Definition{}
	for event, items := range raw {
		defs := make([]hooks.Definition, 0, len(items))
		for _, item := range items {
			def := hooks.Definition{
				Type:           asString(item["type"]),
				Command:        asString(item["command"]),
				URL:            asString(item["url"]),
				Prompt:         asString(item["prompt"]),
				Model:          asString(item["model"]),
				Matcher:        asString(item["matcher"]),
				TimeoutSeconds: asInt(item["timeout_seconds"], 30),
				BlockOnFailure: asBool(item["block_on_failure"], false),
			}
			if h := asStringMap(item["headers"]); len(h) > 0 {
				def.Headers = h
			}
			if def.Type != "" {
				defs = append(defs, def)
			}
		}
		if len(defs) > 0 {
			out[event] = defs
		}
	}
	return out
}

func loadPluginMCP(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{}
	}
	var raw mcpJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return map[string]any{}
	}
	if raw.MCPServers == nil {
		return map[string]any{}
	}
	return raw.MCPServers
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asInt(v any, d int) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return d
	}
}

func asBool(v any, d bool) bool {
	b, ok := v.(bool)
	if !ok {
		return d
	}
	return b
}

func asStringMap(v any) map[string]string {
	raw, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := map[string]string{}
	for k, val := range raw {
		if s, ok := val.(string); ok {
			out[k] = s
		}
	}
	return out
}
