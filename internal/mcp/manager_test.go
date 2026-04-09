package mcp

import (
	"strings"
	"testing"
)

type fakeTransport struct {
	tools     []ToolInfo
	resources []ResourceInfo
	callOut   string
	readOut   string
}

func (f *fakeTransport) ListTools() ([]ToolInfo, error)         { return f.tools, nil }
func (f *fakeTransport) ListResources() ([]ResourceInfo, error) { return f.resources, nil }
func (f *fakeTransport) CallTool(toolName string, arguments map[string]any) (string, error) {
	_ = toolName
	_ = arguments
	return f.callOut, nil
}
func (f *fakeTransport) ReadResource(uri string) (string, error) {
	_ = uri
	return f.readOut, nil
}
func (f *fakeTransport) Close() error { return nil }

func TestManagerUsesInjectedTransportFactory(t *testing.T) {
	manager := NewClientManager(map[string]ServerConfig{
		"fixture": {Type: "stdio", Command: "fake"},
	})
	manager.newTransport = func(serverName string, cfg ServerConfig) (TransportClient, error) {
		_ = serverName
		_ = cfg
		return &fakeTransport{
			tools:     []ToolInfo{{ServerName: "fixture", Name: "hello", Description: "say hi", InputSchema: map[string]any{"type": "object"}}},
			resources: []ResourceInfo{{ServerName: "fixture", Name: "readme", URI: "fixture://readme"}},
			callOut:   "fixture-hello:world",
			readOut:   "fixture resource contents",
		}, nil
	}

	manager.ConnectAll()
	statuses := manager.ListStatuses()
	if len(statuses) != 1 || statuses[0].State != "connected" {
		t.Fatalf("expected connected fixture status, got %+v", statuses)
	}
	if len(statuses[0].Tools) != 1 || statuses[0].Tools[0].Name != "hello" {
		t.Fatalf("expected hello tool in status, got %+v", statuses[0].Tools)
	}

	toolOut, err := manager.CallTool("fixture", "hello", map[string]any{"name": "world"})
	if err != nil {
		t.Fatalf("call tool failed: %v", err)
	}
	if toolOut != "fixture-hello:world" {
		t.Fatalf("unexpected tool output: %q", toolOut)
	}

	resourceOut, err := manager.ReadResource("fixture", "fixture://readme")
	if err != nil {
		t.Fatalf("read resource failed: %v", err)
	}
	if resourceOut != "fixture resource contents" {
		t.Fatalf("unexpected resource output: %q", resourceOut)
	}
}

func TestManagerDualLayerErrorMessage(t *testing.T) {
	manager := NewClientManager(map[string]ServerConfig{})
	_, err := manager.CallTool("missing", "hello", map[string]any{"name": "world"})
	if err == nil {
		t.Fatalf("expected error when calling unknown mcp server")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "tool call failed") || !strings.Contains(msg, "debug:") {
		t.Fatalf("expected dual-layer error message, got %q", err.Error())
	}
}

func TestStdioMCPFixtureIntegration(t *testing.T) {
	if os.Getenv("OPENHARNESS_RUN_MCP_STDIO_TEST") != "1" {
		t.Skip("set OPENHARNESS_RUN_MCP_STDIO_TEST=1 to run real stdio MCP integration test")
	}

	manager := NewClientManager(map[string]ServerConfig{
		"fixture": {
			Type:    "stdio",
			Command: "nonexistent-command",
			Args:    []string{},
		},
	})
	manager.ConnectAll()
	defer manager.Close()

	statuses := manager.ListStatuses()
	if len(statuses) != 1 {
		t.Fatalf("expected one mcp status, got %d", len(statuses))
	}
	if status := statuses[0]; status.State != "failed" {
		t.Fatalf("expected failed mcp status, got %s (%s)", status.State, status.Detail)
	}
}
