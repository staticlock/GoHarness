package hooks

import (
	"context"
	"testing"
)

func TestCommandHookBlocking(t *testing.T) {
	reg := NewRegistry()
	reg.Register(PreToolUse, Definition{
		Type:           "command",
		Command:        "exit 1",
		BlockOnFailure: true,
		TimeoutSeconds: 5,
	})

	exec := NewExecutor(reg, ExecutionContext{CWD: t.TempDir()})
	res := exec.Execute(context.Background(), PreToolUse, map[string]any{"tool_name": "write_file"})
	if !res.IsBlocked() {
		t.Fatalf("expected command hook to block on failure")
	}
}

func TestPromptHookBlocking(t *testing.T) {
	reg := NewRegistry()
	reg.Register(PreToolUse, Definition{
		Type:           "prompt",
		Prompt:         "validate $ARGUMENTS",
		BlockOnFailure: true,
	})

	exec := NewExecutor(reg, ExecutionContext{
		CWD: t.TempDir(),
		PromptEvaluator: func(ctx context.Context, prompt, model string, agentMode bool) (string, error) {
			_ = ctx
			_ = prompt
			_ = model
			_ = agentMode
			return `{"ok": false, "reason": "blocked by test"}`, nil
		},
	})
	res := exec.Execute(context.Background(), PreToolUse, map[string]any{"tool_name": "write_file"})
	if !res.IsBlocked() {
		t.Fatalf("expected prompt hook to block")
	}
	if res.Reason() != "blocked by test" {
		t.Fatalf("unexpected reason: %s", res.Reason())
	}
}
