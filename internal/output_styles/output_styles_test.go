package output_styles

import (
	"strings"
	"testing"
)

func TestGetOutputStylesDir(t *testing.T) {
	dir, err := GetOutputStylesDir()
	if err != nil {
		t.Fatalf("GetOutputStylesDir failed: %v", err)
	}
	if !strings.Contains(dir, ".openharness") {
		t.Fatalf("unexpected dir: %s", dir)
	}
}

func TestLoadOutputStyles(t *testing.T) {
	styles, err := LoadOutputStyles()
	if err != nil {
		t.Fatalf("LoadOutputStyles failed: %v", err)
	}
	if len(styles) < 2 {
		t.Fatalf("expected at least builtin styles, got %d", len(styles))
	}

	foundDefault := false
	foundMinimal := false
	for _, s := range styles {
		if s.Name == "default" {
			foundDefault = true
		}
		if s.Name == "minimal" {
			foundMinimal = true
		}
	}
	if !foundDefault {
		t.Fatalf("expected default style")
	}
	if !foundMinimal {
		t.Fatalf("expected minimal style")
	}
}

func TestOutputStyleFields(t *testing.T) {
	styles, _ := LoadOutputStyles()
	defaultStyle := styles[0]

	if defaultStyle.Name == "" {
		t.Fatalf("expected non-empty name")
	}
	if defaultStyle.Source != "builtin" {
		t.Fatalf("expected builtin source")
	}
}
