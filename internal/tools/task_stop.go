package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/staticlock/GoHarness/internal/tasks"
)

type TaskStopTool struct {
	Manager *tasks.Manager
}

func (t *TaskStopTool) Name() string { return "task_stop" }
func (t *TaskStopTool) Description() string {
	return "Stop a background task."
}
func (t *TaskStopTool) IsReadOnly() bool { return false }

func (t *TaskStopTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "Task identifier",
			},
		},
		"required": []string{"task_id"},
	}
}

type taskStopInput struct {
	TaskID string `json:"task_id"`
}

func (t *TaskStopTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = ctx
	_ = execCtx

	var input taskStopInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	manager := t.Manager
	if manager == nil {
		manager = tasks.DefaultManager()
	}

	record, err := manager.StopTask(input.TaskID)
	if err != nil {
		return NewErrorResult(err), nil
	}

	return NewSuccessResult(fmt.Sprintf("Stopped task %s", record.ID)), nil
}
