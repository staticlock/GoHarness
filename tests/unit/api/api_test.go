package api

import (
	"testing"

	"github.com/staticlock/GoHarness/tests/testutils"
)

func TestClient_NewClient(t *testing.T) {
	testutils.SkipIfNoAPIKey(t)

	config := testutils.NewTestConfig()
	defer testutils.CleanupTestDir(config.TestDir)

	client := NewClient(config.APIKey, config.BaseURL)
	if client == nil {
		t.Error("Expected client to be created, got nil")
	}

	if client.apiKey != config.APIKey {
		t.Errorf("Expected API key %s, got %s", config.APIKey, client.apiKey)
	}
}

func TestClient_UseOpenAI(t *testing.T) {
	testutils.SkipIfNoAPIKey(t)

	config := testutils.NewTestConfig()
	defer testutils.CleanupTestDir(config.TestDir)

	client := NewClient(config.APIKey, config.BaseURL)

	tests := []struct {
		name     string
		model    string
		baseURL  string
		expected bool
	}{
		{"GPT model", "gpt-4-turbo-preview", "", true},
		{"O1 model", "o1-preview", "", true},
		{"O3 model", "o3-mini", "", true},
		{"O4 model", "o4-preview", "", true},
		{"Claude model", "claude-sonnet-4-20250514", "", false},
		{"OpenAI base URL", "claude-sonnet-4-20250514", "https://api.openai.com/v1", true},
		{"OpenRouter base URL", "claude-sonnet-4-20250514", "https://openrouter.ai/api/v1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := struct {
				Model string
			}{Model: tt.model}

			client.baseURL = tt.baseURL

			result := client.useOpenAI(req)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
