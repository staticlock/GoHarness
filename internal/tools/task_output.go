package tools

import (
	"context"
	"encoding/json"

	"github.com/staticlock/GoHarness/internal/tasks"
)

type TaskOutputTool struct {
	Manager *tasks.Manager
}

func (t *TaskOutputTool) Name() string { return "task_output" }
func (t *TaskOutputTool) Description() string {
	return "Read the output log for a background task."
}
func (t *TaskOutputTool) IsReadOnly() bool { return true }

func (t *TaskOutputTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "Task identifier",
			},
			"max_bytes": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum bytes to read (default 12000)",
				"default":     12000,
			},
		},
		"required": []string{"task_id"},
	}
}

type taskOutputInput struct {
	TaskID   string `json:"task_id"`
	MaxBytes int    `json:"max_bytes,omitempty"`
}

func (t *TaskOutputTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = ctx
	_ = execCtx

	var input taskOutputInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	if input.MaxBytes <= 0 {
		input.MaxBytes = 12000
	}
	if input.MaxBytes > 100000 {
		input.MaxBytes = 100000
	}

	manager := t.Manager
	if manager == nil {
		manager = tasks.DefaultManager()
	}

	output, err := manager.ReadTaskOutput(input.TaskID)
	if err != nil {
		return NewErrorResult(err), nil
	}

	if output == "" {
		return NewSuccessResult("(no output)"), nil
	}

	if len(output) > input.MaxBytes {
		output = output[:input.MaxBytes] + "\n... (truncated)"
	}

	return NewSuccessResult(output), nil
}
