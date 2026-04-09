package api_test

import (
	"testing"

	api "github.com/staticlock/GoHarness/internal/api"
	"github.com/staticlock/GoHarness/tests/testutils"
)

func TestClient_NewClient(t *testing.T) {
	testutils.SkipIfNoAPIKey(t)

	config := testutils.NewTestConfig()
	defer testutils.CleanupTestDir(config.TestDir)

	client := api.NewClient(config.APIKey, config.BaseURL)
	if client == nil {
		t.Error("Expected client to be created, got nil")
	}
}

func TestClient_NewClientWithEmptyBaseURL(t *testing.T) {
	testutils.SkipIfNoAPIKey(t)

	config := testutils.NewTestConfig()
	defer testutils.CleanupTestDir(config.TestDir)

	client := api.NewClient(config.APIKey, "")
	if client == nil {
		t.Error("Expected client to be created with empty base URL, got nil")
	}
}
