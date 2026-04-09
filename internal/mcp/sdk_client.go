package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// StdioClient wraps stdio transport using official SDK.
type StdioClient struct {
	serverName string
	mcpClient  *mcp.Client
	session    *mcp.ClientSession
	cmd        *exec.Cmd
	mu         sync.Mutex
}

func newStdioClient(serverName string, cfg ServerConfig) (*StdioClient, error) {
	if strings.TrimSpace(cfg.Command) == "" {
		return nil, fmt.Errorf("stdio mcp server %s missing command", serverName)
	}

	cmd := exec.Command(cfg.Command, cfg.Args...)
	if cfg.CWD != "" {
		cmd.Dir = filepath.Clean(cfg.CWD)
	}
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	cmd.Stdout = nil
	cmd.Stderr = nil

	transport := &mcp.CommandTransport{
		Command: cmd,
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "goharness", Version: "0.1.0"}, nil)
	ctx, cancel := context.WithCancel(context.Background())

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to MCP server %s: %w", serverName, err)
	}

	return &StdioClient{
		serverName: serverName,
		mcpClient:  client,
		session:    session,
		cmd:        cmd,
	}, nil
}

func (c *StdioClient) ListTools() ([]ToolInfo, error) {
	ctx := context.Background()
	var tools []ToolInfo

	toolIter := c.session.Tools(ctx, nil)
	for tool, err := range toolIter {
		if err != nil {
			return nil, fmt.Errorf("failed to list tools: %w", err)
		}
		inputSchema := map[string]any{}
		if tool.InputSchema != nil {
			if is, ok := tool.InputSchema.(map[string]any); ok {
				inputSchema = is
			}
		}
		tools = append(tools, ToolInfo{
			ServerName:  c.serverName,
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: inputSchema,
		})
	}

	return tools, nil
}

func (c *StdioClient) ListResources() ([]ResourceInfo, error) {
	ctx := context.Background()
	var resources []ResourceInfo

	resourceIter := c.session.Resources(ctx, nil)
	for r, err := range resourceIter {
		if err != nil {
			return nil, fmt.Errorf("failed to list resources: %w", err)
		}
		name := r.Name
		if strings.TrimSpace(name) == "" {
			name = string(r.URI)
		}
		resources = append(resources, ResourceInfo{
			ServerName:  c.serverName,
			Name:        name,
			URI:         string(r.URI),
			Description: r.Description,
		})
	}

	return resources, nil
}

func (c *StdioClient) CallTool(toolName string, arguments map[string]any) (string, error) {
	ctx := context.Background()

	params := &mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	}
	result, err := c.session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("tool call failed: %w", err)
	}

	if len(result.Content) == 0 {
		if result.StructuredContent != nil {
			return fmt.Sprintf("%v", result.StructuredContent), nil
		}
		return "(no output)", nil
	}

	var parts []string
	for _, item := range result.Content {
		if tc, ok := item.(*mcp.TextContent); ok {
			if tc.Text != "" {
				parts = append(parts, tc.Text)
				continue
			}
		}
		parts = append(parts, fmt.Sprintf("%v", item))
	}

	if len(parts) == 0 {
		return "(no output)", nil
	}

	return strings.TrimSpace(strings.Join(parts, "\n")), nil
}

func (c *StdioClient) ReadResource(uri string) (string, error) {
	ctx := context.Background()

	params := &mcp.ReadResourceParams{
		URI: uri,
	}
	result, err := c.session.ReadResource(ctx, params)
	if err != nil {
		return "", fmt.Errorf("resource read failed: %w", err)
	}

	if len(result.Contents) == 0 {
		return "(no content)", nil
	}

	var parts []string
	for _, rc := range result.Contents {
		if rc.Text != "" {
			parts = append(parts, rc.Text)
			continue
		}
		if len(rc.Blob) > 0 {
			parts = append(parts, string(rc.Blob))
			continue
		}
		parts = append(parts, fmt.Sprintf("%v", rc))
	}

	return strings.TrimSpace(strings.Join(parts, "\n")), nil
}

func (c *StdioClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.mcpClient != nil {
		_ = c.session.Close()
		c.mcpClient = nil
		c.session = nil
	}

	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_, _ = c.cmd.Process.Wait()
		c.cmd = nil
	}

	return nil
}

// HTTPClient wraps HTTP transport using official SDK.
type HTTPClient struct {
	serverName string
	mcpClient  *mcp.Client
	session    *mcp.ClientSession
	mu         sync.Mutex
}

func newHTTPClient(serverName string, cfg ServerConfig) (*HTTPClient, error) {
	transport := &mcp.StreamableClientTransport{
		Endpoint: cfg.URL,
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "goharness", Version: "0.1.0"}, nil)
	ctx, cancel := context.WithCancel(context.Background())

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to MCP server %s: %w", serverName, err)
	}

	return &HTTPClient{
		serverName: serverName,
		mcpClient:  client,
		session:    session,
	}, nil
}

func (c *HTTPClient) ListTools() ([]ToolInfo, error) {
	ctx := context.Background()
	var tools []ToolInfo

	toolIter := c.session.Tools(ctx, nil)
	for tool, err := range toolIter {
		if err != nil {
			return nil, fmt.Errorf("failed to list tools: %w", err)
		}
		inputSchema := map[string]any{}
		if tool.InputSchema != nil {
			if is, ok := tool.InputSchema.(map[string]any); ok {
				inputSchema = is
			}
		}
		tools = append(tools, ToolInfo{
			ServerName:  c.serverName,
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: inputSchema,
		})
	}

	return tools, nil
}

func (c *HTTPClient) ListResources() ([]ResourceInfo, error) {
	ctx := context.Background()
	var resources []ResourceInfo

	resourceIter := c.session.Resources(ctx, nil)
	for r, err := range resourceIter {
		if err != nil {
			return nil, fmt.Errorf("failed to list resources: %w", err)
		}
		name := r.Name
		if strings.TrimSpace(name) == "" {
			name = string(r.URI)
		}
		resources = append(resources, ResourceInfo{
			ServerName:  c.serverName,
			Name:        name,
			URI:         string(r.URI),
			Description: r.Description,
		})
	}

	return resources, nil
}

func (c *HTTPClient) CallTool(toolName string, arguments map[string]any) (string, error) {
	ctx := context.Background()

	params := &mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	}
	result, err := c.session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("tool call failed: %w", err)
	}

	if len(result.Content) == 0 {
		if result.StructuredContent != nil {
			return fmt.Sprintf("%v", result.StructuredContent), nil
		}
		return "(no output)", nil
	}

	var parts []string
	for _, item := range result.Content {
		if tc, ok := item.(*mcp.TextContent); ok {
			if tc.Text != "" {
				parts = append(parts, tc.Text)
				continue
			}
		}
		parts = append(parts, fmt.Sprintf("%v", item))
	}

	if len(parts) == 0 {
		return "(no output)", nil
	}

	return strings.TrimSpace(strings.Join(parts, "\n")), nil
}

func (c *HTTPClient) ReadResource(uri string) (string, error) {
	ctx := context.Background()

	params := &mcp.ReadResourceParams{
		URI: uri,
	}
	result, err := c.session.ReadResource(ctx, params)
	if err != nil {
		return "", fmt.Errorf("resource read failed: %w", err)
	}

	if len(result.Contents) == 0 {
		return "(no content)", nil
	}

	var parts []string
	for _, rc := range result.Contents {
		if rc.Text != "" {
			parts = append(parts, rc.Text)
			continue
		}
		if len(rc.Blob) > 0 {
			parts = append(parts, string(rc.Blob))
			continue
		}
		parts = append(parts, fmt.Sprintf("%v", rc))
	}

	return strings.TrimSpace(strings.Join(parts, "\n")), nil
}

func (c *HTTPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.mcpClient != nil {
		_ = c.session.Close()
		c.mcpClient = nil
		c.session = nil
	}

	return nil
}

// WSClient wraps WebSocket transport using official SDK.
type WSClient struct {
	serverName string
	mcpClient  *mcp.Client
	session    *mcp.ClientSession
	mu         sync.Mutex
}

func newWSClient(serverName string, cfg ServerConfig) (*WSClient, error) {
	return nil, fmt.Errorf("WebSocket client not implemented")
}

func (c *WSClient) ListTools() ([]ToolInfo, error) {
	return nil, fmt.Errorf("WebSocket client not implemented")
}

func (c *WSClient) ListResources() ([]ResourceInfo, error) {
	return nil, fmt.Errorf("WebSocket client not implemented")
}

func (c *WSClient) CallTool(toolName string, arguments map[string]any) (string, error) {
	return "", fmt.Errorf("WebSocket client not implemented")
}

func (c *WSClient) ReadResource(uri string) (string, error) {
	return "", fmt.Errorf("WebSocket client not implemented")
}

func (c *WSClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.mcpClient != nil {
		_ = c.session.Close()
		c.mcpClient = nil
		c.session = nil
	}

	return nil
}
