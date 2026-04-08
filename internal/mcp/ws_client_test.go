package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestWSClientFlow(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
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
			var result any
			switch method {
			case "tools/list":
				result = map[string]any{"tools": []map[string]any{{"name": "hello", "description": "say hi", "inputSchema": map[string]any{"type": "object"}}}}
			case "resources/list":
				result = map[string]any{"resources": []map[string]any{{"name": "readme", "uri": "fixture://readme", "description": "fixture"}}}
			case "tools/call":
				result = map[string]any{"content": []map[string]any{{"type": "text", "text": "fixture-hello:world"}}}
			case "resources/read":
				result = map[string]any{"contents": []map[string]any{{"text": "fixture resource contents"}}}
			default:
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32601, "message": "unknown method"}})
				continue
			}
			_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
		}
	}))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, err := newWSClient("fixture", ServerConfig{Type: "ws", URL: url, Headers: map[string]string{"Authorization": "Bearer test"}})
	if err != nil {
		t.Fatalf("newWSClient failed: %v", err)
	}
	defer client.Close()

	tools, err := client.ListTools()
	if err != nil || len(tools) != 1 || tools[0].Name != "hello" {
		t.Fatalf("unexpected tools: err=%v tools=%+v", err, tools)
	}
	resources, err := client.ListResources()
	if err != nil || len(resources) != 1 || resources[0].URI != "fixture://readme" {
		t.Fatalf("unexpected resources: err=%v resources=%+v", err, resources)
	}
	out, err := client.CallTool("hello", map[string]any{"name": "world"})
	if err != nil || out != "fixture-hello:world" {
		t.Fatalf("unexpected call output: err=%v out=%q", err, out)
	}
	resource, err := client.ReadResource("fixture://readme")
	if err != nil || !strings.Contains(resource, "fixture resource contents") {
		t.Fatalf("unexpected resource output: err=%v out=%q", err, resource)
	}
	if sawAuth != "Bearer test" {
		t.Fatalf("expected auth header to be sent, got %q", sawAuth)
	}
}

func TestWSClientErrors(t *testing.T) {
	if _, err := newWSClient("fixture", ServerConfig{Type: "ws", URL: ""}); err == nil {
		t.Fatalf("expected missing url error")
	}
}
