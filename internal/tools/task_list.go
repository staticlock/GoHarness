package tools

import (
	"context"
	"encoding/json"

	"github.com/user/goharness/internal/tasks"
)

// TaskListTool lists background tasks.
type TaskListTool struct {
	Manager *tasks.Manager
}

func (t *TaskListTool) Name() string        { return "task_list" }
func (t *TaskListTool) Description() string { return "List background tasks." }
func (t *TaskListTool) IsReadOnly() bool    { return true }

func (t *TaskListTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"status": map[string]interface{}{"type": "string", "description": "Optional status filter"},
		},
	}
}

type taskListInput struct {
	Status string `json:"status,omitempty"`
}

func (t *TaskListTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = ctx
	_ = execCtx
	var input taskListInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}
	manager := t.Manager
	if manager == nil {
		manager = tasks.DefaultManager()
	}
	records := manager.ListTasks(input.Status)
	if len(records) == 0 {
		return NewSuccessResult("(no tasks)"), nil
	}
	lines := make([]string, 0, len(records))
	for _, record := range records {
		lines = append(lines, record.ID+" "+string(record.Type)+" "+string(record.Status)+" "+record.Description)
	}
	return NewSuccessResult(joinLinesWithNewline(lines)), nil
}
