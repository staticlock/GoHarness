package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ConfigValidator provides validation for configuration
type ConfigValidator struct {
	config *Config
}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator(config *Config) *ConfigValidator {
	return &ConfigValidator{config: config}
}

// Validate validates the entire configuration
func (cv *ConfigValidator) Validate() error {
	var errors []string

	// Validate AI configuration
	if err := cv.validateAIConfig(); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate UI configuration
	if err := cv.validateUIConfig(); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate tools configuration
	if err := cv.validateToolsConfig(); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate permission configuration
	if err := cv.validatePermissionConfig(); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate memory configuration
	if err := cv.validateMemoryConfig(); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate plugin configuration
	if err := cv.validatePluginConfig(); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate MCP configuration
	if err := cv.validateMCPConfig(); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate performance configuration
	if err := cv.validatePerformanceConfig(); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate logging configuration
	if err := cv.validateLoggingConfig(); err != nil {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

// validateAIConfig validates AI configuration
func (cv *ConfigValidator) validateAIConfig() error {
	ai := cv.config.AI

	// Validate provider
	validProviders := []string{"anthropic", "openai"}
	if !contains(validProviders, ai.Provider) {
		return fmt.Errorf("invalid AI provider: %s", ai.Provider)
	}

	// Validate model
	if ai.Model == "" {
		return fmt.Errorf("AI model is required")
	}

	// Validate max tokens
	if ai.MaxTokens <= 0 || ai.MaxTokens > 32000 {
		return fmt.Errorf("AI max tokens must be between 1 and 32000")
	}

	// Validate temperature
	if ai.Temperature < 0 || ai.Temperature > 2 {
		return fmt.Errorf("AI temperature must be between 0 and 2")
	}

	// Validate timeout
	if ai.Timeout <= 0 {
		return fmt.Errorf("AI timeout must be greater than 0")
	}

	// Validate base URL if provided
	if ai.BaseURL != "" {
		if !strings.HasPrefix(ai.BaseURL, "http://") && !strings.HasPrefix(ai.BaseURL, "https://") {
			return fmt.Errorf("AI base URL must be a valid HTTP/HTTPS URL")
		}
	}

	return nil
}

// validateUIConfig validates UI configuration
func (cv *ConfigValidator) validateUIConfig() error {
	ui := cv.config.UI

	// Validate theme
	validThemes := []string{"default", "dark", "auto"}
	if !contains(validThemes, ui.Theme) {
		return fmt.Errorf("invalid UI theme: %s", ui.Theme)
	}

	// Validate output style
	validStyles := []string{"default", "markdown", "code"}
	if !contains(validStyles, ui.OutputStyle) {
		return fmt.Errorf("invalid UI output style: %s", ui.OutputStyle)
	}

	return nil
}

// validateToolsConfig validates tools configuration
func (cv *ConfigValidator) validateToolsConfig() error {
	tools := cv.config.Tools

	// Validate timeout
	if tools.Timeout <= 0 {
		return fmt.Errorf("tools timeout must be greater than 0")
	}

	// Validate max concurrent
	if tools.MaxConcurrent <= 0 {
		return fmt.Errorf("tools max concurrent must be greater than 0")
	}

	// Validate enabled tools
	for _, tool := range tools.Enabled {
		if tool == "" {
			return fmt.Errorf("tool name cannot be empty")
		}
	}

	// Validate disabled tools
	for _, tool := range tools.Disabled {
		if tool == "" {
			return fmt.Errorf("tool name cannot be empty")
		}
	}

	return nil
}

// validatePermissionConfig validates permission configuration
func (cv *ConfigValidator) validatePermissionConfig() error {
	permission := cv.config.Permission

	// Validate mode
	validModes := []string{"default", "plan", "full_auto"}
	if !contains(validModes, permission.Mode) {
		return fmt.Errorf("invalid permission mode: %s", permission.Mode)
	}

	// Validate path rules
	for i, rule := range permission.PathRules {
		if rule.Pattern == "" {
			return fmt.Errorf("path rule %d: pattern cannot be empty", i)
		}

		// Validate regex pattern
		if _, err := compileRegex(rule.Pattern); err != nil {
			return fmt.Errorf("path rule %d: invalid pattern '%s': %v", i, rule.Pattern, err)
		}
	}

	// Validate denied commands
	for i, cmd := range permission.DeniedCmds {
		if cmd == "" {
			return fmt.Errorf("denied command %d: command cannot be empty", i)
		}
	}

	// Validate allowed commands
	for i, cmd := range permission.AllowedCmds {
		if cmd == "" {
			return fmt.Errorf("allowed command %d: command cannot be empty", i)
		}
	}

	return nil
}

// validateMemoryConfig validates memory configuration
func (cv *ConfigValidator) validateMemoryConfig() error {
	memory := cv.config.Memory

	// Validate max files
	if memory.MaxFiles <= 0 {
		return fmt.Errorf("memory max files must be greater than 0")
	}

	// Validate max entrypoint lines
	if memory.MaxEntrypointLines <= 0 {
		return fmt.Errorf("memory max entrypoint lines must be greater than 0")
	}

	// Validate max context lines
	if memory.MaxContextLines <= 0 {
		return fmt.Errorf("memory max context lines must be greater than 0")
	}

	// Validate retention days
	if memory.RetentionDays <= 0 {
		return fmt.Errorf("memory retention days must be greater than 0")
	}

	return nil
}

// validatePluginConfig validates plugin configuration
func (cv *ConfigValidator) validatePluginConfig() error {
	plugins := cv.config.Plugins

	// Validate timeout
	if plugins.Timeout <= 0 {
		return fmt.Errorf("plugins timeout must be greater than 0")
	}

	// Validate plugin directories
	for i, dir := range plugins.Directory {
		if dir == "" {
			return fmt.Errorf("plugin directory %d: directory cannot be empty", i)
		}

		// Check if directory exists
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("plugin directory %d: directory does not exist: %s", i, dir)
		}
	}

	return nil
}

// validateMCPConfig validates MCP configuration
func (cv *ConfigValidator) validateMCPConfig() error {
	mcp := cv.config.MCP

	// Validate servers
	for name, server := range mcp.Servers {
		if name == "" {
			return fmt.Errorf("MCP server: server name cannot be empty")
		}

		if server.Command == "" {
			return fmt.Errorf("MCP server '%s': command cannot be empty", name)
		}

		if server.Timeout <= 0 {
			return fmt.Errorf("MCP server '%s': timeout must be greater than 0", name)
		}
	}

	return nil
}

// validatePerformanceConfig validates performance configuration
func (cv *ConfigValidator) validatePerformanceConfig() error {
	perf := cv.config.Performance

	// Validate cache TTL
	if perf.CacheTTL <= 0 {
		return fmt.Errorf("performance cache TTL must be greater than 0")
	}

	// Validate max requests
	if perf.MaxRequests <= 0 {
		return fmt.Errorf("performance max requests must be greater than 0")
	}

	// Validate timeout
	if perf.Timeout <= 0 {
		return fmt.Errorf("performance timeout must be greater than 0")
	}

	return nil
}

// validateLoggingConfig validates logging configuration
func (cv *ConfigValidator) validateLoggingConfig() error {
	log := cv.config.Logging

	// Validate level
	validLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLevels, log.Level) {
		return fmt.Errorf("invalid logging level: %s", log.Level)
	}

	// Validate format
	validFormats := []string{"text", "json"}
	if !contains(validFormats, log.Format) {
		return fmt.Errorf("invalid logging format: %s", log.Format)
	}

	// Validate max size
	if log.MaxSize == "" {
		return fmt.Errorf("logging max size is required")
	}

	// Validate max backups
	if log.MaxBackups <= 0 {
		return fmt.Errorf("logging max backups must be greater than 0")
	}

	// Validate max age
	if log.MaxAge <= 0 {
		return fmt.Errorf("logging max age must be greater than 0")
	}

	// Validate log file path if provided
	if log.File != "" {
		if !filepath.IsAbs(log.File) {
			return fmt.Errorf("logging file path must be absolute: %s", log.File)
		}

		// Create directory if it doesn't exist
		logDir := filepath.Dir(log.File)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create logging directory: %v", err)
		}
	}

	return nil
}

// compileRegex compiles a regex pattern with better error handling
func compileRegex(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, fmt.Errorf("pattern cannot be empty")
	}

	// Convert glob patterns to regex
	if strings.Contains(pattern, "**") {
		pattern = strings.ReplaceAll(pattern, "**", ".*")
	}
	if strings.Contains(pattern, "*") {
		pattern = strings.ReplaceAll(pattern, "*", "[^/]*")
	}
	if strings.Contains(pattern, "?") {
		pattern = strings.ReplaceAll(pattern, "?", "[^/]")
	}

	return regexp.Compile("^" + pattern + "$")
}

// ValidateConfigFile validates a configuration file
func ValidateConfigFile(filePath string) error {
	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse the JSON
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate the configuration
	validator := NewConfigValidator(&config)
	return validator.Validate()
}

// GetConfigSchema returns the configuration schema as JSON
func GetConfigSchema() string {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"ai": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider": map[string]interface{}{
						"type": "string",
						"enum": []string{"anthropic", "openai"},
					},
					"model": map[string]interface{}{
						"type": "string",
					},
					"max_tokens": map[string]interface{}{
						"type":    "integer",
						"minimum": 1,
						"maximum": 32000,
					},
					"temperature": map[string]interface{}{
						"type":    "number",
						"minimum": 0,
						"maximum": 2,
					},
					"timeout": map[string]interface{}{
						"type":    "integer",
						"minimum": 1,
					},
				},
				"required": []string{"provider", "model"},
			},
			"ui": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"theme": map[string]interface{}{
						"type": "string",
						"enum": []string{"default", "dark", "auto"},
					},
					"output_style": map[string]interface{}{
						"type": "string",
						"enum": []string{"default", "markdown", "code"},
					},
				},
			},
			"tools": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timeout": map[string]interface{}{
						"type":    "integer",
						"minimum": 1,
					},
					"max_concurrent": map[string]interface{}{
						"type":    "integer",
						"minimum": 1,
					},
				},
			},
		},
	}

	schemaData, _ := json.MarshalIndent(schema, "", "  ")
	return string(schemaData)
}
