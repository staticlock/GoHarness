package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/staticlock/GoHarness/internal/config"
)

// RemoteTriggerTool triggers a cron job immediately.
type RemoteTriggerTool struct{}

func (t *RemoteTriggerTool) Name() string { return "remote_trigger" }
func (t *RemoteTriggerTool) Description() string {
	return "Trigger a configured local cron-style job immediately."
}
func (t *RemoteTriggerTool) IsReadOnly() bool { return false }

func (t *RemoteTriggerTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Cron job name",
			},
			"timeout_seconds": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds",
			},
		},
		"required": []string{"name"},
	}
}

type remoteTriggerInput struct {
	Name           string `json:"name"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func (t *RemoteTriggerTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input remoteTriggerInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	timeout := 120
	if input.TimeoutSeconds > 0 {
		timeout = input.TimeoutSeconds
	}

	job, err := config.GetCronJob(input.Name)
	if err != nil {
		return NewErrorResult(err), nil
	}
	if job == nil {
		return ToolResult{Output: fmt.Sprintf("Cron job not found: %s", input.Name), IsError: true}, nil
	}

	cwd := job.Cwd
	if cwd == "" {
		cwd = execCtx.CWD
	}

	cmd := exec.Command("bash", "-lc", job.Command)
	cmd.Dir = cwd

	output, err := cmd.CombinedOutput()
	if err != nil {
		return ToolResult{
			Output:  fmt.Sprintf("Triggered %s\n%s", input.Name, string(output)),
			IsError: true,
			Metadata: map[string]interface{}{
				"returncode": -1,
			},
		}, nil
	}

	select {
	case <-ctx.Done():
		ctxTimeout, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()
		<-ctxTimeout.Done()
		return ToolResult{
			Output:  fmt.Sprintf("Trigger timed out after %d seconds", timeout),
			IsError: true,
		}, nil
	default:
	}

	return ToolResult{
		Output: fmt.Sprintf("Triggered %s\n%s", input.Name, string(output)),
		Metadata: map[string]interface{}{
			"returncode": 0,
		},
	}, nil
}
