package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestListMcpResourcesTool(t *testing.T) {
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: t.TempDir(), Metadata: map[string]any{}}

	args, _ := json.Marshal(map[string]any{})
	result, err := (&ListMcpResourcesTool{}).Execute(ctx, args, execCtx)
	if err != nil {
		t.Fatalf("list_mcp_resources failed: %v", err)
	}
	if result.Output != "(no MCP resources)" {
		t.Fatalf("expected no resources, got: %s", result.Output)
	}
}

func TestReadMcpResourceTool(t *testing.T) {
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: t.TempDir(), Metadata: map[string]any{}}

	args, _ := json.Marshal(map[string]any{"uri": "test://resource"})
	result, err := (&ReadMcpResourceTool{}).Execute(ctx, args, execCtx)
	if err != nil {
		t.Fatalf("read_mcp_resource failed: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error for missing manager, got: %s", result.Output)
	}
}
