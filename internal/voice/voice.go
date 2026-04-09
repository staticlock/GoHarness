package voice

import (
	"regexp"
	"sort"
)

type VoiceDiagnostics struct {
	Available bool
	Reason    string
	Recorder  string
}

func ToggleVoiceMode(enabled bool) bool {
	return !enabled
}

func InspectVoiceCapabilities(voiceSupported bool, voiceReason string) VoiceDiagnostics {
	recorder := findRecorder()
	if !voiceSupported {
		return VoiceDiagnostics{
			Available: false,
			Reason:    voiceReason,
			Recorder:  recorder,
		}
	}
	if recorder == "" {
		return VoiceDiagnostics{
			Available: false,
			Reason:    "no supported recorder found (expected sox, ffmpeg, or arecord)",
			Recorder:  "",
		}
	}
	return VoiceDiagnostics{
		Available: true,
		Reason:    "voice shell is available",
		Recorder:  recorder,
	}
}

func ExtractKeyterms(text string) []string {
	re := regexp.MustCompile(`[A-Za-z0-9_]{4,}`)
	matches := re.FindAllString(text, -1)
	seen := make(map[string]bool)
	var result []string
	for _, match := range matches {
		lower := toLower(match)
		if !seen[lower] {
			seen[lower] = true
			result = append(result, lower)
		}
	}
	sort.Strings(result)
	return result
}

func TranscribeStream() string {
	return "(stream transcription not implemented - requires audio input device)"
}

func findRecorder() string {
	recorders := []string{"sox", "ffmpeg", "arecord"}
	for _, r := range recorders {
		if hasCommand(r) {
			return r
		}
	}
	return ""
}

func hasCommand(name string) bool {
	// Simplified check - in real implementation would check PATH
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}
