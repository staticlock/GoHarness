package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/staticlock/GoHarness/internal/tasks"
)

type SendMessageTool struct {
	Manager *tasks.Manager
}

func (t *SendMessageTool) Name() string { return "send_message" }
func (t *SendMessageTool) Description() string {
	return "Send a follow-up message to a running local agent task."
}
func (t *SendMessageTool) IsReadOnly() bool { return false }

func (t *SendMessageTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "Target local agent task id",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Message to write to the task stdin",
			},
		},
		"required": []string{"task_id", "message"},
	}
}

type sendMessageInput struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
}

func (t *SendMessageTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input sendMessageInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	record, ok := t.Manager.GetTask(input.TaskID)
	if !ok {
		return ToolResult{Output: fmt.Sprintf("No task found with ID: %s", input.TaskID), IsError: true}, nil
	}

	if record.Status != "running" {
		return ToolResult{Output: fmt.Sprintf("Task %s is not running", input.TaskID), IsError: true}, nil
	}

	if record.Type != "local_agent" {
		return ToolResult{Output: fmt.Sprintf("Task %s does not accept input", input.TaskID), IsError: true}, nil
	}

	if err := t.Manager.WriteToTask(input.TaskID, input.Message); err != nil {
		return ToolResult{Output: err.Error(), IsError: true}, nil
	}

	return NewSuccessResult(fmt.Sprintf("Sent message to task %s", input.TaskID)), nil
}
