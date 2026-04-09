package tools

import (
	"context"
	"encoding/json"
	"strings"
)

// ToolSearchTool searches available tools.
type ToolSearchTool struct{}

func (t *ToolSearchTool) Name() string { return "tool_search" }
func (t *ToolSearchTool) Description() string {
	return "Search the available tool list by name or description."
}
func (t *ToolSearchTool) IsReadOnly() bool { return true }

func (t *ToolSearchTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Substring to search in tool names and descriptions",
			},
		},
		"required": []string{"query"},
	}
}

type toolSearchInput struct {
	Query string `json:"query"`
}

func (t *ToolSearchTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input toolSearchInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	query := strings.ToLower(input.Query)
	registry := execCtx.Metadata["tool_registry"]
	if registry == nil {
		return ToolResult{Output: "Tool registry context not available", IsError: true}, nil
	}

	reg, ok := registry.(*Registry)
	if !ok {
		return ToolResult{Output: "Tool registry context not available", IsError: true}, nil
	}

	matches := reg.List()
	filtered := make([]string, 0)
	for _, tool := range matches {
		name := strings.ToLower(tool.Name())
		desc := strings.ToLower(tool.Description())
		if strings.Contains(name, query) || strings.Contains(desc, query) {
			filtered = append(filtered, tool.Name()+": "+tool.Description())
		}
	}

	if len(filtered) == 0 {
		return NewSuccessResult("(no matches)"), nil
	}

	return NewSuccessResult(joinLinesWithNewline(filtered)), nil
}
