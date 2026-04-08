package ui

import (
	"encoding/json"
	"testing"
)

func TestBackendEventToolInputJSONField(t *testing.T) {
	event := BackendEvent{Type: "tool_started", ToolName: "write_file", ToolInput: map[string]any{"path": "a.txt"}}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := decoded["tool_input"]; !ok {
		t.Fatalf("expected tool_input field in backend event payload")
	}
}
