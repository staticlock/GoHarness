package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type wsClient struct {
	serverName string
	conn       *websocket.Conn
	nextID     int
	mu         sync.Mutex
}

func newWSClient(serverName string, cfg ServerConfig) (*wsClient, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, fmt.Errorf("ws mcp server %s missing url", serverName)
	}
	headers := http.Header{}
	for k, v := range cfg.Headers {
		headers.Set(k, v)
	}
	conn, _, err := websocket.DefaultDialer.Dial(cfg.URL, headers)
	if err != nil {
		return nil, err
	}
	return &wsClient{serverName: serverName, conn: conn}, nil
}

func (c *wsClient) ListTools() ([]ToolInfo, error) {
	result, err := c.call("tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, err
	}
	tools := make([]ToolInfo, 0, len(parsed.Tools))
	for _, tool := range parsed.Tools {
		tools = append(tools, ToolInfo{ServerName: c.serverName, Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema})
	}
	return tools, nil
}

func (c *wsClient) ListResources() ([]ResourceInfo, error) {
	result, err := c.call("resources/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Resources []struct {
			Name        string `json:"name"`
			URI         string `json:"uri"`
			Description string `json:"description"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, err
	}
	resources := make([]ResourceInfo, 0, len(parsed.Resources))
	for _, r := range parsed.Resources {
		name := r.Name
		if strings.TrimSpace(name) == "" {
			name = r.URI
		}
		resources = append(resources, ResourceInfo{ServerName: c.serverName, Name: name, URI: r.URI, Description: r.Description})
	}
	return resources, nil
}

func (c *wsClient) CallTool(toolName string, arguments map[string]any) (string, error) {
	result, err := c.call("tools/call", map[string]any{"name": toolName, "arguments": arguments})
	if err != nil {
		return "", err
	}
	var parsed struct {
		Content           []map[string]any `json:"content"`
		StructuredContent any              `json:"structuredContent"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return "", err
	}
	parts := make([]string, 0)
	for _, item := range parsed.Content {
		if typ, _ := item["type"].(string); typ == "text" {
			if text, _ := item["text"].(string); text != "" {
				parts = append(parts, text)
				continue
			}
		}
		b, _ := json.Marshal(item)
		parts = append(parts, string(b))
	}
	if len(parts) == 0 && parsed.StructuredContent != nil {
		parts = append(parts, fmt.Sprintf("%v", parsed.StructuredContent))
	}
	if len(parts) == 0 {
		parts = append(parts, "(no output)")
	}
	return strings.TrimSpace(strings.Join(parts, "\n")), nil
}

func (c *wsClient) ReadResource(uri string) (string, error) {
	result, err := c.call("resources/read", map[string]any{"uri": uri})
	if err != nil {
		return "", err
	}
	var parsed struct {
		Contents []map[string]any `json:"contents"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return "", err
	}
	parts := make([]string, 0, len(parsed.Contents))
	for _, item := range parsed.Contents {
		if text, _ := item["text"].(string); text != "" {
			parts = append(parts, text)
			continue
		}
		if blob, _ := item["blob"].(string); blob != "" {
			parts = append(parts, blob)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n")), nil
}

func (c *wsClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *wsClient) call(method string, params map[string]any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	id := c.nextID

	req := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	if err := c.conn.WriteJSON(req); err != nil {
		return nil, err
	}
	for {
		_ = c.conn.SetReadDeadline(time.Now().Add(15 * time.Second))
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return nil, err
		}
		var resp struct {
			ID     *int            `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			continue
		}
		if resp.ID == nil || *resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("mcp error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}
