package voice

import (
	"testing"
)

func TestToggleVoiceMode(t *testing.T) {
	if ToggleVoiceMode(true) != false {
		t.Fatalf("toggle true should return false")
	}
	if ToggleVoiceMode(false) != true {
		t.Fatalf("toggle false should return true")
	}
}

func TestInspectVoiceCapabilities(t *testing.T) {
	diag := InspectVoiceCapabilities(false, "provider not supported")
	if diag.Available != false {
		t.Fatalf("expected unavailable when provider not supported")
	}
	if diag.Reason != "provider not supported" {
		t.Fatalf("unexpected reason: %s", diag.Reason)
	}

	diag = InspectVoiceCapabilities(true, "")
	if diag.Available {
		t.Fatalf("expected unavailable when no recorder")
	}
}

func TestExtractKeyterms(t *testing.T) {
	text := "Python programming language and Go programming language"
	terms := ExtractKeyterms(text)
	if len(terms) == 0 {
		t.Fatalf("expected keyterms from text")
	}

	foundPython := false
	foundGo := false
	for _, t := range terms {
		if t == "python" {
			foundPython = true
		}
		if t == "programming" {
			_ = foundGo // programming is in the text too
		}
	}
	if !foundPython {
		t.Fatalf("expected python in terms")
	}
}

func TestTranscribeStream(t *testing.T) {
	result := TranscribeStream()
	if result == "" {
		t.Fatalf("expected non-empty stream response")
	}
}
