package keybindings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultKeybindings(t *testing.T) {
	if DefaultKeybindings == nil {
		t.Fatalf("expected non-nil keybindings")
	}
	if len(DefaultKeybindings) == 0 {
		t.Fatalf("expected non-empty keybindings")
	}

	if DefaultKeybindings["ctrl+l"] != "clear" {
		t.Fatalf("expected ctrl+l -> clear")
	}
	if DefaultKeybindings["ctrl+k"] != "toggle_vim" {
		t.Fatalf("expected ctrl+k -> toggle_vim")
	}
}

func TestParseKeybindings(t *testing.T) {
	jsonStr := `{"ctrl+q": "quit", "ctrl+p": "preview"}`
	m, err := ParseKeybindings(jsonStr)
	if err != nil {
		t.Fatalf("ParseKeybindings failed: %v", err)
	}
	if m["ctrl+q"] != "quit" {
		t.Fatalf("expected ctrl+q -> quit")
	}
	if m["ctrl+p"] != "preview" {
		t.Fatalf("expected ctrl+p -> preview")
	}
}

func TestResolveKeybindings(t *testing.T) {
	overrides := map[string]string{
		"ctrl+l": "new_action",
	}
	result := ResolveKeybindings(overrides)

	if result["ctrl+l"] != "new_action" {
		t.Fatalf("expected override to work")
	}
	if result["ctrl+k"] != "toggle_vim" {
		t.Fatalf("expected default keybinding preserved")
	}
}

func TestLoadKeybindings(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "custom.json")
	os.WriteFile(path, []byte(`{"ctrl+x": "exit"}`), 0644)

	m, err := LoadKeybindings(path)
	if err != nil {
		t.Fatalf("LoadKeybindings failed: %v", err)
	}
	if m["ctrl+x"] != "exit" {
		t.Fatalf("expected ctrl+x -> exit")
	}
}

func TestGetKeybindingsPath(t *testing.T) {
	path, err := GetKeybindingsPath()
	if err != nil {
		t.Fatalf("GetKeybindingsPath failed: %v", err)
	}
	if !strings.Contains(path, ".openharness") {
		t.Fatalf("unexpected path: %s", path)
	}
}
