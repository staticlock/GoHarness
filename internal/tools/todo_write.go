package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TodoWriteTool appends items to a TODO file.
type TodoWriteTool struct{}

func (t *TodoWriteTool) Name() string { return "todo_write" }
func (t *TodoWriteTool) Description() string {
	return "Append a TODO item to a markdown checklist file."
}
func (t *TodoWriteTool) IsReadOnly() bool { return false }

func (t *TodoWriteTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"item": map[string]interface{}{
				"type":        "string",
				"description": "TODO item text",
			},
			"checked": map[string]interface{}{
				"type":        "boolean",
				"description": "Mark as checked",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "TODO file path",
			},
		},
		"required": []string{"item"},
	}
}

type todoWriteInput struct {
	Item    string `json:"item"`
	Checked bool   `json:"checked,omitempty"`
	Path    string `json:"path,omitempty"`
}

func (t *TodoWriteTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input todoWriteInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	path := input.Path
	if path == "" {
		path = "TODO.md"
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(execCtx.CWD, path)
	}

	prefix := "- [ ]"
	if input.Checked {
		prefix = "- [x]"
	}

	var existing string
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	} else if !os.IsNotExist(err) {
		return NewErrorResult(err), nil
	} else {
		existing = "# TODO\n"
	}

	existing = trimNewline(existing)
	updated := existing + "\n" + prefix + " " + input.Item + "\n"

	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		return NewErrorResult(err), nil
	}

	return NewSuccessResult(fmt.Sprintf("Updated %s", path)), nil
}
