package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Config represents the simplified configuration structure
type Config struct {
	// AI Configuration
	AI AIConfig `json:"ai"`

	// UI Configuration
	UI UIConfig `json:"ui"`

	// Tools Configuration
	Tools ToolsConfig `json:"tools"`

	// Permission Configuration
	Permission PermissionConfig `json:"permission"`

	// Memory Configuration
	Memory MemoryConfig `json:"memory"`

	// Plugin Configuration
	Plugins PluginConfig `json:"plugins"`

	// MCP Configuration
	MCP MCPConfig `json:"mcp"`

	// Performance Configuration
	Performance PerformanceConfig `json:"performance"`

	// Logging Configuration
	Logging LoggingConfig `json:"logging"`
}

// AIConfig contains AI-related configuration
type AIConfig struct {
	Provider     string  `json:"provider" validate:"oneof=anthropic openai"`
	Model        string  `json:"model" validate:"required"`
	APIKey       string  `json:"-"` // Not serialized, handled separately
	MaxTokens    int     `json:"max_tokens" validate:"min=1,max=32000"`
	Temperature  float64 `json:"temperature" validate:"min=0,max=2"`
	TopP         float64 `json:"top_p" validate:"min=0,max=1"`
	BaseURL      string  `json:"base_url"`
	Timeout      int     `json:"timeout" validate:"min=1"`
	SystemPrompt string  `json:"system_prompt"`
}

// UIConfig contains UI-related configuration
type UIConfig struct {
	Theme          string `json:"theme" validate:"oneof=default dark auto"`
	OutputStyle    string `json:"output_style" validate:"oneof=default markdown code"`
	VimMode        bool   `json:"vim_mode"`
	VoiceMode      bool   `json:"voice_mode"`
	FastMode       bool   `json:"fast_mode"`
	ShowTimestamps bool   `json:"show_timestamps"`
	ShowToolCalls  bool   `json:"show_tool_calls"`
	AutoSave       bool   `json:"auto_save"`
	ConfirmExit    bool   `json:"confirm_exit"`
}

// ToolsConfig contains tool-related configuration
type ToolsConfig struct {
	Enabled       []string `json:"enabled"`
	Disabled      []string `json:"disabled"`
	Timeout       int      `json:"timeout" validate:"min=1"`
	MaxConcurrent int      `json:"max_concurrent" validate:"min=1"`
	AutoConfirm   bool     `json:"auto_confirm"`
}

// PermissionConfig contains permission-related configuration
type PermissionConfig struct {
	Mode         string     `json:"mode" validate:"oneof=default plan full_auto"`
	AllowedTools []string   `json:"allowed_tools"`
	DeniedTools  []string   `json:"denied_tools"`
	PathRules    []PathRule `json:"path_rules"`
	DeniedCmds   []string   `json:"denied_commands"`
	AllowedCmds  []string   `json:"allowed_commands"`
}

// PathRule defines a rule for file path permissions
type PathRule struct {
	Pattern string `json:"pattern" validate:"required"`
	Allow   bool   `json:"allow"`
}

// MemoryConfig contains memory-related configuration
type MemoryConfig struct {
	Enabled            bool `json:"enabled"`
	MaxFiles           int  `json:"max_files" validate:"min=1"`
	MaxEntrypointLines int  `json:"max_entrypoint_lines" validate:"min=1"`
	MaxContextLines    int  `json:"max_context_lines" validate:"min=1"`
	RetentionDays      int  `json:"retention_days" validate:"min=1"`
	Compress           bool `json:"compress"`
}

// PluginConfig contains plugin-related configuration
type PluginConfig struct {
	Enabled   map[string]bool `json:"enabled"`
	Directory []string        `json:"directory"`
	AutoLoad  bool            `json:"auto_load"`
	Timeout   int             `json:"timeout" validate:"min=1"`
}

// MCPConfig contains MCP-related configuration
type MCPConfig struct {
	Servers map[string]MCPServer `json:"servers"`
}

// MCPServer defines a MCP server configuration
type MCPServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	Timeout int               `json:"timeout" validate:"min=1"`
}

// PerformanceConfig contains performance-related configuration
type PerformanceConfig struct {
	Enabled     bool `json:"enabled"`
	CacheTTL    int  `json:"cache_ttl" validate:"min=1"`
	MaxRequests int  `json:"max_requests" validate:"min=1"`
	Timeout     int  `json:"timeout" validate:"min=1"`
}

// LoggingConfig contains logging-related configuration
type LoggingConfig struct {
	Level      string `json:"level" validate:"oneof=debug info warn error fatal"`
	File       string `json:"file"`
	Format     string `json:"format" validate:"oneof=text json"`
	MaxSize    string `json:"max_size" validate:"required"`
	MaxBackups int    `json:"max_backups" validate:"min=1"`
	MaxAge     int    `json:"max_age" validate:"min=1"`
}

// ConfigManager manages configuration loading and validation
type ConfigManager struct {
	config     *Config
	configPath string
	mu         sync.RWMutex
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		config:     getDefaultConfig(),
		configPath: getDefaultConfigPath(),
	}
}

// getDefaultConfig returns the default configuration
func getDefaultConfig() *Config {
	return &Config{
		AI: AIConfig{
			Provider:    "anthropic",
			Model:       "claude-sonnet-4-20250514",
			MaxTokens:   16384,
			Temperature: 0.7,
			TopP:        1.0,
			Timeout:     30000,
		},
		UI: UIConfig{
			Theme:          "default",
			OutputStyle:    "default",
			VimMode:        false,
			VoiceMode:      false,
			FastMode:       false,
			ShowTimestamps: true,
			ShowToolCalls:  true,
			AutoSave:       true,
			ConfirmExit:    true,
		},
		Tools: ToolsConfig{
			Enabled:       []string{"file_read", "file_write", "bash", "grep", "web_fetch"},
			Disabled:      []string{},
			Timeout:       30000,
			MaxConcurrent: 3,
			AutoConfirm:   false,
		},
		Permission: PermissionConfig{
			Mode:         "default",
			AllowedTools: []string{},
			DeniedTools:  []string{"rm", "format", "dd"},
			PathRules: []PathRule{
				{Pattern: "**/*.tmp", Allow: false},
				{Pattern: "/etc/**", Allow: false},
			},
			DeniedCmds:  []string{"rm -rf", "format", "dd"},
			AllowedCmds: []string{"git", "npm", "docker", "python", "node"},
		},
		Memory: MemoryConfig{
			Enabled:            true,
			MaxFiles:           5,
			MaxEntrypointLines: 200,
			MaxContextLines:    1000,
			RetentionDays:      7,
			Compress:           true,
		},
		Plugins: PluginConfig{
			Enabled:   make(map[string]bool),
			Directory: []string{filepath.Join(os.Getenv("HOME"), ".openharness", "plugins")},
			AutoLoad:  true,
			Timeout:   5000,
		},
		MCP: MCPConfig{
			Servers: make(map[string]MCPServer),
		},
		Performance: PerformanceConfig{
			Enabled:     true,
			CacheTTL:    3600,
			MaxRequests: 5,
			Timeout:     30000,
		},
		Logging: LoggingConfig{
			Level:      "info",
			File:       "",
			Format:     "text",
			MaxSize:    "100MB",
			MaxBackups: 5,
			MaxAge:     7,
		},
	}
}

// getDefaultConfigPath returns the default configuration file path
func getDefaultConfigPath() string {
	configDir := os.Getenv("OPENHARNESS_CONFIG_DIR")
	if configDir == "" {
		configDir = filepath.Join(os.Getenv("HOME"), ".openharness")
	}
	return filepath.Join(configDir, "config.json")
}

// Load loads configuration from file and environment variables
func (cm *ConfigManager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Load from file if it exists
	if err := cm.loadFromFile(); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Apply environment variable overrides
	cm.applyEnvironmentOverrides()

	// Validate configuration
	if err := cm.validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	return nil
}

// loadFromFile loads configuration from JSON file
func (cm *ConfigManager) loadFromFile() error {
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, create it with defaults
			return cm.saveToFile()
		}
		return err
	}

	return json.Unmarshal(data, cm.config)
}

// saveToFile saves configuration to JSON file
func (cm *ConfigManager) saveToFile() error {
	// Create config directory if it doesn't exist
	configDir := filepath.Dir(cm.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cm.configPath, data, 0644)
}

// applyEnvironmentOverrides applies environment variable overrides
func (cm *ConfigManager) applyEnvironmentOverrides() {
	// AI Configuration
	if provider := os.Getenv("GOHARNESS_AI_PROVIDER"); provider != "" {
		cm.config.AI.Provider = provider
	}
	if model := os.Getenv("GOHARNESS_AI_MODEL"); model != "" {
		cm.config.AI.Model = model
	}
	if maxTokens := os.Getenv("GOHARNESS_AI_MAX_TOKENS"); maxTokens != "" {
		if tokens, err := strconv.Atoi(maxTokens); err == nil && tokens > 0 {
			cm.config.AI.MaxTokens = tokens
		}
	}
	if temperature := os.Getenv("GOHARNESS_AI_TEMPERATURE"); temperature != "" {
		if temp, err := strconv.ParseFloat(temperature, 64); err == nil && temp >= 0 && temp <= 2 {
			cm.config.AI.Temperature = temp
		}
	}
	if timeout := os.Getenv("GOHARNESS_AI_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil && t > 0 {
			cm.config.AI.Timeout = t
		}
	}

	// API Key handling
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		cm.config.AI.APIKey = apiKey
	}
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		cm.config.AI.APIKey = apiKey
	}

	// UI Configuration
	if theme := os.Getenv("GOHARNESS_UI_THEME"); theme != "" {
		cm.config.UI.Theme = theme
	}
	if outputStyle := os.Getenv("GOHARNESS_UI_OUTPUT_STYLE"); outputStyle != "" {
		cm.config.UI.OutputStyle = outputStyle
	}

	// Tools Configuration
	if timeout := os.Getenv("GOHARNESS_TOOLS_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil && t > 0 {
			cm.config.Tools.Timeout = t
		}
	}

	// Permission Configuration
	if mode := os.Getenv("GOHARNESS_PERMISSION_MODE"); mode != "" {
		cm.config.Permission.Mode = mode
	}
}

// validate validates the configuration
func (cm *ConfigManager) validate() error {
	// Validate AI configuration
	if cm.config.AI.Provider == "" {
		return errors.New("AI provider is required")
	}
	if cm.config.AI.Model == "" {
		return errors.New("AI model is required")
	}
	if cm.config.AI.MaxTokens <= 0 || cm.config.AI.MaxTokens > 32000 {
		return errors.New("AI max tokens must be between 1 and 32000")
	}
	if cm.config.AI.Temperature < 0 || cm.config.AI.Temperature > 2 {
		return errors.New("AI temperature must be between 0 and 2")
	}

	// Validate UI configuration
	validThemes := []string{"default", "dark", "auto"}
	if !contains(validThemes, cm.config.UI.Theme) {
		return fmt.Errorf("invalid UI theme: %s", cm.config.UI.Theme)
	}

	validOutputStyles := []string{"default", "markdown", "code"}
	if !contains(validOutputStyles, cm.config.UI.OutputStyle) {
		return fmt.Errorf("invalid UI output style: %s", cm.config.UI.OutputStyle)
	}

	// Validate permission configuration
	validModes := []string{"default", "plan", "full_auto"}
	if !contains(validModes, cm.config.Permission.Mode) {
		return fmt.Errorf("invalid permission mode: %s", cm.config.Permission.Mode)
	}

	// Validate path rules
	for _, rule := range cm.config.Permission.PathRules {
		if rule.Pattern == "" {
			return errors.New("path rule pattern cannot be empty")
		}
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return fmt.Errorf("invalid path rule pattern: %s", rule.Pattern)
		}
	}

	// Validate memory configuration
	if cm.config.Memory.MaxFiles <= 0 {
		return errors.New("memory max files must be greater than 0")
	}
	if cm.config.Memory.MaxEntrypointLines <= 0 {
		return errors.New("memory max entrypoint lines must be greater than 0")
	}

	return nil
}

// Get returns the current configuration
func (cm *ConfigManager) Get() *Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Return a copy to avoid external modifications
	configCopy := *cm.config
	return &configCopy
}

// Set updates the configuration
func (cm *ConfigManager) Set(config *Config) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Validate the new configuration
	if err := validateConfig(config); err != nil {
		return err
	}

	cm.config = config
	return cm.saveToFile()
}

// UpdateField updates a specific field in the configuration
func (cm *ConfigManager) UpdateField(path string, value interface{}) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Use reflection to update the field
	configValue := reflect.ValueOf(cm.config)
	if configValue.Kind() == reflect.Ptr {
		configValue = configValue.Elem()
	}

	fieldNames := strings.Split(path, ".")
	currentValue := configValue

	for i, fieldName := range fieldNames {
		if i == len(fieldNames)-1 {
			// Last field, set the value
			field := currentValue.FieldByName(fieldName)
			if !field.IsValid() {
				return fmt.Errorf("field %s not found", path)
			}

			valueField := reflect.ValueOf(value)
			if field.CanSet() {
				field.Set(valueField)
			} else {
				return fmt.Errorf("field %s is not settable", path)
			}
		} else {
			// Navigate to the next field
			currentValue = currentValue.FieldByName(fieldName)
			if !currentValue.IsValid() {
				return fmt.Errorf("field %s not found", path)
			}
		}
	}

	return cm.saveToFile()
}

// GetAPIKey returns the API key with priority: config > env > error
func (cm *ConfigManager) GetAPIKey() (string, error) {
	if cm.config.AI.APIKey != "" {
		return cm.config.AI.APIKey, nil
	}

	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		return apiKey, nil
	}

	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		return apiKey, nil
	}

	return "", errors.New("no API key found; set ANTHROPIC_API_KEY, OPENAI_API_KEY, or configure in settings")
}

// Reset resets the configuration to defaults
func (cm *ConfigManager) Reset() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.config = getDefaultConfig()
	return cm.saveToFile()
}

// validateConfig validates a configuration structure
func validateConfig(config *Config) error {
	// Create a temporary config manager for validation
	tempCM := &ConfigManager{config: config}
	return tempCM.validate()
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
