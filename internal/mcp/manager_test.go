package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
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

func TestManagerSupportsHTTPAndWS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		id := int(req["id"].(float64))
		method, _ := req["method"].(string)
		result := map[string]any{}
		switch method {
		case "tools/list":
			result = map[string]any{"tools": []map[string]any{}}
		case "resources/list":
			result = map[string]any{"resources": []map[string]any{}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}))
	defer srv.Close()

	upgrader := websocket.Upgrader{}
	wsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var req map[string]any
			if err := json.Unmarshal(msg, &req); err != nil {
				continue
			}
			id := int(req["id"].(float64))
			method, _ := req["method"].(string)
			result := map[string]any{}
			switch method {
			case "tools/list":
				result = map[string]any{"tools": []map[string]any{}}
			case "resources/list":
				result = map[string]any{"resources": []map[string]any{}}
			}
			_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
		}
	}))
	defer wsSrv.Close()
	wsURL := "ws" + strings.TrimPrefix(wsSrv.URL, "http")

	manager := NewClientManager(map[string]ServerConfig{
		"h": {Type: "http", URL: srv.URL},
		"w": {Type: "ws", URL: wsURL},
	})
	manager.ConnectAll()
	statuses := manager.ListStatuses()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	statusByName := map[string]ConnectionStatus{}
	for _, s := range statuses {
		statusByName[s.Name] = s
	}
	if statusByName["h"].State != "connected" {
		t.Fatalf("expected http server connected, got %+v", statusByName["h"])
	}
	if statusByName["w"].State != "connected" {
		t.Fatalf("expected ws server connected, got %+v", statusByName["w"])
	}
}

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

	pythonExe, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python executable not found in PATH")
	}

	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
	serverScript := filepath.Join(projectRoot, "tests", "fixtures", "fake_mcp_server.py")

	manager := NewClientManager(map[string]ServerConfig{
		"fixture": {
			Type:    "stdio",
			Command: pythonExe,
			Args:    []string{serverScript},
		},
	})
	manager.ConnectAll()
	defer manager.Close()

	statuses := manager.ListStatuses()
	if len(statuses) != 1 {
		t.Fatalf("expected one mcp status, got %d", len(statuses))
	}
	status := statuses[0]
	if status.State != "connected" {
		if canSkipIntegration(status.Detail) {
			t.Skipf("mcp fixture unavailable in current environment: %s", status.Detail)
		}
		t.Fatalf("expected connected mcp status, got %s (%s)", status.State, status.Detail)
	}
	if len(status.Tools) == 0 || status.Tools[0].Name != "hello" {
		t.Fatalf("expected hello tool from fixture, got %+v", status.Tools)
	}
	if len(status.Resources) == 0 || status.Resources[0].URI != "fixture://readme" {
		t.Fatalf("expected fixture resource, got %+v", status.Resources)
	}

	output, err := manager.CallTool("fixture", "hello", map[string]any{"name": "world"})
	if err != nil {
		t.Fatalf("call tool failed: %v", err)
	}
	if strings.TrimSpace(output) != "fixture-hello:world" {
		t.Fatalf("unexpected tool output: %q", output)
	}

	resource, err := manager.ReadResource("fixture", "fixture://readme")
	if err != nil {
		t.Fatalf("read resource failed: %v", err)
	}
	if !strings.Contains(resource, "fixture resource contents") {
		t.Fatalf("unexpected resource output: %q", resource)
	}
}

func canSkipIntegration(detail string) bool {
	low := strings.ToLower(detail)
	return strings.Contains(low, "no module named") || strings.Contains(low, "not found") || strings.Contains(low, "can't open file")
}
