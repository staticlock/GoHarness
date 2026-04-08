package tasks

import "testing"

func TestManagerCreateListGet(t *testing.T) {
	m := NewManager()
	rec := m.CreateShellTask("echo hi", "demo task", "/tmp")
	if rec.ID == "" || rec.Type != TaskTypeLocalBash || rec.Status != TaskStatusRunning {
		t.Fatalf("unexpected record: %+v", rec)
	}
	got, ok := m.GetTask(rec.ID)
	if !ok || got.ID != rec.ID {
		t.Fatalf("task lookup failed: %+v", got)
	}
	items := m.ListTasks("")
	if len(items) != 1 || items[0].ID != rec.ID {
		t.Fatalf("unexpected task list: %+v", items)
	}
}

func TestManagerUpdateStopAndReadOutput(t *testing.T) {
	m := NewManager()
	rec := m.CreateShellTask("echo hi", "demo", "/tmp")

	desc := "updated"
	progress := 50
	note := "halfway"
	updated, err := m.UpdateTask(rec.ID, &desc, &progress, &note)
	if err != nil {
		t.Fatalf("update task failed: %v", err)
	}
	if updated.Description != "updated" || updated.Metadata["progress"] != "50" || updated.Metadata["status_note"] != "halfway" {
		t.Fatalf("unexpected updated task: %+v", updated)
	}

	stopped, err := m.StopTask(rec.ID)
	if err != nil {
		t.Fatalf("stop task failed: %v", err)
	}
	if stopped.Status != TaskStatusKilled {
		t.Fatalf("expected killed status, got %s", stopped.Status)
	}

	blank, err := m.ReadTaskOutput(rec.ID)
	if err != nil {
		t.Fatalf("read output failed: %v", err)
	}
	if blank != "" {
		t.Fatalf("expected empty output, got %q", blank)
	}
}

func TestManagerCreateAgentTask(t *testing.T) {
	m := NewManager()
	rec := m.CreateAgentTask("build feature", "agent task", "/tmp", "claude-sonnet")
	if rec.ID == "" || rec.Type != TaskTypeLocalAgent || rec.Prompt != "build feature" {
		t.Fatalf("unexpected agent task record: %+v", rec)
	}
	if rec.Metadata["model"] != "claude-sonnet" {
		t.Fatalf("expected model metadata to be set, got %+v", rec.Metadata)
	}
}
