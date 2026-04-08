package tools

import (
	"context"
	"encoding/json"

	"github.com/user/goharness/internal/mcp"
)

// McpToolAdapter exposes one MCP tool through the local tool registry.
type McpToolAdapter struct {
	manager *mcp.ClientManager
	info    mcp.ToolInfo
}

// NewMcpToolAdapter creates a tool adapter for one MCP tool.
func NewMcpToolAdapter(manager *mcp.ClientManager, info mcp.ToolInfo) *McpToolAdapter {
	return &McpToolAdapter{manager: manager, info: info}
}

func (t *McpToolAdapter) Name() string        { return t.info.Name }
func (t *McpToolAdapter) Description() string { return t.info.Description }
func (t *McpToolAdapter) InputSchema() map[string]interface{} {
	if t.info.InputSchema == nil {
		return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	}
	return t.info.InputSchema
}
func (t *McpToolAdapter) IsReadOnly() bool { return false }

func (t *McpToolAdapter) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = ctx
	_ = execCtx
	var input map[string]any
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}
	output, err := t.manager.CallTool(t.info.ServerName, t.info.Name, input)
	if err != nil {
		return NewErrorResult(err), nil
	}
	if output == "" {
		output = "(no output)"
	}
	return NewSuccessResult(output), nil
}
