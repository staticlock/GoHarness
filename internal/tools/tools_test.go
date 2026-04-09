package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWriteEditFlow(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: tmp}

	writeArgs, _ := json.Marshal(map[string]any{"path": "a.txt", "content": "hello\nworld\n"})
	wr, err := (&WriteTool{}).Execute(ctx, writeArgs, execCtx)
	if err != nil || wr.IsError {
		t.Fatalf("write failed: %v %+v", err, wr)
	}

	readArgs, _ := json.Marshal(map[string]any{"path": "a.txt", "offset": 0, "limit": 2})
	rd, err := (&ReadTool{}).Execute(ctx, readArgs, execCtx)
	if err != nil || rd.IsError {
		t.Fatalf("read failed: %v %+v", err, rd)
	}
	if !strings.Contains(rd.Output, "1\thello") || !strings.Contains(rd.Output, "2\tworld") {
		t.Fatalf("unexpected read output: %s", rd.Output)
	}

	editArgs, _ := json.Marshal(map[string]any{"path": "a.txt", "old_str": "world", "new_str": "go"})
	ed, err := (&EditTool{}).Execute(ctx, editArgs, execCtx)
	if err != nil || ed.IsError {
		t.Fatalf("edit failed: %v %+v", err, ed)
	}

	content, err := os.ReadFile(filepath.Join(tmp, "a.txt"))
	if err != nil {
		t.Fatalf("read back failed: %v", err)
	}
	if string(content) != "hello\ngo\n" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestGrepAndWebFetch(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "x.txt"), []byte("alpha\nBETA\n"), 0o644); err != nil {
		t.Fatalf("seed file failed: %v", err)
	}

	ctx := context.Background()
	execCtx := ToolExecutionContext{CWD: tmp}
	grepArgs, _ := json.Marshal(map[string]any{
		"pattern":        "beta",
		"case_sensitive": false,
		"file_glob":      "**/*",
	})
	gr, err := (&GrepTool{}).Execute(ctx, grepArgs, execCtx)
	if err != nil || gr.IsError {
		t.Fatalf("grep failed: %v %+v", err, gr)
	}
	if !strings.Contains(gr.Output, "x.txt:2:BETA") {
		t.Fatalf("unexpected grep output: %s", gr.Output)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body><h1>Hello</h1><p>World</p></body></html>"))
	}))
	defer srv.Close()

	wfArgs, _ := json.Marshal(map[string]any{"url": srv.URL, "max_chars": 200})
	wf, err := (&WebFetchTool{Client: srv.Client()}).Execute(ctx, wfArgs, execCtx)
	if err != nil || wf.IsError {
		t.Fatalf("web_fetch failed: %v %+v", err, wf)
	}
	if !strings.Contains(wf.Output, "Status: 200") || !strings.Contains(wf.Output, "Hello World") {
		t.Fatalf("unexpected web_fetch output: %s", wf.Output)
	}
}

func TestSkillTool(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Setenv("OPENHARNESS_CONFIG_DIR", filepath.Join(tmp, "cfg")); err != nil {
		t.Fatalf("set env failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("OPENHARNESS_CONFIG_DIR") })

	userSkillsDir := filepath.Join(tmp, "cfg", "skills")
	if err := os.MkdirAll(userSkillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userSkillsDir, "demo.md"), []byte("# Demo\nSkill content line."), 0o644); err != nil {
		t.Fatalf("write skill failed: %v", err)
	}

	args, _ := json.Marshal(map[string]any{"name": "Demo"})
	result, err := (&SkillTool{}).Execute(context.Background(), args, ToolExecutionContext{CWD: tmp})
	if err != nil || result.IsError {
		t.Fatalf("skill tool failed: %v %+v", err, result)
	}
	if !strings.Contains(result.Output, "Skill content line.") {
		t.Fatalf("unexpected skill output: %s", result.Output)
	}
}
