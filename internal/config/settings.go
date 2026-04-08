package config

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
)

// PermissionSettings mirrors Python's permission settings JSON structure.
type PermissionSettings struct {
	Mode         string   `json:"mode"`
	AllowedTools []string `json:"allowed_tools"`
	DeniedTools  []string `json:"denied_tools"`
	PathRules    []any    `json:"path_rules"`
	DeniedCmds   []string `json:"denied_commands"`
}

// MemorySettings mirrors Python's memory settings JSON structure.
type MemorySettings struct {
	Enabled            bool `json:"enabled"`
	MaxFiles           int  `json:"max_files"`
	MaxEntrypointLines int  `json:"max_entrypoint_lines"`
}

// Settings mirrors core OpenHarness settings for compatibility.
type Settings struct {
	APIKey         string                 `json:"api_key"`
	Model          string                 `json:"model"`
	MaxTokens      int                    `json:"max_tokens"`
	BaseURL        string                 `json:"base_url,omitempty"`
	SystemPrompt   string                 `json:"system_prompt,omitempty"`
	Permission     PermissionSettings     `json:"permission"`
	Hooks          map[string][]any       `json:"hooks"`
	Memory         MemorySettings         `json:"memory"`
	EnabledPlugins map[string]bool        `json:"enabled_plugins"`
	MCPServers     map[string]interface{} `json:"mcp_servers"`
	Theme          string                 `json:"theme"`
	OutputStyle    string                 `json:"output_style"`
	VimMode        bool                   `json:"vim_mode"`
	VoiceMode      bool                   `json:"voice_mode"`
	FastMode       bool                   `json:"fast_mode"`
	Effort         string                 `json:"effort"`
	Passes         int                    `json:"passes"`
	Verbose        bool                   `json:"verbose"`
}

func defaultSettings() Settings {
	return Settings{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 16384,
		Permission: PermissionSettings{
			Mode:         "default",
			AllowedTools: []string{},
			DeniedTools:  []string{},
			PathRules:    []any{},
			DeniedCmds:   []string{},
		},
		Hooks:          map[string][]any{},
		Memory:         MemorySettings{Enabled: true, MaxFiles: 5, MaxEntrypointLines: 200},
		EnabledPlugins: map[string]bool{},
		MCPServers:     map[string]interface{}{},
		Theme:          "default",
		OutputStyle:    "default",
		Effort:         "medium",
		Passes:         1,
	}
}

func applyEnvOverrides(s Settings) Settings {
	if model := os.Getenv("ANTHROPIC_MODEL"); model != "" {
		s.Model = model
	} else if model := os.Getenv("OPENHARNESS_MODEL"); model != "" {
		s.Model = model
	}

	if baseURL := os.Getenv("ANTHROPIC_BASE_URL"); baseURL != "" {
		s.BaseURL = baseURL
	} else if baseURL := os.Getenv("OPENHARNESS_BASE_URL"); baseURL != "" {
		s.BaseURL = baseURL
	}

	if maxTokens := os.Getenv("OPENHARNESS_MAX_TOKENS"); maxTokens != "" {
		parsed, err := strconv.Atoi(maxTokens)
		if err == nil && parsed > 0 {
			s.MaxTokens = parsed
		}
	}

	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		s.APIKey = apiKey
	}
	return s
}

// LoadSettings reads settings.json and applies env overrides.
func LoadSettings() (Settings, error) {
	path, err := ConfigFilePath()
	if err != nil {
		return Settings{}, err
	}

	s := defaultSettings()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return applyEnvOverrides(s), nil
		}
		return Settings{}, err
	}

	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, err
	}

	return applyEnvOverrides(s), nil
}

// SaveSettings writes settings.json with 2-space indentation.
func SaveSettings(s Settings) error {
	path, err := ConfigFilePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// ResolveAPIKey follows Python behavior: config first, then ANTHROPIC_API_KEY.
func (s Settings) ResolveAPIKey() (string, error) {
	if s.APIKey != "" {
		return s.APIKey, nil
	}
	if env := os.Getenv("ANTHROPIC_API_KEY"); env != "" {
		return env, nil
	}
	return "", errors.New("no API key found; set ANTHROPIC_API_KEY or configure api_key in ~/.openharness/settings.json")
}
