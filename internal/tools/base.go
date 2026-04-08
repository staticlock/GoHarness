// Package tools provides the tool system for the agent harness.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	Output   string                 `json:"output"`
	IsError  bool                   `json:"is_error"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToolExecutionContext provides context for tool execution.
type ToolExecutionContext struct {
	CWD      string
	Metadata map[string]interface{}
}

// Tool is the interface that all tools must implement.
type Tool interface {
	// Name returns the unique name of the tool.
	Name() string
	// Description returns the description of what the tool does.
	Description() string
	// InputSchema returns the JSON schema for the tool's input parameters.
	InputSchema() map[string]interface{}
	// Execute runs the tool with the given arguments.
	Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error)
	// IsReadOnly returns true if the tool doesn't modify state.
	IsReadOnly() bool
}

// Registry holds all registered tools.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Unregister removes a tool by name.
func (r *Registry) Unregister(name string) {
	delete(r.tools, name)
}

// Has reports whether a tool exists in the registry.
func (r *Registry) Has(name string) bool {
	_, ok := r.tools[name]
	return ok
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools.
func (r *Registry) List() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ToAPISchema returns all tools in API format.
func (r *Registry) ToAPISchema() []map[string]interface{} {
	schemas := make([]map[string]interface{}, 0, len(r.tools))
	for _, tool := range r.tools {
		schemas = append(schemas, map[string]interface{}{
			"name":         tool.Name(),
			"description":  tool.Description(),
			"input_schema": tool.InputSchema(),
		})
	}
	return schemas
}

// NewSuccessResult creates a successful tool result.
func NewSuccessResult(output string) ToolResult {
	return ToolResult{Output: output}
}

// NewErrorResult creates an error tool result.
func NewErrorResult(err error) ToolResult {
	return ToolResult{
		Output:  fmt.Sprintf("Error: %v", err),
		IsError: true,
	}
}

// NewErrorResultf creates a formatted error result.
func NewErrorResultf(format string, args ...interface{}) ToolResult {
	return ToolResult{
		Output:  fmt.Sprintf("Error: "+format, args...),
		IsError: true,
	}
}
