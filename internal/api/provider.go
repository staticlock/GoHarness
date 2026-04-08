package api

import "strings"

// ProviderInfo mirrors provider capability diagnostics used by UI.
type ProviderInfo struct {
	Name           string
	AuthKind       string
	VoiceSupported bool
	VoiceReason    string
}

// DetectProvider infers provider metadata from model/base URL.
func DetectProvider(model, baseURL string) ProviderInfo {
	base := strings.ToLower(strings.TrimSpace(baseURL))
	m := strings.ToLower(strings.TrimSpace(model))
	if strings.Contains(base, "moonshot") || strings.HasPrefix(m, "kimi") {
		return ProviderInfo{Name: "moonshot-anthropic-compatible", AuthKind: "api_key", VoiceReason: "voice mode requires a Claude.ai-style authenticated voice backend"}
	}
	if strings.Contains(base, "bedrock") {
		return ProviderInfo{Name: "bedrock-compatible", AuthKind: "aws", VoiceReason: "voice mode is not wired for Bedrock in this build"}
	}
	if strings.Contains(base, "vertex") || strings.Contains(base, "aiplatform") {
		return ProviderInfo{Name: "vertex-compatible", AuthKind: "gcp", VoiceReason: "voice mode is not wired for Vertex in this build"}
	}
	if base != "" {
		return ProviderInfo{Name: "anthropic-compatible", AuthKind: "api_key", VoiceReason: "voice mode currently requires a dedicated Claude.ai-style provider"}
	}
	return ProviderInfo{Name: "anthropic", AuthKind: "api_key", VoiceReason: "voice mode shell exists, but live voice auth/streaming is not configured in this build"}
}

// AuthStatus returns compact auth status summary.
func AuthStatus(apiKey string) string {
	if strings.TrimSpace(apiKey) != "" {
		return "configured"
	}
	return "missing"
}
