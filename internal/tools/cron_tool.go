package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/staticlock/GoHarness/internal/config"
)

// CronCreateTool creates a cron job.
type CronCreateTool struct{}

func (t *CronCreateTool) Name() string        { return "cron_create" }
func (t *CronCreateTool) Description() string { return "Create or replace a local cron-style job." }
func (t *CronCreateTool) IsReadOnly() bool    { return false }

func (t *CronCreateTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Unique cron job name",
			},
			"schedule": map[string]interface{}{
				"type":        "string",
				"description": "Human-readable schedule expression",
			},
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Shell command to run when triggered",
			},
			"cwd": map[string]interface{}{
				"type":        "string",
				"description": "Optional working directory override",
			},
		},
		"required": []string{"name", "schedule", "command"},
	}
}

type cronCreateInput struct {
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
	Cwd      string `json:"cwd,omitempty"`
}

func (t *CronCreateTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input cronCreateInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	cwd := input.Cwd
	if cwd == "" {
		cwd = execCtx.CWD
	}

	job := config.CronJob{
		Name:     input.Name,
		Schedule: input.Schedule,
		Command:  input.Command,
		Cwd:      cwd,
	}

	if err := config.UpsertCronJob(job); err != nil {
		return NewErrorResult(err), nil
	}

	return NewSuccessResult(fmt.Sprintf("Created cron job %s", input.Name)), nil
}

// CronListTool lists cron jobs.
type CronListTool struct{}

func (t *CronListTool) Name() string        { return "cron_list" }
func (t *CronListTool) Description() string { return "List configured local cron-style jobs." }
func (t *CronListTool) IsReadOnly() bool    { return true }

func (t *CronListTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *CronListTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = execCtx
	jobs, err := config.LoadCronJobs()
	if err != nil {
		return NewErrorResult(err), nil
	}

	if len(jobs) == 0 {
		return NewSuccessResult("No cron jobs configured."), nil
	}

	lines := make([]string, 0, len(jobs))
	for _, job := range jobs {
		lines = append(lines, fmt.Sprintf("%s [%s] -> %s", job.Name, job.Schedule, job.Command))
	}

	return NewSuccessResult(joinLinesWithNewline(lines)), nil
}

// CronDeleteTool deletes a cron job.
type CronDeleteTool struct{}

func (t *CronDeleteTool) Name() string        { return "cron_delete" }
func (t *CronDeleteTool) Description() string { return "Delete a local cron-style job by name." }
func (t *CronDeleteTool) IsReadOnly() bool    { return false }

func (t *CronDeleteTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Cron job name",
			},
		},
		"required": []string{"name"},
	}
}

type cronDeleteInput struct {
	Name string `json:"name"`
}

func (t *CronDeleteTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = execCtx
	var input cronDeleteInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	deleted, err := config.DeleteCronJob(input.Name)
	if err != nil {
		return NewErrorResult(err), nil
	}
	if !deleted {
		return ToolResult{Output: fmt.Sprintf("Cron job not found: %s", input.Name), IsError: true}, nil
	}

	return NewSuccessResult(fmt.Sprintf("Deleted cron job %s", input.Name)), nil
}
