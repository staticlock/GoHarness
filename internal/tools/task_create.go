package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/user/goharness/internal/tasks"
)

// TaskCreateTool creates a background task record.
type TaskCreateTool struct {
	Manager *tasks.Manager
}

func (t *TaskCreateTool) Name() string { return "task_create" }
func (t *TaskCreateTool) Description() string {
	return "Create a background shell or local-agent task."
}
func (t *TaskCreateTool) IsReadOnly() bool { return false }

func (t *TaskCreateTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"type":        map[string]interface{}{"type": "string", "description": "Task type: local_bash or local_agent"},
			"description": map[string]interface{}{"type": "string", "description": "Short task description"},
			"command":     map[string]interface{}{"type": "string", "description": "Shell command for local_bash"},
			"prompt":      map[string]interface{}{"type": "string", "description": "Prompt for local_agent"},
			"model":       map[string]interface{}{"type": "string", "description": "Optional model for local_agent"},
		},
		"required": []string{"description"},
	}
}

type taskCreateInput struct {
	Type        string `json:"type,omitempty"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	Model       string `json:"model,omitempty"`
}

func (t *TaskCreateTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = ctx
	var input taskCreateInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}
	if input.Type == "" {
		input.Type = "local_bash"
	}
	manager := t.Manager
	if manager == nil {
		manager = tasks.DefaultManager()
	}

	switch input.Type {
	case "local_bash":
		if input.Command == "" {
			return ToolResult{Output: "command is required for local_bash tasks", IsError: true}, nil
		}
		record := manager.CreateShellTask(input.Command, input.Description, execCtx.CWD)
		return NewSuccessResult(fmt.Sprintf("Created task %s (%s)", record.ID, record.Type)), nil
	case "local_agent":
		if input.Prompt == "" {
			return ToolResult{Output: "prompt is required for local_agent tasks", IsError: true}, nil
		}
		record := manager.CreateAgentTask(input.Prompt, input.Description, execCtx.CWD, input.Model)
		return NewSuccessResult(fmt.Sprintf("Created task %s (%s)", record.ID, record.Type)), nil
	default:
		return ToolResult{Output: fmt.Sprintf("unsupported task type: %s", input.Type), IsError: true}, nil
	}
}
