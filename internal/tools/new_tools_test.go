package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobTool(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: tmp}

	os.MkdirAll(filepath.Join(tmp, "sub"), 0o755)
	os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(tmp, "sub", "c.txt"), []byte("c"), 0o644)

	args, _ := json.Marshal(map[string]any{"pattern": "*.txt"})
	result, err := (&GlobTool{}).Execute(ctx, args, execCtx)
	if err != nil || result.IsError {
		t.Fatalf("glob failed: %v %+v", err, result)
	}
	if !strings.Contains(result.Output, "a.txt") || !strings.Contains(result.Output, "b.txt") {
		t.Fatalf("unexpected glob output: %s", result.Output)
	}
}

func TestTodoWriteTool(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: tmp}

	args, _ := json.Marshal(map[string]any{"item": "Test todo item", "checked": false, "path": "TODO.md"})
	result, err := (&TodoWriteTool{}).Execute(ctx, args, execCtx)
	if err != nil || result.IsError {
		t.Fatalf("todo_write failed: %v %+v", err, result)
	}

	content, _ := os.ReadFile(filepath.Join(tmp, "TODO.md"))
	if !strings.Contains(string(content), "- [ ] Test todo item") {
		t.Fatalf("unexpected todo content: %s", string(content))
	}

	argsChecked, _ := json.Marshal(map[string]any{"item": "Done item", "checked": true})
	_, _ = (&TodoWriteTool{}).Execute(ctx, argsChecked, execCtx)
	content, _ = os.ReadFile(filepath.Join(tmp, "TODO.md"))
	if !strings.Contains(string(content), "- [x] Done item") {
		t.Fatalf("unexpected checked todo: %s", string(content))
	}
}

func TestNotebookEditTool(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: tmp}

	args, _ := json.Marshal(map[string]any{
		"path":       "test.ipynb",
		"cell_index": 0,
		"new_source": "print('hello')",
		"cell_type":  "code",
		"mode":       "replace",
	})
	result, err := (&NotebookEditTool{}).Execute(ctx, args, execCtx)
	if err != nil || result.IsError {
		t.Fatalf("notebook_edit failed: %v %+v", err, result)
	}

	content, _ := os.ReadFile(filepath.Join(tmp, "test.ipynb"))
	if !strings.Contains(string(content), "print('hello')") {
		t.Fatalf("unexpected notebook content: %s", string(content))
	}
}

func TestToolSearchTool(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: tmp}

	registry := NewRegistry()
	registry.Register(&ReadTool{})
	registry.Register(&WriteTool{})
	registry.Register(&BashTool{})

	execCtx.Metadata = map[string]any{"tool_registry": registry}

	args, _ := json.Marshal(map[string]any{"query": "file"})
	result, err := (&ToolSearchTool{}).Execute(ctx, args, execCtx)
	if err != nil || result.IsError {
		t.Fatalf("tool_search failed: %v %+v", err, result)
	}
	if !strings.Contains(result.Output, "read_file") {
		t.Fatalf("unexpected search output: %s", result.Output)
	}

	argsNoMatch, _ := json.Marshal(map[string]any{"query": "nonexistent"})
	resultNoMatch, _ := (&ToolSearchTool{}).Execute(ctx, argsNoMatch, execCtx)
	if resultNoMatch.Output != "(no matches)" {
		t.Fatalf("expected no matches: %s", resultNoMatch.Output)
	}
}
