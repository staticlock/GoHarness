package mcp

import (
	"testing"

	"github.com/staticlock/GoHarness/internal/config"
)

func TestLoadServerConfigsAndPrefix(t *testing.T) {
	settings := config.Settings{
		MCPServers: map[string]interface{}{
			"local": map[string]any{
				"type":    "stdio",
				"command": "node",
				"args":    []any{"server.js"},
			},
		},
	}
	cfgs := LoadServerConfigs(settings)
	cfg, ok := cfgs["local"]
	if !ok {
		t.Fatalf("expected local config to be loaded")
	}
	if cfg.Type != "stdio" || cfg.Command != "node" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if PrefixPluginServerName("demo", "svc") != "demo:svc" {
		t.Fatalf("unexpected plugin prefixing")
	}
}
