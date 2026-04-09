package engine

import (
	"context"
	"testing"

	"github.com/staticlock/GoHarness/internal/config"
	"github.com/staticlock/GoHarness/internal/permissions"
	"github.com/staticlock/GoHarness/internal/tools"
)

type fakeClient struct{}

func (c fakeClient) StreamMessage(req ApiMessageRequest) (<-chan ApiStreamEvent, error) {
	out := make(chan ApiStreamEvent, 1)
	go func() {
		defer close(out)
		if len(req.Messages) > 0 {
			last := req.Messages[len(req.Messages)-1]
			if last.Role == "user" && len(last.ToolResults) > 0 {
				out <- ApiStreamEvent{Complete: &ApiMessageCompleteEvent{Message: ConversationMessage{Role: "assistant", Text: "done"}}}
				return
			}
		}
		out <- ApiStreamEvent{Complete: &ApiMessageCompleteEvent{Message: ConversationMessage{
			Role: "assistant",
			ToolUses: []ToolUse{{
				ID:   "1",
				Name: "write_file",
				Input: map[string]any{
					"path":    "x.txt",
					"content": "hello",
				},
			}},
		}}}
	}()
	return out, nil
}

type permAdapter struct{ checker *permissions.Checker }

func (p permAdapter) Evaluate(toolName string, isReadOnly bool, filePath, command string) PermissionDecision {
	d := p.checker.Evaluate(toolName, isReadOnly, filePath, command)
	return PermissionDecision{Allowed: d.Allowed, RequiresConfirmation: d.RequiresConfirmation, Reason: d.Reason}
}

func TestRunQueryPlanModeBlocksMutatingTool(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&tools.WriteTool{})

	// Build checker via config type to match runtime usage.
	pc := permissions.NewChecker(config.PermissionSettings{Mode: "plan"})

	qc := QueryContext{
		APIClient:         fakeClient{},
		ToolRegistry:      reg,
		PermissionChecker: permAdapter{checker: pc},
		CWD:               t.TempDir(),
		Model:             "test",
		SystemPrompt:      "",
		MaxTokens:         256,
		MaxTurns:          4,
	}

	messages := []ConversationMessage{FromUserText("please write")}
	var events []StreamEvent
	err := RunQuery(context.Background(), qc, &messages, func(ev StreamEvent, usage *UsageSnapshot) error {
		events = append(events, ev)
		_ = usage
		return nil
	})
	if err != nil {
		t.Fatalf("run query failed: %v", err)
	}

	foundDenied := false
	for _, msg := range messages {
		for _, tr := range msg.ToolResults {
			if tr.IsError && tr.Content != "" {
				foundDenied = true
			}
		}
	}
	if !foundDenied {
		t.Fatalf("expected permission-denied tool result in messages: %#v", events)
	}
}
