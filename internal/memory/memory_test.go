package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectMemoryDir(t *testing.T) {
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "test_project")
	os.MkdirAll(projectDir, 0755)

	memoryDir, err := GetProjectMemoryDir(projectDir)
	if err != nil {
		t.Fatalf("GetProjectMemoryDir failed: %v", err)
	}
	if memoryDir == "" {
		t.Fatalf("expected non-empty memory dir")
	}
	if _, err := os.Stat(memoryDir); os.IsNotExist(err) {
		t.Fatalf("memory dir should exist: %s", memoryDir)
	}
}

func TestMemoryEntrypoint(t *testing.T) {
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "test_project")
	os.MkdirAll(projectDir, 0755)

	entrypoint, err := GetMemoryEntrypoint(projectDir)
	if err != nil {
		t.Fatalf("GetMemoryEntrypoint failed: %v", err)
	}
	if !strings.HasSuffix(entrypoint, "MEMORY.md") {
		t.Fatalf("unexpected entrypoint: %s", entrypoint)
	}
}

func TestListMemoryFiles(t *testing.T) {
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "test_project")
	os.MkdirAll(projectDir, 0755)

	memoryDir, _ := GetProjectMemoryDir(projectDir)
	os.WriteFile(filepath.Join(memoryDir, "test.md"), []byte("test content"), 0644)
	os.WriteFile(filepath.Join(memoryDir, "another.md"), []byte("another"), 0644)

	files, err := ListMemoryFiles(projectDir)
	if err != nil {
		t.Fatalf("ListMemoryFiles failed: %v", err)
	}
	if len(files) != 2 {
		t.Logf("files found: %v", files)
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestAddAndRemoveMemoryEntry(t *testing.T) {
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "test_project")
	os.MkdirAll(projectDir, 0755)

	path, err := AddMemoryEntry(projectDir, "Test Note", "# Test Note\nTest content")
	if err != nil {
		t.Fatalf("AddMemoryEntry failed: %v", err)
	}
	if path == "" {
		t.Fatalf("expected non-empty path")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("memory file should exist: %s", path)
	}

	deleted, err := RemoveMemoryEntry(projectDir, "test_note")
	if err != nil {
		t.Fatalf("RemoveMemoryEntry failed: %v", err)
	}
	if !deleted {
		t.Fatalf("expected deleted = true")
	}
}

func TestLoadMemoryPrompt(t *testing.T) {
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "test_project")
	os.MkdirAll(projectDir, 0755)

	entrypoint, _ := GetMemoryEntrypoint(projectDir)
	os.WriteFile(entrypoint, []byte("# Memory Index\n- [Testing](testing.md)\n"), 0644)

	prompt, err := LoadMemoryPrompt(projectDir)
	if err != nil {
		t.Fatalf("LoadMemoryPrompt failed: %v", err)
	}
	if prompt == "" {
		t.Fatalf("expected non-empty prompt")
	}
	if !strings.Contains(prompt, "Testing") {
		t.Fatalf("unexpected prompt content: %s", prompt)
	}
}
