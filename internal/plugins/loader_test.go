package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/staticlock/GoHarness/internal/config"
)

func TestLoadPluginExtensions(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Setenv("OPENHARNESS_CONFIG_DIR", filepath.Join(tmp, "cfg")); err != nil {
		t.Fatalf("set env failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("OPENHARNESS_CONFIG_DIR") })

	pluginDir := filepath.Join(tmp, ".openharness", "plugins", "demo")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	manifest := `{"name":"demo","enabled_by_default":true,"hooks_file":"hooks.json","mcp_file":"mcp.json"}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("manifest write failed: %v", err)
	}
	hooksJSON := `{"pre_tool_use":[{"type":"command","command":"echo ok"}]}`
	if err := os.WriteFile(filepath.Join(pluginDir, "hooks.json"), []byte(hooksJSON), 0o644); err != nil {
		t.Fatalf("hooks write failed: %v", err)
	}
	mcpJSON := `{"mcpServers":{"svc":{"type":"stdio","command":"node","args":["server.js"]}}}`
	if err := os.WriteFile(filepath.Join(pluginDir, "mcp.json"), []byte(mcpJSON), 0o644); err != nil {
		t.Fatalf("mcp write failed: %v", err)
	}

	settings := config.Settings{EnabledPlugins: map[string]bool{"demo": true}}
	hookMap, mcpMap := LoadPluginExtensions(settings, tmp)
	if len(hookMap["pre_tool_use"]) != 1 {
		t.Fatalf("expected pre_tool_use hook from plugin")
	}
	if _, ok := mcpMap["demo:svc"]; !ok {
		t.Fatalf("expected prefixed mcp server from plugin")
	}
}
