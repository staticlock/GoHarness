package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// BashTool executes a shell command with stdout/stderr capture.
type BashTool struct{}

func (t *BashTool) Name() string        { return "bash" }
func (t *BashTool) Description() string { return "Run a shell command in the local repository." }
func (t *BashTool) IsReadOnly() bool    { return false }

func (t *BashTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Shell command to execute",
			},
			"cwd": map[string]interface{}{
				"type":        "string",
				"description": "Working directory override",
			},
			"timeout_seconds": map[string]interface{}{
				"type":        "integer",
				"description": "Execution timeout in seconds",
			},
		},
		"required": []string{"command"},
	}
}

type bashInput struct {
	Command        string `json:"command"`
	CWD            string `json:"cwd,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func (t *BashTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input bashInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}
	if input.Command == "" {
		return NewErrorResultf("command is required"), nil
	}

	cwd := execCtx.CWD
	if input.CWD != "" {
		cwd = resolvePath(execCtx.CWD, input.CWD)
	}
	if input.TimeoutSeconds <= 0 {
		input.TimeoutSeconds = 120
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(input.TimeoutSeconds)*time.Second)
	defer cancel()

	cmd := shellCommand(timeoutCtx, input.Command)
	cmd.Dir = filepath.Clean(cwd)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if timeoutCtx.Err() == context.DeadlineExceeded {
		return ToolResult{Output: fmt.Sprintf("Command timed out after %d seconds", input.TimeoutSeconds), IsError: true}, nil
	}

	out := stdout.String()
	errText := stderr.String()
	text := ""
	if out != "" {
		text = trimNewline(out)
	}
	if errText != "" {
		if text != "" {
			text += "\n"
		}
		text += trimNewline(errText)
	}
	if text == "" {
		text = "(no output)"
	}
	if len(text) > 12000 {
		text = text[:12000] + "\n...[truncated]..."
	}

	result := ToolResult{Output: text, IsError: err != nil}
	if err != nil {
		result.Metadata = map[string]interface{}{"error": err.Error()}
	}
	return result, nil
}

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "/bin/bash", "-lc", command)
}
