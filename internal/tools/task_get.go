package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/staticlock/GoHarness/internal/tasks"
)

// TaskGetTool retrieves one task record.
type TaskGetTool struct {
	Manager *tasks.Manager
}

func (t *TaskGetTool) Name() string        { return "task_get" }
func (t *TaskGetTool) Description() string { return "Get details for a background task." }
func (t *TaskGetTool) IsReadOnly() bool    { return true }

func (t *TaskGetTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_id": map[string]interface{}{"type": "string", "description": "Task identifier"},
		},
		"required": []string{"task_id"},
	}
}

type taskGetInput struct {
	TaskID string `json:"task_id"`
}

func (t *TaskGetTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = ctx
	_ = execCtx
	var input taskGetInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}
	manager := t.Manager
	if manager == nil {
		manager = tasks.DefaultManager()
	}
	record, ok := manager.GetTask(input.TaskID)
	if !ok {
		return ToolResult{Output: "No task found with ID: " + input.TaskID, IsError: true}, nil
	}
	return NewSuccessResult(fmt.Sprintf("{id:%s type:%s status:%s description:%q cwd:%q command:%q created_at:%s}", record.ID, record.Type, record.Status, record.Description, record.CWD, record.Command, record.CreatedAt.Format(time.RFC3339))), nil
}
