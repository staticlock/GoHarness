package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/staticlock/GoHarness/tests/testutils"
)

func TestLoadSettings_DefaultValues(t *testing.T) {
	// Clear environment variables to test defaults
	oldApiKey := os.Getenv("ANTHROPIC_API_KEY")
	oldModel := os.Getenv("ANTHROPIC_MODEL")
	oldBaseURL := os.Getenv("ANTHROPIC_BASE_URL")

	os.Setenv("ANTHROPIC_API_KEY", "")
	os.Setenv("ANTHROPIC_MODEL", "")
	os.Setenv("ANTHROPIC_BASE_URL", "")
	defer func() {
		os.Setenv("ANTHROPIC_API_KEY", oldApiKey)
		os.Setenv("ANTHROPIC_MODEL", oldModel)
		os.Setenv("ANTHROPIC_BASE_URL", oldBaseURL)
	}()

	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if settings.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Expected default model 'claude-sonnet-4-20250514', got '%s'", settings.Model)
	}

	if settings.MaxTokens != 16384 {
		t.Errorf("Expected default max tokens 16384, got %d", settings.MaxTokens)
	}

	if settings.Permission.Mode != "default" {
		t.Errorf("Expected default permission mode 'default', got '%s'", settings.Permission.Mode)
	}
}

func TestLoadSettings_EnvironmentOverrides(t *testing.T) {
	testutils.CreateTestFile(t.TempDir(), "test.txt", "test content")

	os.Setenv("ANTHROPIC_MODEL", "claude-3-5-sonnet-20241022")
	os.Setenv("ANTHROPIC_BASE_URL", "https://api.anthropic.com")
	os.Setenv("OPENHARNESS_MAX_TOKENS", "8192")
	defer func() {
		os.Setenv("ANTHROPIC_MODEL", "")
		os.Setenv("ANTHROPIC_BASE_URL", "")
		os.Setenv("OPENHARNESS_MAX_TOKENS", "")
	}()

	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if settings.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Expected model from env 'claude-3-5-sonnet-20241022', got '%s'", settings.Model)
	}

	if settings.BaseURL != "https://api.anthropic.com" {
		t.Errorf("Expected base URL from env 'https://api.anthropic.com', got '%s'", settings.BaseURL)
	}

	if settings.MaxTokens != 8192 {
		t.Errorf("Expected max tokens from env 8192, got %d", settings.MaxTokens)
	}
}

func TestLoadSettings_ConfigFile(t *testing.T) {
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "settings.json")

	configContent := `{
  "model": "claude-3-haiku-20240307",
  "max_tokens": 4096,
  "permission": {
    "mode": "plan",
    "allowed_tools": ["file_read", "file_write"],
    "denied_tools": ["bash"]
  },
  "memory": {
    "enabled": true,
    "max_files": 10,
    "max_entrypoint_lines": 300
  }
}`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Mock the config file path to use our test directory
	oldConfigDir := os.Getenv("OPENHARNESS_CONFIG_DIR")
	os.Setenv("OPENHARNESS_CONFIG_DIR", configDir)
	defer func() {
		os.Setenv("OPENHARNESS_CONFIG_DIR", oldConfigDir)
	}()

	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if settings.Model != "claude-3-haiku-20240307" {
		t.Errorf("Expected model from config 'claude-3-haiku-20240307', got '%s'", settings.Model)
	}

	if settings.MaxTokens != 4096 {
		t.Errorf("Expected max tokens from config 4096, got %d", settings.MaxTokens)
	}

	if settings.Permission.Mode != "plan" {
		t.Errorf("Expected permission mode from config 'plan', got '%s'", settings.Permission.Mode)
	}

	if settings.Memory.MaxFiles != 10 {
		t.Errorf("Expected max files from config 10, got %d", settings.Memory.MaxFiles)
	}
}

func TestResolveAPIKey(t *testing.T) {
	tests := []struct {
		name        string
		apiKey      string
		envKey      string
		expectError bool
		expectedKey string
	}{
		{"API key in config", "config-key", "", false, "config-key"},
		{"API key in env", "", "env-key", false, "env-key"},
		{"Both config and env", "config-key", "env-key", false, "config-key"},
		{"No API key", "", "", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			oldApiKey := os.Getenv("ANTHROPIC_API_KEY")
			os.Setenv("ANTHROPIC_API_KEY", tt.envKey)
			defer func() {
				os.Setenv("ANTHROPIC_API_KEY", oldApiKey)
			}()

			settings := Settings{
				APIKey: tt.apiKey,
			}

			key, err := settings.ResolveAPIKey()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if key != tt.expectedKey {
					t.Errorf("Expected key '%s', got '%s'", tt.expectedKey, key)
				}
			}
		})
	}
}
