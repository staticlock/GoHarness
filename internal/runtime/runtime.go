package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/staticlock/GoHarness/internal/api"
	"github.com/staticlock/GoHarness/internal/config"
	"github.com/staticlock/GoHarness/internal/coordinator"
	"github.com/staticlock/GoHarness/internal/engine"
	"github.com/staticlock/GoHarness/internal/hooks"
	"github.com/staticlock/GoHarness/internal/mcp"
	"github.com/staticlock/GoHarness/internal/permissions"
	"github.com/staticlock/GoHarness/internal/plugins"
	"github.com/staticlock/GoHarness/internal/prompts"
	"github.com/staticlock/GoHarness/internal/tasks"
	"github.com/staticlock/GoHarness/internal/tools"
)

// RuntimeBundle groups long-lived runtime objects for a session.
type RuntimeBundle struct {
	CWD          string
	ToolRegistry *tools.Registry
	Engine       *engine.QueryEngine
	MCPManager   *mcp.ClientManager
	HookExecutor *hooks.Executor
	mcpToolKeys  map[string]string
}

// BuildOptions carries optional runtime callback/client overrides.
type BuildOptions struct {
	APIClient        engine.SupportsStreamingMessages
	PermissionPrompt engine.PermissionPrompt
	AskUserPrompt    engine.AskUserPrompt
}

// BuildRuntime assembles a runtime bundle with core tools and permission checks.
func BuildRuntime(cwd, model, baseURL, systemPrompt, apiKey, permissionMode string, maxTurns int) (*RuntimeBundle, error) {
	return BuildRuntimeWithOptions(cwd, model, baseURL, systemPrompt, apiKey, permissionMode, maxTurns, BuildOptions{})
}

// BuildRuntimeWithOptions assembles a runtime bundle with optional prompt/client wiring.
func BuildRuntimeWithOptions(cwd, model, baseURL, systemPrompt, apiKey, permissionMode string, maxTurns int, opts BuildOptions) (*RuntimeBundle, error) {
	settings, err := config.LoadSettings()
	if err != nil {
		return nil, err
	}
	if model != "" {
		settings.Model = model
	}
	if baseURL != "" {
		settings.BaseURL = baseURL
	}
	if systemPrompt != "" {
		settings.SystemPrompt = systemPrompt
	}
	if apiKey != "" {
		settings.APIKey = apiKey
	}
	if permissionMode != "" {
		settings.Permission.Mode = permissionMode
	}

	if cwd == "" {
		cwd, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	if settings.SystemPrompt == "" {
		settings.SystemPrompt = prompts.BuildSystemPrompt("", cwd)
	}

	pluginHooks, pluginMCP := plugins.LoadPluginExtensions(settings, cwd)
	mergedMCP := map[string]interface{}{}
	for name, raw := range settings.MCPServers {
		mergedMCP[name] = raw
	}
	for name, raw := range pluginMCP {
		mergedMCP[name] = raw
	}

	mcpManager := mcp.NewClientManager(mcp.LoadServerConfigsFromMap(mergedMCP))
	mcpManager.ConnectAll()

	apiClient := opts.APIClient
	if apiClient == nil {
		apiClient = selectAPIClient(settings)
	}

	hookRegistry := hooks.LoadRegistry(settings, pluginHooks)
	hookExecutor := hooks.NewExecutor(hookRegistry, hooks.ExecutionContext{
		CWD:          cwd,
		DefaultModel: settings.Model,
		PromptEvaluator: func(ctx context.Context, prompt, model string, agentMode bool) (string, error) {
			_ = agentMode
			return evaluatePromptHookWithAPI(ctx, apiClient, settings, prompt, model)
		},
	})

	registry := tools.NewRegistry()
	taskManager := tasks.DefaultManager()
	registry.Register(&tools.ReadTool{})
	registry.Register(&tools.WriteTool{})
	registry.Register(&tools.EditTool{})
	registry.Register(&tools.GlobTool{})
	registry.Register(&tools.GrepTool{})
	registry.Register(&tools.BashTool{})
	registry.Register(&tools.WebFetchTool{})
	registry.Register(&tools.AskUserQuestionTool{})
	registry.Register(&tools.SkillTool{})
	registry.Register(&tools.TaskCreateTool{Manager: taskManager})
	registry.Register(&tools.TaskListTool{Manager: taskManager})
	registry.Register(&tools.TaskGetTool{Manager: taskManager})
	registry.Register(&tools.TaskUpdateTool{Manager: taskManager})
	registry.Register(&tools.TaskStopTool{Manager: taskManager})
	registry.Register(&tools.TaskOutputTool{Manager: taskManager})
	registry.Register(&tools.WebSearchTool{})
	registry.Register(&tools.ConfigTool{})
	registry.Register(&tools.BriefTool{})
	registry.Register(&tools.SleepTool{})
	registry.Register(&tools.NotebookEditTool{})
	registry.Register(&tools.CronCreateTool{})
	registry.Register(&tools.CronListTool{})
	registry.Register(&tools.CronDeleteTool{})
	registry.Register(&tools.RemoteTriggerTool{})
	registry.Register(&tools.EnterPlanModeTool{})
	registry.Register(&tools.ExitPlanModeTool{})
	registry.Register(&tools.EnterWorktreeTool{})
	registry.Register(&tools.ExitWorktreeTool{})
	registry.Register(&tools.TodoWriteTool{})
	registry.Register(&tools.ToolSearchTool{})
	registry.Register(&tools.ListMcpResourcesTool{Manager: mcpManager})
	registry.Register(&tools.ReadMcpResourceTool{Manager: mcpManager})
	registry.Register(&tools.TeamCreateTool{})
	registry.Register(&tools.TeamDeleteTool{})
	mcpToolKeys := map[string]string{}
	for _, toolInfo := range mcpManager.ListTools() {
		adapter := tools.NewMcpToolAdapter(mcpManager, toolInfo)
		registry.Register(adapter)
		mcpToolKeys[mcpToolKey(toolInfo)] = adapter.Name()
	}

	checker := permissions.NewChecker(settings.Permission)

	qc := engine.QueryContext{
		APIClient:         apiClient,
		ToolRegistry:      registry,
		PermissionChecker: permissionAdapter{checker: checker},
		HookExecutor:      hookAdapter{executor: hookExecutor},
		CWD:               cwd,
		Model:             settings.Model,
		SystemPrompt:      settings.SystemPrompt,
		MaxTokens:         settings.MaxTokens,
		PermissionPrompt:  opts.PermissionPrompt,
		AskUserPrompt:     opts.AskUserPrompt,
		MaxTurns:          maxTurns,
	}

	return &RuntimeBundle{CWD: cwd, ToolRegistry: registry, Engine: engine.NewQueryEngine(qc), MCPManager: mcpManager, HookExecutor: hookExecutor, mcpToolKeys: mcpToolKeys}, nil
}

func evaluatePromptHookWithAPI(ctx context.Context, client engine.SupportsStreamingMessages, settings config.Settings, prompt, model string) (string, error) {
	effectiveModel := strings.TrimSpace(model)
	if effectiveModel == "" {
		effectiveModel = settings.Model
	}
	stream, err := client.StreamMessage(engine.ApiMessageRequest{
		Model:        effectiveModel,
		Messages:     []engine.ConversationMessage{engine.FromUserText(prompt)},
		SystemPrompt: settings.SystemPrompt,
		MaxTokens:    settings.MaxTokens,
		Tools:        nil,
	})
	if err != nil {
		return "", err
	}
	last := ""
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case ev, ok := <-stream:
			if !ok {
				if strings.TrimSpace(last) == "" {
					return "", fmt.Errorf("prompt hook stream completed without assistant content")
				}
				return strings.TrimSpace(last), nil
			}
			if ev.Complete != nil {
				last = ev.Complete.Message.Text
			}
		}
	}
}

func selectAPIClient(settings config.Settings) engine.SupportsStreamingMessages {
	apiKey, err := settings.ResolveAPIKey()
	if err != nil || apiKey == "" {
		return newLocalEchoClient()
	}
	return api.NewClient(apiKey, settings.BaseURL)
}

// StartRuntime emits session start hooks.
func StartRuntime(ctx context.Context, bundle *RuntimeBundle) {
	if bundle == nil {
		return
	}
	RefreshMCPToolsIncremental(bundle)
	if bundle.HookExecutor == nil {
		return
	}
	bundle.HookExecutor.Execute(ctx, hooks.SessionStart, map[string]any{
		"cwd":   bundle.CWD,
		"event": string(hooks.SessionStart),
	})
}

// CloseRuntime closes runtime-owned resources and emits session end hooks.
func CloseRuntime(ctx context.Context, bundle *RuntimeBundle) {
	if bundle == nil {
		return
	}
	if bundle.MCPManager != nil {
		bundle.MCPManager.Close()
	}
	if bundle.HookExecutor != nil {
		bundle.HookExecutor.Execute(ctx, hooks.SessionEnd, map[string]any{
			"cwd":   bundle.CWD,
			"event": string(hooks.SessionEnd),
		})
	}
}

// RefreshMCPToolsIncremental reconnects MCP servers and incrementally updates MCP tools.
func RefreshMCPToolsIncremental(bundle *RuntimeBundle) {
	if bundle == nil || bundle.MCPManager == nil || bundle.ToolRegistry == nil {
		return
	}
	bundle.MCPManager.ReconnectAll()
	latest := bundle.MCPManager.ListTools()
	newKeyMap := map[string]string{}

	for _, info := range latest {
		key := mcpToolKey(info)
		if existingName, ok := bundle.mcpToolKeys[key]; ok {
			newKeyMap[key] = existingName
			continue
		}
		adapter := tools.NewMcpToolAdapter(bundle.MCPManager, info)
		name := adapter.Name()
		if !bundle.ToolRegistry.Has(name) {
			bundle.ToolRegistry.Register(adapter)
		}
		newKeyMap[key] = name
	}

	for oldKey, oldName := range bundle.mcpToolKeys {
		if _, ok := newKeyMap[oldKey]; !ok {
			bundle.ToolRegistry.Unregister(oldName)
		}
	}
	bundle.mcpToolKeys = newKeyMap
}

func mcpToolKey(info mcp.ToolInfo) string {
	return info.ServerName + "::" + info.Name
}

type permissionAdapter struct {
	checker *permissions.Checker
}

func (a permissionAdapter) Evaluate(toolName string, isReadOnly bool, filePath, command string) engine.PermissionDecision {
	d := a.checker.Evaluate(toolName, isReadOnly, filePath, command)
	return engine.PermissionDecision{
		Allowed:              d.Allowed,
		RequiresConfirmation: d.RequiresConfirmation,
		Reason:               d.Reason,
	}
}

type hookAdapter struct {
	executor *hooks.Executor
}

func (a hookAdapter) PreToolUse(toolName string, toolInput map[string]any) (bool, string) {
	if a.executor == nil {
		return false, ""
	}
	result := a.executor.Execute(context.Background(), hooks.PreToolUse, map[string]any{
		"tool_name":  toolName,
		"tool_input": toolInput,
		"event":      string(hooks.PreToolUse),
	})
	return result.IsBlocked(), result.Reason()
}

func (a hookAdapter) PostToolUse(toolName string, toolInput map[string]any, result tools.ToolResult) {
	if a.executor == nil {
		return
	}
	a.executor.Execute(context.Background(), hooks.PostToolUse, map[string]any{
		"tool_name":     toolName,
		"tool_input":    toolInput,
		"tool_output":   result.Output,
		"tool_is_error": result.IsError,
		"event":         string(hooks.PostToolUse),
	})
}

// HandleLine submits one line to the engine and streams events.
func HandleLine(ctx context.Context, bundle *RuntimeBundle, line string, render func(engine.StreamEvent) error) error {
	events, errs := bundle.Engine.SubmitMessage(ctx, line)
	for ev := range events {
		if err := render(ev); err != nil {
			return err
		}
	}
	if err, ok := <-errs; ok && err != nil {
		return err
	}
	return nil
}

// localEchoClient is a temporary API client used until provider migration is complete.
type localEchoClient struct{}

func newLocalEchoClient() *localEchoClient { return &localEchoClient{} }

func (c *localEchoClient) StreamMessage(req engine.ApiMessageRequest) (<-chan engine.ApiStreamEvent, error) {
	out := make(chan engine.ApiStreamEvent, 2)
	go func() {
		defer close(out)
		response := fmt.Sprintf("[go-runtime] %s", lastUserText(req.Messages))
		out <- engine.ApiStreamEvent{TextDelta: response}
		out <- engine.ApiStreamEvent{Complete: &engine.ApiMessageCompleteEvent{
			Message: engine.ConversationMessage{Role: "assistant", Text: response},
			Usage:   engine.UsageSnapshot{},
		}}
	}()
	return out, nil
}

func lastUserText(messages []engine.ConversationMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].Text != "" {
			return messages[i].Text
		}
	}
	return ""
}
