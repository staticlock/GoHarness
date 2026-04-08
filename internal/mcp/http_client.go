package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type httpClient struct {
	serverName string
	url        string
	headers    map[string]string
	client     *http.Client

	nextID int
	mu     sync.Mutex
}

func newHTTPClient(serverName string, cfg ServerConfig) (*httpClient, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, fmt.Errorf("http mcp server %s missing url", serverName)
	}
	headers := map[string]string{}
	for k, v := range cfg.Headers {
		headers[k] = v
	}
	return &httpClient{
		serverName: serverName,
		url:        cfg.URL,
		headers:    headers,
		client:     &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (c *httpClient) ListTools() ([]ToolInfo, error) {
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

func (c *httpClient) ListResources() ([]ResourceInfo, error) {
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

func (c *httpClient) CallTool(toolName string, arguments map[string]any) (string, error) {
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

func (c *httpClient) ReadResource(uri string) (string, error) {
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

func (c *httpClient) Close() error { return nil }

func (c *httpClient) call(method string, params map[string]any) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()

	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("mcp http status %d", resp.StatusCode)
	}
	var parsed struct {
		ID     *int            `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if parsed.ID == nil || *parsed.ID != id {
		return nil, fmt.Errorf("invalid mcp response id")
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", parsed.Error.Code, parsed.Error.Message)
	}
	return parsed.Result, nil
}
