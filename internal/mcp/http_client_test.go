package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPClientFlowAndHeaders(t *testing.T) {
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
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
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32601, "message": "unknown method"}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}))
	defer srv.Close()

	client, err := newHTTPClient("fixture", ServerConfig{Type: "http", URL: srv.URL, Headers: map[string]string{"Authorization": "Bearer test"}})
	if err != nil {
		t.Fatalf("newHTTPClient failed: %v", err)
	}

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

func TestHTTPClientErrors(t *testing.T) {
	t.Run("http status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer srv.Close()
		client, _ := newHTTPClient("fixture", ServerConfig{Type: "http", URL: srv.URL})
		_, err := client.ListTools()
		if err == nil || !strings.Contains(err.Error(), "status") {
			t.Fatalf("expected http status error, got %v", err)
		}
	})

	t.Run("jsonrpc error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			id := int(req["id"].(float64))
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32001, "message": "bad"}})
		}))
		defer srv.Close()
		client, _ := newHTTPClient("fixture", ServerConfig{Type: "http", URL: srv.URL})
		_, err := client.ListTools()
		if err == nil || !strings.Contains(err.Error(), "mcp error") {
			t.Fatalf("expected jsonrpc error, got %v", err)
		}
	})
}
