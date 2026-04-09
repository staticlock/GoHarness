package testutils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/staticlock/GoHarness/internal/config"
)

// TestConfig provides test configuration
type TestConfig struct {
	APIKey  string
	Model   string
	BaseURL string
	TestDir string
	Timeout time.Duration
}

// NewTestConfig creates a new test configuration
func NewTestConfig() *TestConfig {
	return &TestConfig{
		APIKey:  os.Getenv("GOHARNESS_TEST_API_KEY"),
		Model:   "claude-sonnet-4-20250514",
		BaseURL: "",
		TestDir: createTestDir(),
		Timeout: 30 * time.Second,
	}
}

// createTestDir creates a temporary directory for tests
func createTestDir() string {
	dir, err := os.MkdirTemp("", "goharness-test-*")
	if err != nil {
		panic(fmt.Sprintf("Failed to create test directory: %v", err))
	}
	return dir
}

// CleanupTestDir cleans up the test directory
func CleanupTestDir(testDir string) {
	if testDir != "" {
		os.RemoveAll(testDir)
	}
}

// CreateTestFile creates a test file with given content
func CreateTestFile(dir, filename, content string) string {
	filePath := filepath.Join(dir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		panic(fmt.Sprintf("Failed to create test file %s: %v", filePath, err))
	}
	return filePath
}

// LoadTestConfig loads test configuration from environment
func LoadTestConfig() config.Settings {
	settings, _ := config.LoadSettings()

	// Override with test environment variables
	if apiKey := os.Getenv("GOHARNESS_TEST_API_KEY"); apiKey != "" {
		settings.APIKey = apiKey
	}
	if model := os.Getenv("GOHARNESS_TEST_MODEL"); model != "" {
		settings.Model = model
	}
	if baseURL := os.Getenv("GOHARNESS_TEST_BASE_URL"); baseURL != "" {
		settings.BaseURL = baseURL
	}

	return settings
}

// ContextWithTimeout creates a context with timeout for tests
func ContextWithTimeout(t *testing.T, timeout time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	return ctx
}

// SkipIfNoAPIKey skips test if no API key is available
func SkipIfNoAPIKey(t *testing.T) {
	if os.Getenv("GOHARNESS_TEST_API_KEY") == "" {
		t.Skip("Skipping test: GOHARNESS_TEST_API_KEY not set")
	}
}

// SkipIfNoModel skips test if no model is specified
func SkipIfNoModel(t *testing.T) {
	if os.Getenv("GOHARNESS_TEST_MODEL") == "" {
		t.Skip("Skipping test: GOHARNESS_TEST_MODEL not set")
	}
}
