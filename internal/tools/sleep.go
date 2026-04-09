package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type SleepTool struct{}

func (t *SleepTool) Name() string { return "sleep" }
func (t *SleepTool) Description() string {
	return "Sleep for a short duration."
}
func (t *SleepTool) IsReadOnly() bool { return true }

func (t *SleepTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"seconds": map[string]interface{}{
				"type":        "number",
				"description": "Number of seconds to sleep (0-30)",
				"default":     1.0,
			},
		},
	}
}

type sleepInput struct {
	Seconds float64 `json:"seconds"`
}

func (t *SleepTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = execCtx

	var input sleepInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	if input.Seconds <= 0 {
		input.Seconds = 1.0
	}
	if input.Seconds > 30 {
		input.Seconds = 30
	}

	duration := time.Duration(input.Seconds * float64(time.Second))
	select {
	case <-ctx.Done():
		return NewSuccessResult("Sleep interrupted"), nil
	case <-time.After(duration):
		return NewSuccessResult(fmt.Sprintf("Slept for %.1f seconds", input.Seconds)), nil
	}
}
