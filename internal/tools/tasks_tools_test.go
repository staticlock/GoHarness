package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/user/goharness/internal/tasks"
)

func TestTaskToolsFlow(t *testing.T) {
	manager := tasks.NewManager()
	execCtx := ToolExecutionContext{CWD: t.TempDir()}

	createArgs, _ := json.Marshal(map[string]any{
		"type":        "local_bash",
		"description": "run demo",
		"command":     "echo hello",
	})
	createResult, err := (&TaskCreateTool{Manager: manager}).Execute(context.Background(), createArgs, execCtx)
	if err != nil || createResult.IsError {
		t.Fatalf("task_create failed: %v %+v", err, createResult)
	}
	if !strings.Contains(createResult.Output, "Created task ") {
		t.Fatalf("unexpected create output: %s", createResult.Output)
	}

	listArgs, _ := json.Marshal(map[string]any{})
	listResult, err := (&TaskListTool{Manager: manager}).Execute(context.Background(), listArgs, execCtx)
	if err != nil || listResult.IsError {
		t.Fatalf("task_list failed: %v %+v", err, listResult)
	}
	if !strings.Contains(listResult.Output, "local_bash") {
		t.Fatalf("unexpected list output: %s", listResult.Output)
	}

	items := manager.ListTasks("")
	if len(items) == 0 {
		t.Fatalf("expected at least one task")
	}
	getArgs, _ := json.Marshal(map[string]any{"task_id": items[0].ID})
	getResult, err := (&TaskGetTool{Manager: manager}).Execute(context.Background(), getArgs, execCtx)
	if err != nil || getResult.IsError {
		t.Fatalf("task_get failed: %v %+v", err, getResult)
	}
	if !strings.Contains(getResult.Output, items[0].ID) {
		t.Fatalf("unexpected get output: %s", getResult.Output)
	}
}

func TestTaskCreateLocalAgent(t *testing.T) {
	manager := tasks.NewManager()
	execCtx := ToolExecutionContext{CWD: t.TempDir()}

	createArgs, _ := json.Marshal(map[string]any{
		"type":        "local_agent",
		"description": "agent demo",
		"prompt":      "implement feature",
		"model":       "claude-sonnet",
	})
	createResult, err := (&TaskCreateTool{Manager: manager}).Execute(context.Background(), createArgs, execCtx)
	if err != nil || createResult.IsError {
		t.Fatalf("task_create local_agent failed: %v %+v", err, createResult)
	}
	if !strings.Contains(createResult.Output, "Created task ") || !strings.Contains(createResult.Output, "local_agent") {
		t.Fatalf("unexpected create output: %s", createResult.Output)
	}
}
