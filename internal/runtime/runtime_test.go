package runtime

import "context"
import "testing"

import "github.com/user/goharness/internal/config"
import "github.com/user/goharness/internal/engine"
import "github.com/user/goharness/internal/mcp"
import "github.com/user/goharness/internal/tools"

func TestBuildRuntimeInitializesMCPAndHooks(t *testing.T) {
	bundle, err := BuildRuntime(t.TempDir(), "", "", "", "", "", 2)
	if err != nil {
		t.Fatalf("build runtime failed: %v", err)
	}
	if bundle.MCPManager == nil {
		t.Fatalf("expected mcp manager to be initialized")
	}
	if bundle.HookExecutor == nil {
		t.Fatalf("expected hook executor to be initialized")
	}

	StartRuntime(context.Background(), bundle)
	RefreshMCPToolsIncremental(bundle)
	CloseRuntime(context.Background(), bundle)
}

func TestRefreshMCPToolsIncrementalNilSafe(t *testing.T) {
	RefreshMCPToolsIncremental(nil)
}

type fakeMCPTransport struct {
	tools []mcp.ToolInfo
}

func (f *fakeMCPTransport) ListTools() ([]mcp.ToolInfo, error)         { return f.tools, nil }
func (f *fakeMCPTransport) ListResources() ([]mcp.ResourceInfo, error) { return nil, nil }
func (f *fakeMCPTransport) CallTool(toolName string, arguments map[string]any) (string, error) {
	_ = toolName
	_ = arguments
	return "ok", nil
}
func (f *fakeMCPTransport) ReadResource(uri string) (string, error) { _ = uri; return "", nil }
func (f *fakeMCPTransport) Close() error                            { return nil }

func TestRefreshMCPToolsIncrementalDiff(t *testing.T) {
	manager := mcp.NewClientManager(map[string]mcp.ServerConfig{"fixture": {Type: "stdio", Command: "fake"}})
	call := 0
	manager.SetTransportFactory(func(serverName string, cfg mcp.ServerConfig) (mcp.TransportClient, error) {
		_ = serverName
		_ = cfg
		call++
		if call == 1 {
			return &fakeMCPTransport{tools: []mcp.ToolInfo{{ServerName: "fixture", Name: "hello", Description: "v1", InputSchema: map[string]any{"type": "object"}}}}, nil
		}
		return &fakeMCPTransport{tools: []mcp.ToolInfo{{ServerName: "fixture", Name: "goodbye", Description: "v2", InputSchema: map[string]any{"type": "object"}}}}, nil
	})
	manager.ConnectAll()

	registry := tools.NewRegistry()
	keys := map[string]string{}
	for _, info := range manager.ListTools() {
		adapter := tools.NewMcpToolAdapter(manager, info)
		registry.Register(adapter)
		keys[mcpToolKey(info)] = adapter.Name()
	}
	bundle := &RuntimeBundle{ToolRegistry: registry, MCPManager: manager, mcpToolKeys: keys}

	if !bundle.ToolRegistry.Has("hello") {
		t.Fatalf("expected initial mcp tool hello")
	}
	RefreshMCPToolsIncremental(bundle)
	if bundle.ToolRegistry.Has("hello") {
		t.Fatalf("expected stale mcp tool hello to be removed")
	}
	if !bundle.ToolRegistry.Has("goodbye") {
		t.Fatalf("expected new mcp tool goodbye to be registered")
	}
}

type fakeStreamClient struct{}

func (fakeStreamClient) StreamMessage(req engine.ApiMessageRequest) (<-chan engine.ApiStreamEvent, error) {
	out := make(chan engine.ApiStreamEvent, 1)
	go func() {
		defer close(out)
		out <- engine.ApiStreamEvent{Complete: &engine.ApiMessageCompleteEvent{Message: engine.ConversationMessage{Role: "assistant", Text: "ok"}}}
		_ = req
	}()
	return out, nil
}

type fakePromptHookClient struct{}

func (fakePromptHookClient) StreamMessage(req engine.ApiMessageRequest) (<-chan engine.ApiStreamEvent, error) {
	out := make(chan engine.ApiStreamEvent, 1)
	go func() {
		defer close(out)
		_ = req
		out <- engine.ApiStreamEvent{Complete: &engine.ApiMessageCompleteEvent{Message: engine.ConversationMessage{Role: "assistant", Text: `{"ok": true}`}}}
	}()
	return out, nil
}

func TestEvaluatePromptHookWithAPI(t *testing.T) {
	settings := config.Settings{Model: "claude-sonnet", MaxTokens: 256, SystemPrompt: "system"}
	text, err := evaluatePromptHookWithAPI(context.Background(), fakePromptHookClient{}, settings, "validate this", "")
	if err != nil {
		t.Fatalf("evaluatePromptHookWithAPI failed: %v", err)
	}
	if text != `{"ok": true}` {
		t.Fatalf("unexpected prompt hook output: %q", text)
	}
}

func TestBuildRuntimeWithOptionsWiresCallbacks(t *testing.T) {
	bundle, err := BuildRuntimeWithOptions(t.TempDir(), "", "", "", "", "", 2, BuildOptions{
		APIClient: fakeStreamClient{},
		PermissionPrompt: func(toolName, reason string) bool {
			_ = toolName
			_ = reason
			return true
		},
		AskUserPrompt: func(question string) string {
			_ = question
			return "answer"
		},
	})
	if err != nil {
		t.Fatalf("build runtime with options failed: %v", err)
	}
	if bundle == nil || bundle.Engine == nil {
		t.Fatalf("expected runtime bundle with engine")
	}
}
