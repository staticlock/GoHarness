package prompts

import (
	"strings"
	"testing"
)

func TestBuildSystemPromptIncludesEnvironment(t *testing.T) {
	prompt := BuildSystemPrompt("", ".")
	if !strings.Contains(prompt, "# Environment") {
		t.Fatalf("expected environment section in system prompt")
	}
	if !strings.Contains(prompt, "Working directory") {
		t.Fatalf("expected working directory details")
	}
}

func TestBuildSystemPromptUsesCustomPrompt(t *testing.T) {
	prompt := BuildSystemPrompt("custom", ".")
	if !strings.HasPrefix(prompt, "custom") {
		t.Fatalf("expected custom prompt prefix")
	}
}
