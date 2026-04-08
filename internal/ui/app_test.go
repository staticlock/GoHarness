package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunREPLSinglePrompt(t *testing.T) {
	t.Setenv("OPENHARNESS_CONFIG_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "")
	var out bytes.Buffer
	err := RunREPL(ReplOptions{Prompt: "hello", CWD: t.TempDir(), In: strings.NewReader(""), Out: &out})
	if err != nil {
		t.Fatalf("run repl failed: %v", err)
	}
	if !strings.Contains(out.String(), "[go-runtime] hello") {
		t.Fatalf("unexpected repl output: %s", out.String())
	}
}

func TestRunREPLInteractiveCommands(t *testing.T) {
	t.Setenv("OPENHARNESS_CONFIG_DIR", t.TempDir())
	var out bytes.Buffer
	input := strings.NewReader("/help\n/exit\n")
	err := RunREPL(ReplOptions{CWD: t.TempDir(), In: input, Out: &out})
	if err != nil {
		t.Fatalf("run repl failed: %v", err)
	}
	if !strings.Contains(out.String(), "Available commands:") {
		t.Fatalf("expected help output, got: %s", out.String())
	}
}
