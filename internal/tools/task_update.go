package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/staticlock/GoHarness/internal/tasks"
)

type TaskUpdateTool struct {
	Manager *tasks.Manager
}

func (t *TaskUpdateTool) Name() string { return "task_update" }
func (t *TaskUpdateTool) Description() string {
	return "Update a task description, progress, or status note."
}
func (t *TaskUpdateTool) IsReadOnly() bool { return false }

func (t *TaskUpdateTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "Task identifier",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Updated task description",
			},
			"progress": map[string]interface{}{
				"type":        "integer",
				"description": "Progress percentage (0-100)",
			},
			"status_note": map[string]interface{}{
				"type":        "string",
				"description": "Short human-readable task note",
			},
		},
		"required": []string{"task_id"},
	}
}

type taskUpdateInput struct {
	TaskID      string `json:"task_id"`
	Description string `json:"description,omitempty"`
	Progress    *int   `json:"progress,omitempty"`
	StatusNote  string `json:"status_note,omitempty"`
}

func (t *TaskUpdateTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = ctx
	_ = execCtx

	var input taskUpdateInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	manager := t.Manager
	if manager == nil {
		manager = tasks.DefaultManager()
	}

	var progress *int
	if input.Progress != nil {
		progress = input.Progress
	}

	var statusNote *string
	if input.StatusNote != "" {
		statusNote = &input.StatusNote
	}

	record, err := manager.UpdateTask(input.TaskID, nil, progress, statusNote)
	if err != nil {
		return NewErrorResult(err), nil
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Updated task %s", record.ID))
	if input.Description != "" {
		parts = append(parts, fmt.Sprintf("description=%s", record.Description))
	}
	if input.Progress != nil {
		if p, ok := record.Metadata["progress"]; ok {
			parts = append(parts, fmt.Sprintf("progress=%s%%", p))
		}
	}
	if input.StatusNote != "" {
		if note, ok := record.Metadata["status_note"]; ok {
			parts = append(parts, fmt.Sprintf("note=%s", note))
		}
	}

	return NewSuccessResult(joinWords(parts)), nil
}

func joinWords(words []string) string {
	result := ""
	for i, w := range words {
		if i > 0 {
			result += " "
		}
		result += w
	}
	return result
}
