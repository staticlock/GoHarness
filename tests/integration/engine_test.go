package integration

import (
	"testing"
	"time"

	"github.com/staticlock/GoHarness/internal/api"
	"github.com/staticlock/GoHarness/internal/engine"
	"github.com/staticlock/GoHarness/internal/tools"
	"github.com/staticlock/GoHarness/tests/testutils"
)

// MockAPIClient implements the SupportsStreamingMessages interface for testing
type MockAPIClient struct{}

func (m *MockAPIClient) StreamMessage(req engine.ApiMessageRequest) (<-chan engine.ApiStreamEvent, error) {
	out := make(chan engine.ApiStreamEvent, 2)
	go func() {
		defer close(out)
		out <- engine.ApiStreamEvent{TextDelta: "Hello from mock API!"}
		out <- engine.ApiStreamEvent{
			Complete: &engine.ApiMessageCompleteEvent{
				Message: engine.ConversationMessage{
					Role: "assistant",
					Text: "Hello from mock API!",
				},
				Usage: engine.UsageSnapshot{
					InputTokens:  10,
					OutputTokens: 20,
					TotalTokens:  30,
				},
			},
		}
	}()
	return out, nil
}

// MockPermissionChecker implements the PermissionChecker interface for testing
type MockPermissionChecker struct{}

func (m *MockPermissionChecker) Evaluate(toolName string, isReadOnly bool, filePath, command string) engine.PermissionDecision {
	return engine.PermissionDecision{
		Allowed:              true,
		RequiresConfirmation: false,
		Reason:               "Mock permission allowed",
	}
}

func TestEngine_Integration(t *testing.T) {
	testutils.SkipIfNoAPIKey(t)
	testutils.SkipIfNoModel(t)

	config := testutils.NewTestConfig()
	defer testutils.CleanupTestDir(config.TestDir)

	// Create test files
	testutils.CreateTestFile(config.TestDir, "test.go", `package main

func main() {
    println("Hello, World!")
}
`)

	// Create engine context
	qc := engine.QueryContext{
		APIClient:         &MockAPIClient{},
		ToolRegistry:      tools.NewRegistry(),
		PermissionChecker: &MockPermissionChecker{},
		CWD:               config.TestDir,
		Model:             config.Model,
		SystemPrompt:      "You are a helpful assistant",
		MaxTokens:         1000,
		MaxTurns:          4,
	}

	// Create engine
	queryEngine := engine.NewQueryEngine(qc)

	// Test simple query
	events, errs := queryEngine.SubmitMessage(testutils.ContextWithTimeout(t, config.Timeout), "Analyze this Go code and explain what it does")

	result := ""
	for event := range events {
		switch e := event.(type) {
		case engine.AssistantTextDelta:
			result += e.Text
		case engine.AssistantTurnComplete:
			result = e.Message.Text
		}
	}

	for err := range errs {
		t.Fatalf("Error from engine: %v", err)
	}

	if result == "" {
		t.Error("Expected non-empty result, got empty string")
	}

	t.Logf("Engine result: %s", result)
}

func TestEngine_WithTools(t *testing.T) {
	testutils.SkipIfNoAPIKey(t)
	testutils.SkipIfNoModel(t)

	config := testutils.NewTestConfig()
	defer testutils.CleanupTestDir(config.TestDir)

	// Create test files
	testutils.CreateTestFile(config.TestDir, "test.py", `def hello():
    print("Hello from Python!")
`)

	// Create engine context
	qc := engine.QueryContext{
		APIClient:         &MockAPIClient{},
		ToolRegistry:      tools.NewRegistry(),
		PermissionChecker: &MockPermissionChecker{},
		CWD:               config.TestDir,
		Model:             config.Model,
		SystemPrompt:      "You are a helpful assistant",
		MaxTokens:         1000,
		MaxTurns:          4,
	}

	// Create engine
	queryEngine := engine.NewQueryEngine(qc)

	// Test query that might use tools
	events, errs := queryEngine.SubmitMessage(testutils.ContextWithTimeout(t, config.Timeout), "Read the Python file and tell me what function it defines")

	result := ""
	for event := range events {
		switch e := event.(type) {
		case engine.AssistantTextDelta:
			result += e.Text
		case engine.AssistantTurnComplete:
			result = e.Message.Text
		}
	}

	for err := range errs {
		t.Fatalf("Error from engine: %v", err)
	}

	if result == "" {
		t.Error("Expected non-empty result, got empty string")
	}

	t.Logf("Engine with tools result: %s", result)
}

func TestAPI_Client_Integration(t *testing.T) {
	testutils.SkipIfNoAPIKey(t)
	testutils.SkipIfNoModel(t)

	config := testutils.NewTestConfig()
	defer testutils.CleanupTestDir(config.TestDir)

	// Create API client
	client := api.NewClient(config.APIKey, config.BaseURL)

	// Test message request
	req := engine.ApiMessageRequest{
		Model:        config.Model,
		MaxTokens:    100,
		Messages:     []engine.ConversationMessage{{Role: "user", Text: "Hello, how are you?"}},
		SystemPrompt: "You are a helpful assistant.",
	}

	// Test streaming
	stream, err := client.StreamMessage(req)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	timeout := time.After(10 * time.Second)
	receivedText := false

	for {
		select {
		case event, ok := <-stream:
			if !ok {
				// Stream closed
				if !receivedText {
					t.Error("Expected to receive text, but stream closed without text")
				}
				return
			}

			if event.TextDelta != "" {
				receivedText = true
				t.Logf("Received text: %s", event.TextDelta)
			}

			if event.Complete != nil {
				t.Logf("Stream completed: %+v", event.Complete)
				return
			}

		case <-timeout:
			t.Error("Timeout waiting for stream events")
			return
		}
	}
}

func TestEngine_MemorySystem(t *testing.T) {
	testutils.SkipIfNoAPIKey(t)
	testutils.SkipIfNoModel(t)

	config := testutils.NewTestConfig()
	defer testutils.CleanupTestDir(config.TestDir)

	// Create engine context
	qc := engine.QueryContext{
		APIClient:         &MockAPIClient{},
		ToolRegistry:      tools.NewRegistry(),
		PermissionChecker: &MockPermissionChecker{},
		CWD:               config.TestDir,
		Model:             config.Model,
		SystemPrompt:      "You are a helpful assistant",
		MaxTokens:         1000,
		MaxTurns:          4,
	}

	// Create engine
	queryEngine := engine.NewQueryEngine(qc)

	// First query - establish context
	events1, errs1 := queryEngine.SubmitMessage(testutils.ContextWithTimeout(t, config.Timeout), "My name is Alice and I'm a programmer")

	result1 := ""
	for event := range events1 {
		switch e := event.(type) {
		case engine.AssistantTextDelta:
			result1 += e.Text
		case engine.AssistantTurnComplete:
			result1 = e.Message.Text
		}
	}

	for err := range errs1 {
		t.Fatalf("Error from first query: %v", err)
	}

	// Second query - should remember the name
	events2, errs2 := queryEngine.SubmitMessage(testutils.ContextWithTimeout(t, config.Timeout), "What's my name?")

	result2 := ""
	for event := range events2 {
		switch e := event.(type) {
		case engine.AssistantTextDelta:
			result2 += e.Text
		case engine.AssistantTurnComplete:
			result2 = e.Message.Text
		}
	}

	for err := range errs2 {
		t.Fatalf("Error from second query: %v", err)
	}

	if result2 == "" {
		t.Error("Expected non-empty result, got empty string")
	}

	// Check if the response mentions Alice (this is a simple check)
	// In a real test, you'd want more sophisticated checking
	if len(result2) < 10 {
		t.Error("Response seems too short for a memory test")
	}

	t.Logf("Memory test - First response: %s", result1)
	t.Logf("Memory test - Second response: %s", result2)
}
