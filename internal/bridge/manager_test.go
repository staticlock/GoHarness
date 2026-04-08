package bridge

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestManagerUpsertListRemove(t *testing.T) {
	m := NewManager()
	m.Upsert(SessionSnapshot{SessionID: "s1", Status: "running", StartedAt: 1})
	m.Upsert(SessionSnapshot{SessionID: "s2", Status: "running", StartedAt: 2})
	items := m.ListSnapshots()
	if len(items) != 2 || items[0].SessionID != "s2" {
		t.Fatalf("unexpected session list: %+v", items)
	}
	m.Remove("s1")
	items = m.ListSnapshots()
	if len(items) != 1 || items[0].SessionID != "s2" {
		t.Fatalf("unexpected list after remove: %+v", items)
	}
}

func TestManagerSpawnReadOutputAndStop(t *testing.T) {
	t.Setenv("OPENHARNESS_CONFIG_DIR", t.TempDir())
	m := NewManager()

	session, err := m.Spawn("echo bridge-test", t.TempDir())
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}
	if session.SessionID == "" || session.Status != "running" {
		t.Fatalf("unexpected session after spawn: %+v", session)
	}

	var out string
	for i := 0; i < 20; i++ {
		out, err = m.ReadOutput(session.SessionID)
		if err == nil && strings.Contains(out, "bridge-test") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(out, "bridge-test") {
		t.Fatalf("expected output to include bridge-test, got %q (err=%v)", out, err)
	}

	longCmd := "sleep 5"
	if runtime.GOOS == "windows" {
		longCmd = "ping -n 6 127.0.0.1 > nul"
	}
	running, err := m.Spawn(longCmd, t.TempDir())
	if err != nil {
		t.Fatalf("spawn long command failed: %v", err)
	}
	if err := m.Stop(running.SessionID); err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	items := m.ListSnapshots()
	foundKilled := false
	for _, item := range items {
		if item.SessionID == running.SessionID && item.Status == "killed" {
			foundKilled = true
		}
	}
	if !foundKilled {
		t.Fatalf("expected killed status for stopped session; items=%+v", items)
	}
}
