package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildBackendCommandIncludesBackendOnlyAndFlags(t *testing.T) {
	tf := &TerminalFrontend{}
	cmd := tf.BuildBackendCommand(BackendParams{
		Cwd:          "C:/work",
		Model:        "gpt-test",
		BaseURL:      "https://example.com",
		SystemPrompt: "be precise",
		APIKey:       "secret",
	})

	if len(cmd) < 2 {
		t.Fatalf("expected backend command parts, got %v", cmd)
	}
	if cmd[1] != "--backend-only" {
		t.Fatalf("expected --backend-only in command, got %v", cmd)
	}
	joined := strings.Join(cmd, " ")
	for _, token := range []string{"--cwd C:/work", "--model gpt-test", "--base-url https://example.com", "--system-prompt be precise", "--api-key secret"} {
		if !strings.Contains(joined, token) {
			t.Fatalf("expected command to contain %q, got %q", token, joined)
		}
	}
}

func TestGetRepoRootFindsGoMod(t *testing.T) {
	root, err := getRepoRoot()
	if err != nil {
		t.Fatalf("getRepoRoot failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("expected go.mod in repo root %q: %v", root, err)
	}
}

func TestResolveNpmPrefersWindowsBinaryWhenAvailable(t *testing.T) {
	path := resolveNpm()
	if path == "" {
		// Environment may not have npm in CI/local machine.
		t.Skip("npm is not available in PATH")
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(path), "npm.cmd") {
		t.Fatalf("expected npm.cmd on windows, got %q", path)
	}
}
