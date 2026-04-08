package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/goharness/internal/engine"
)

func TestSaveListLoadSessionSnapshot(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Setenv("OPENHARNESS_CONFIG_DIR", filepath.Join(tmp, "cfg")); err != nil {
		t.Fatalf("set env failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("OPENHARNESS_CONFIG_DIR") })

	messages := []engine.ConversationMessage{{Role: "user", Text: "hello world"}}
	usage := engine.UsageSnapshot{InputTokens: 1, OutputTokens: 2, TotalTokens: 3}

	path, err := SaveSessionSnapshot(tmp, "model-a", "sys", "sess-1", messages, usage)
	if err != nil {
		t.Fatalf("save session failed: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected saved path, got error: %v", err)
	}

	latest, err := LoadLatestSession(tmp)
	if err != nil || latest == nil {
		t.Fatalf("load latest failed: %v", err)
	}
	if latest.SessionID != "sess-1" {
		t.Fatalf("unexpected latest session id: %s", latest.SessionID)
	}

	list, err := ListSessions(tmp, 10)
	if err != nil {
		t.Fatalf("list sessions failed: %v", err)
	}
	if len(list) == 0 || list[0].SessionID != "sess-1" {
		t.Fatalf("expected session in list")
	}

	byID, err := LoadSessionByID(tmp, "sess-1")
	if err != nil || byID == nil {
		t.Fatalf("load by id failed: %v", err)
	}
	if byID.Model != "model-a" {
		t.Fatalf("unexpected session model: %s", byID.Model)
	}
}
