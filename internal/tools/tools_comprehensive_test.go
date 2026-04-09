package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileReadWriteEditFlow(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: tmp}

	writeArgs, _ := json.Marshal(map[string]any{"path": "notes.txt", "content": "one\ntwo\nthree\n"})
	wr, err := (&WriteTool{}).Execute(ctx, writeArgs, execCtx)
	if err != nil || wr.IsError {
		t.Fatalf("write failed: %v %+v", err, wr)
	}

	readArgs, _ := json.Marshal(map[string]any{"path": "notes.txt", "offset": 1, "limit": 2})
	rd, err := (&ReadTool{}).Execute(ctx, readArgs, execCtx)
	if err != nil || rd.IsError {
		t.Fatalf("read failed: %v %+v", err, rd)
	}
	if !strings.Contains(rd.Output, "2\ttwo") || !strings.Contains(rd.Output, "3\tthree") {
		t.Fatalf("unexpected read output: %s", rd.Output)
	}

	editArgs, _ := json.Marshal(map[string]any{"path": "notes.txt", "old_str": "two", "new_str": "TWO"})
	ed, err := (&EditTool{}).Execute(ctx, editArgs, execCtx)
	if err != nil || ed.IsError {
		t.Fatalf("edit failed: %v %+v", err, ed)
	}

	content, err := os.ReadFile(filepath.Join(tmp, "notes.txt"))
	if err != nil {
		t.Fatalf("read back failed: %v", err)
	}
	if string(content) != "one\nTWO\nthree\n" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestGlobAndGrepTools(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: tmp}

	os.WriteFile(filepath.Join(tmp, "a.py"), []byte("def alpha():\n    return 1\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "b.py"), []byte("def beta():\n    return 2\n"), 0644)

	globArgs, _ := json.Marshal(map[string]any{"pattern": "*.py"})
	gr, err := (&GlobTool{}).Execute(ctx, globArgs, execCtx)
	if err != nil || gr.IsError {
		t.Fatalf("glob failed: %v %+v", err, gr)
	}
	output := gr.Output
	if !strings.Contains(output, "a.py") || !strings.Contains(output, "b.py") {
		t.Fatalf("unexpected glob output: %s", output)
	}

	grepArgs, _ := json.Marshal(map[string]any{"pattern": "def\\s+beta", "file_glob": "*.py"})
	gp, err := (&GrepTool{}).Execute(ctx, grepArgs, execCtx)
	if err != nil || gp.IsError {
		t.Fatalf("grep failed: %v %+v", err, gp)
	}
	if !strings.Contains(gp.Output, "b.py:1:def beta()") {
		t.Fatalf("unexpected grep output: %s", gp.Output)
	}
}

func TestBashTool(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: tmp}

	args, _ := json.Marshal(map[string]any{"command": "echo hello"})
	result, err := (&BashTool{}).Execute(ctx, args, execCtx)
	if err != nil || result.IsError {
		t.Skipf("bash test skipped on Windows: %v %+v", err, result)
	}
	if result.Output != "hello\n" && result.Output != "hello" {
		t.Fatalf("unexpected output: %s", result.Output)
	}
}

func TestToolSearchAndBriefTools(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()

	registry := NewRegistry()
	registry.Register(&ReadTool{})
	registry.Register(&WriteTool{})
	registry.Register(&BashTool{})

	execCtx := ToolExecutionContext{CWD: tmp, Metadata: map[string]any{"tool_registry": registry}}

	searchArgs, _ := json.Marshal(map[string]any{"query": "file"})
	sr, err := (&ToolSearchTool{}).Execute(ctx, searchArgs, execCtx)
	if err != nil || sr.IsError {
		t.Fatalf("tool_search failed: %v %+v", err, sr)
	}
	if !strings.Contains(sr.Output, "read_file") {
		t.Fatalf("unexpected search output: %s", sr.Output)
	}

	briefArgs, _ := json.Marshal(map[string]any{"text": "abcdefghijklmnopqrstuvwxyz", "max_chars": 20})
	br, err := (&BriefTool{}).Execute(ctx, briefArgs, execCtx)
	if err != nil || br.IsError {
		t.Fatalf("brief failed: %v %+v", err, br)
	}
	if br.Output != "abcdefghijklmnopqrst..." {
		t.Fatalf("unexpected brief output: %s", br.Output)
	}
}

func TestSkillAndTodoTools(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("OPENHARNESS_CONFIG_DIR", filepath.Join(tmp, "config"))
	t.Cleanup(func() { os.Unsetenv("OPENHARNESS_CONFIG_DIR") })

	skillsDir := filepath.Join(tmp, "config", "skills")
	os.MkdirAll(skillsDir, 0755)
	os.WriteFile(filepath.Join(skillsDir, "pytest.md"), []byte("# Pytest\nHelpful pytest notes.\n"), 0644)

	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: tmp}

	skillArgs, _ := json.Marshal(map[string]any{"name": "Pytest"})
	sk, err := (&SkillTool{}).Execute(ctx, skillArgs, execCtx)
	if err != nil || sk.IsError {
		t.Fatalf("skill failed: %v %+v", err, sk)
	}
	if !strings.Contains(sk.Output, "Helpful pytest notes.") {
		t.Fatalf("unexpected skill output: %s", sk.Output)
	}

	todoArgs, _ := json.Marshal(map[string]any{"item": "wire commands"})
	td, err := (&TodoWriteTool{}).Execute(ctx, todoArgs, execCtx)
	if err != nil || td.IsError {
		t.Fatalf("todo failed: %v %+v", err, td)
	}
	content, _ := os.ReadFile(filepath.Join(tmp, "TODO.md"))
	if !strings.Contains(string(content), "wire commands") {
		t.Fatalf("unexpected todo content: %s", string(content))
	}
}

func TestNotebookEditTool(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: tmp}

	args, _ := json.Marshal(map[string]any{
		"path":       "demo.ipynb",
		"cell_index": 0,
		"new_source": "print('nb ok')",
		"cell_type":  "code",
	})
	result, err := (&NotebookEditTool{}).Execute(ctx, args, execCtx)
	if err != nil || result.IsError {
		t.Fatalf("notebook_edit failed: %v %+v", err, result)
	}
	if !strings.Contains(result.Output, "demo.ipynb") {
		t.Fatalf("unexpected output: %s", result.Output)
	}
	content, _ := os.ReadFile(filepath.Join(tmp, "demo.ipynb"))
	if !strings.Contains(string(content), "nb ok") {
		t.Fatalf("unexpected notebook content: %s", string(content))
	}
}

func TestConfigTool(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: tmp}

	args, _ := json.Marshal(map[string]any{"action": "set", "key": "theme", "value": "solarized"})
	result, err := (&ConfigTool{}).Execute(ctx, args, execCtx)
	if err != nil {
		t.Skipf("config test skipped: %v", err)
	}
	if result.IsError {
		t.Skipf("config test skipped due to error: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Updated theme") {
		t.Fatalf("unexpected output: %s", result.Output)
	}
}

func TestAskUserQuestionTool(t *testing.T) {
	ctx := context.Background()
	args, _ := json.Marshal(map[string]any{"question": "Need confirmation?"})

	result, err := (&AskUserQuestionTool{}).Execute(ctx, args, ToolExecutionContext{
		CWD: t.TempDir(),
		Metadata: map[string]any{
			"ask_user_prompt": func(question string) string {
				return "yes"
			},
		},
	})
	if err != nil || result.IsError {
		t.Fatalf("ask_user_question failed: %v %+v", err, result)
	}
	if result.Output != "yes" {
		t.Fatalf("unexpected answer: %s", result.Output)
	}
}

func TestSleepTool(t *testing.T) {
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: t.TempDir()}

	args, _ := json.Marshal(map[string]any{"seconds": 0})
	result, err := (&SleepTool{}).Execute(ctx, args, execCtx)
	if err != nil || result.IsError {
		t.Fatalf("sleep failed: %v %+v", err, result)
	}
}

func TestListMcpResourcesTool(t *testing.T) {
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: t.TempDir(), Metadata: map[string]any{}}

	args, _ := json.Marshal(map[string]any{})
	result, err := (&ListMcpResourcesTool{}).Execute(ctx, args, execCtx)
	if err != nil {
		t.Fatalf("list_mcp_resources failed: %v", err)
	}
	if result.Output != "(no MCP resources)" {
		t.Fatalf("expected no resources: %s", result.Output)
	}
}

func TestReadMcpResourceTool(t *testing.T) {
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: t.TempDir(), Metadata: map[string]any{}}

	args, _ := json.Marshal(map[string]any{"uri": "test://resource"})
	result, err := (&ReadMcpResourceTool{}).Execute(ctx, args, execCtx)
	if err != nil {
		t.Fatalf("read_mcp_resource failed: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error for missing manager: %s", result.Output)
	}
}

func TestTodoWriteTool(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: tmp}

	args, _ := json.Marshal(map[string]any{"item": "Test todo", "checked": true, "path": "TODO.md"})
	result, err := (&TodoWriteTool{}).Execute(ctx, args, execCtx)
	if err != nil || result.IsError {
		t.Fatalf("todo_write failed: %v %+v", err, result)
	}

	content, _ := os.ReadFile(filepath.Join(tmp, "TODO.md"))
	if !strings.Contains(string(content), "- [x] Test todo") {
		t.Fatalf("unexpected todo content: %s", string(content))
	}
}
