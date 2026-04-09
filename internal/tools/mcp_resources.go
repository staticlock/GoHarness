package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/staticlock/GoHarness/internal/mcp"
)

// ListMcpResourcesTool lists MCP resources.
type ListMcpResourcesTool struct {
	Manager *mcp.ClientManager
}

func (t *ListMcpResourcesTool) Name() string { return "list_mcp_resources" }
func (t *ListMcpResourcesTool) Description() string {
	return "List MCP resources available from connected servers."
}
func (t *ListMcpResourcesTool) IsReadOnly() bool { return true }

func (t *ListMcpResourcesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *ListMcpResourcesTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = ctx
	_ = execCtx

	manager := t.Manager
	if manager == nil {
		return NewSuccessResult("(no MCP resources)"), nil
	}

	resources := manager.ListResources()
	if len(resources) == 0 {
		return NewSuccessResult("(no MCP resources)"), nil
	}

	lines := make([]string, 0, len(resources))
	for _, r := range resources {
		desc := r.Description
		if desc == "" {
			desc = "(no description)"
		}
		lines = append(lines, fmt.Sprintf("%s:%s %s", r.ServerName, r.URI, desc))
	}

	return NewSuccessResult(joinLinesWithNewline(lines)), nil
}

// ReadMcpResourceTool reads an MCP resource.
type ReadMcpResourceTool struct {
	Manager *mcp.ClientManager
}

func (t *ReadMcpResourceTool) Name() string        { return "read_mcp_resource" }
func (t *ReadMcpResourceTool) Description() string { return "Read a specific MCP resource by URI." }
func (t *ReadMcpResourceTool) IsReadOnly() bool    { return true }

func (t *ReadMcpResourceTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"uri": map[string]interface{}{
				"type":        "string",
				"description": "MCP resource URI to read",
			},
		},
		"required": []string{"uri"},
	}
}

type readMcpResourceInput struct {
	URI string `json:"uri"`
}

func (t *ReadMcpResourceTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = ctx
	_ = execCtx

	var input readMcpResourceInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	manager := t.Manager
	if manager == nil {
		return ToolResult{Output: "MCP manager not available", IsError: true}, nil
	}

	// Find server from URI prefix
	serverName := ""
	for _, r := range manager.ListResources() {
		if r.URI == input.URI {
			serverName = r.ServerName
			break
		}
	}
	if serverName == "" {
		return ToolResult{Output: "Resource not found: " + input.URI, IsError: true}, nil
	}

	content, err := manager.ReadResource(serverName, input.URI)
	if err != nil {
		return NewErrorResult(err), nil
	}

	if content == "" {
		content = "(no content)"
	}

	return NewSuccessResult(content), nil
}
