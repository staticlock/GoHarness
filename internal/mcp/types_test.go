package mcp

import (
	"testing"
)

func TestServerConfigTypes(t *testing.T) {
	cfg := ServerConfig{
		Type:    "stdio",
		Command: "python",
		Args:    []string{"-m", "mcp_server"},
		Env:     map[string]string{"KEY": "value"},
		CWD:     "/home/user",
	}
	if cfg.Type != "stdio" {
		t.Fatalf("expected stdio type, got %s", cfg.Type)
	}
	if cfg.Command != "python" {
		t.Fatalf("expected python command, got %s", cfg.Command)
	}
	if cfg.Env["KEY"] != "value" {
		t.Fatalf("expected env value, got %s", cfg.Env["KEY"])
	}
}

func TestToolInfo(t *testing.T) {
	info := ToolInfo{
		ServerName:  "test-server",
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]any{"type": "object"},
	}
	if info.Name != "test_tool" {
		t.Fatalf("expected test_tool, got %s", info.Name)
	}
	if info.ServerName != "test-server" {
		t.Fatalf("expected test-server, got %s", info.ServerName)
	}
}

func TestResourceInfo(t *testing.T) {
	info := ResourceInfo{
		ServerName:  "test-server",
		Name:        "test-resource",
		URI:         "test://resource",
		Description: "A test resource",
	}
	if info.URI != "test://resource" {
		t.Fatalf("expected test://resource, got %s", info.URI)
	}
}

func TestConnectionStatus(t *testing.T) {
	status := ConnectionStatus{
		Name:           "test-server",
		State:          "connected",
		Detail:         "",
		Transport:      "stdio",
		AuthConfigured: true,
		Tools: []ToolInfo{
			{Name: "tool1", Description: "desc1"},
		},
		Resources: []ResourceInfo{
			{Name: "res1", URI: "test://res1"},
		},
	}
	if status.State != "connected" {
		t.Fatalf("expected connected, got %s", status.State)
	}
	if len(status.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(status.Tools))
	}
	if len(status.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(status.Resources))
	}
}
