package ui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/staticlock/GoHarness/internal/api"
	"github.com/staticlock/GoHarness/internal/bridge"
	"github.com/staticlock/GoHarness/internal/commands"
	"github.com/staticlock/GoHarness/internal/config"
	"github.com/staticlock/GoHarness/internal/engine"
	"github.com/staticlock/GoHarness/internal/runtime"
	"github.com/staticlock/GoHarness/internal/services"
	"github.com/staticlock/GoHarness/internal/state"
	"github.com/staticlock/GoHarness/internal/tasks"
)

const assistantDeltaFlushInterval = 300 * time.Millisecond

// BackendHostConfig contains the runtime options for backend-only mode.
type BackendHostConfig struct {
	Model          string
	BaseURL        string
	SystemPrompt   string
	APIKey         string
	APIClient      engine.SupportsStreamingMessages
	CWD            string
	PermissionMode string
	MaxTurns       int
	In             io.Reader
	Out            io.Writer
}

type processResult struct {
	shouldStop bool
	err        error
}

type backendHost struct {
	ctx      context.Context
	bundle   *runtime.RuntimeBundle // 运行捆绑包
	registry *commands.Registry     // 命令注册表
	in       io.Reader
	out      io.Writer

	requestCh chan FrontendRequest
	doneCh    chan processResult
	busy      bool

	writeMu           sync.Mutex // 确保事件发送的原子性，避免多个事件交错在一起导致前端解析错误
	pendingMu         sync.Mutex
	snapshotMu        sync.Mutex
	pendingP          map[string]chan bool
	pendingQ          map[string]chan string
	lastStateSnapshot string
	lastTasksSnapshot string
}

// RunBackendHost executes the OHJSON protocol loop for the React terminal UI.
func RunBackendHost(ctx context.Context, cfg BackendHostConfig) error {
	in := cfg.In
	if in == nil {
		in = os.Stdin
	}
	out := cfg.Out
	if out == nil {
		out = os.Stdout
	}

	h := &backendHost{
		ctx:       ctx,
		in:        in,
		out:       out,
		requestCh: make(chan FrontendRequest, 32),
		doneCh:    make(chan processResult, 1),
		pendingP:  map[string]chan bool{},
		pendingQ:  map[string]chan string{},
	}

	bundle, err := runtime.BuildRuntimeWithOptions(cfg.CWD, cfg.Model, cfg.BaseURL, cfg.SystemPrompt,
		cfg.APIKey, cfg.PermissionMode, cfg.MaxTurns,
		runtime.BuildOptions{
			APIClient:        cfg.APIClient,
			PermissionPrompt: h.permissionPrompt, // 许可提示 询问用户这个工具操作我能不能用
			AskUserPrompt:    h.askUserPrompt,
		})
	if err != nil {
		return err
	}
	h.bundle = bundle
	h.registry = commands.CreateDefaultRegistry()

	runtime.StartRuntime(ctx, bundle)
	defer runtime.CloseRuntime(ctx, bundle)

	commandNamesRaw := make([]string, 0, len(h.registry.ListCommands()))
	for _, cmd := range h.registry.ListCommands() {
		commandNamesRaw = append(commandNamesRaw, cmd.Name)
	}

	if err := h.emitEvent(BackendEvent{
		Type:           "ready",
		State:          statePayload(buildAppState(h.bundle)),
		Tasks:          taskSnapshots(tasks.DefaultManager().ListTasks("")),
		MCPServers:     mcpServerPayload(h.bundle.MCPManager.ListStatuses()),
		BridgeSessions: bridgeSessionPayload(bridge.DefaultManager().ListSnapshots()),
		Commands:       commandNames(commandNamesRaw),
	}); err != nil {
		return err
	}
	go h.readRequests()
	for {
		if h.requestCh == nil && !h.busy {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case req, ok := <-h.requestCh:
			if !ok {
				h.requestCh = nil
				continue
			}
			stop, err := h.handleFrontendRequest(req)
			if err != nil {
				return err
			}
			if stop {
				return nil
			}
		case done := <-h.doneCh:
			h.busy = false
			if done.err != nil {
				return done.err
			}
			if done.shouldStop {
				return nil
			}
		}
	}
}

func (h *backendHost) readRequests() {
	scanner := bufio.NewScanner(h.in)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req FrontendRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			_ = h.emitEvent(BackendEvent{Type: "error", Message: "Invalid request: " + err.Error()})
			continue
		}
		h.requestCh <- req
	}
	close(h.requestCh)
}

func (h *backendHost) handleFrontendRequest(req FrontendRequest) (bool, error) {
	switch req.Type {
	case "shutdown":
		return true, h.emitEvent(BackendEvent{Type: "shutdown"})
	case "list_sessions":
		return false, h.emitListSessions()
	case "permission_response":
		h.resolvePermission(req.RequestID, req.Allowed != nil && *req.Allowed)
		return false, nil
	case "question_response":
		h.resolveQuestion(req.RequestID, req.Answer)
		return false, nil
	case "submit_line":
		line := strings.TrimSpace(req.Line)
		if line == "" {
			return false, nil
		}
		if h.busy {
			// Frontends may occasionally send a duplicate submit while a turn is in-flight.
			// Ignore it to keep the transcript clean and avoid spurious error banners.
			return false, nil
		}
		h.busy = true
		go func() {
			stop, err := h.processLine(line)
			h.doneCh <- processResult{shouldStop: stop, err: err}
		}()
		return false, nil
	default:
		return false, h.emitEvent(BackendEvent{Type: "error", Message: "Unknown request type: " + req.Type})
	}
}

func (h *backendHost) processLine(line string) (bool, error) {
	if err := h.emitEvent(BackendEvent{Type: "transcript_item",
		Item: &TranscriptItem{Role: "user", Text: line}}); err != nil {
		return false, err
	}

	if cmd, args := h.registry.Lookup(line); cmd != nil {
		res := cmd.Handler(args, commands.Context{Engine: h.bundle.Engine, CWD: h.bundle.CWD, ToolRegistry: h.bundle.ToolRegistry, MCPManager: h.bundle.MCPManager})
		if res.ClearScreen {
			if err := h.emitEvent(BackendEvent{Type: "clear_transcript"}); err != nil {
				return false, err
			}
		}
		if strings.TrimSpace(res.Message) != "" {
			if err := h.emitEvent(BackendEvent{Type: "transcript_item", Item: &TranscriptItem{Role: "system", Text: res.Message}}); err != nil {
				return false, err
			}
		}
		if err := persistSessionSnapshot(h.bundle); err != nil {
			if emitErr := h.emitEvent(BackendEvent{Type: "error", Message: "session snapshot save failed: " + err.Error()}); emitErr != nil {
				return false, emitErr
			}
		}
		if err := h.emitStatusSnapshot(); err != nil {
			return false, err
		}
		if err := h.emitTasksSnapshot(); err != nil {
			return false, err
		}
		if err := h.emitEvent(BackendEvent{Type: "line_complete"}); err != nil {
			return false, err
		}
		if res.ShouldExit {
			if err := h.emitEvent(BackendEvent{Type: "shutdown"}); err != nil {
				return false, err
			}
			return true, nil
		}
		return false, nil
	}

	var pendingDelta strings.Builder
	lastDeltaFlush := time.Now()
	flushDelta := func() error {
		if pendingDelta.Len() == 0 {
			return nil
		}
		text := pendingDelta.String()
		pendingDelta.Reset()
		lastDeltaFlush = time.Now()
		return h.emitEvent(BackendEvent{Type: "assistant_delta", Message: text})
	}

	render := func(ev engine.StreamEvent) error {
		switch e := ev.(type) {
		case engine.AssistantTextDelta:
			pendingDelta.WriteString(e.Text)
			if time.Since(lastDeltaFlush) >= assistantDeltaFlushInterval {
				return flushDelta()
			}
			return nil
		case engine.AssistantTurnComplete:
			if err := flushDelta(); err != nil {
				return err
			}
			msg := strings.TrimSpace(e.Message.Text)
			if len(e.Message.ToolUses) > 0 {
				if msg != "" {
					return h.emitEvent(BackendEvent{Type: "transcript_item", Item: &TranscriptItem{Role: "assistant", Text: msg}})
				}
				return nil
			}
			if msg == "" {
				return nil
			}
			return h.emitEvent(BackendEvent{Type: "assistant_complete", Message: msg, Item: &TranscriptItem{Role: "assistant", Text: msg}})
		case engine.ToolExecutionStarted:
			if err := flushDelta(); err != nil {
				return err
			}
			textBytes, _ := json.Marshal(e.ToolInput)
			return h.emitEvent(BackendEvent{Type: "tool_started", ToolName: e.ToolName, ToolInput: e.ToolInput, Item: &TranscriptItem{Role: "tool", Text: e.ToolName + " " + string(textBytes), ToolName: e.ToolName, ToolInput: e.ToolInput}})
		case engine.ToolExecutionCompleted:
			if err := flushDelta(); err != nil {
				return err
			}
			if err := h.emitEvent(BackendEvent{Type: "tool_completed", ToolName: e.ToolName, Output: e.Output, IsError: boolPtr(e.IsError), Item: &TranscriptItem{Role: "tool_result", Text: e.Output, ToolName: e.ToolName, IsError: boolPtr(e.IsError)}}); err != nil {
				return err
			}
			if err := h.emitTasksSnapshot(); err != nil {
				return err
			}
			return h.emitStatusSnapshot()
		}
		return nil
	}

	if err := runtime.HandleLine(h.ctx, h.bundle, line, render); err != nil {
		if emitErr := h.emitEvent(BackendEvent{Type: "error", Message: err.Error()}); emitErr != nil {
			return false, emitErr
		}
	}
	if pendingDelta.Len() > 0 {
		if err := h.emitEvent(BackendEvent{Type: "assistant_delta", Message: pendingDelta.String()}); err != nil {
			return false, err
		}
	}
	if err := persistSessionSnapshot(h.bundle); err != nil {
		if emitErr := h.emitEvent(BackendEvent{Type: "error", Message: "session snapshot save failed: " + err.Error()}); emitErr != nil {
			return false, emitErr
		}
	}
	if err := h.emitStatusSnapshot(); err != nil {
		return false, err
	}
	if err := h.emitTasksSnapshot(); err != nil {
		return false, err
	}
	if err := h.emitEvent(BackendEvent{Type: "line_complete"}); err != nil {
		return false, err
	}
	return false, nil
}

func (h *backendHost) emitListSessions() error {
	sessions, err := services.ListSessions(h.bundle.CWD, 10)
	if err != nil {
		return h.emitEvent(BackendEvent{Type: "error", Message: err.Error()})
	}
	options := make([]map[string]any, 0, len(sessions))
	for _, item := range sessions {
		ts := time.Unix(int64(item.CreatedAt), 0).Format("01/02 15:04")
		summary := strings.TrimSpace(item.Summary)
		if summary == "" {
			summary = "(no summary)"
		}
		if len(summary) > 50 {
			summary = summary[:50]
		}
		options = append(options, map[string]any{"value": item.SessionID, "label": fmt.Sprintf("%s  %dmsg  %s", ts, item.MessageCount, summary)})
	}
	return h.emitEvent(BackendEvent{Type: "select_request", Modal: map[string]any{"kind": "select", "title": "Resume Session", "submit_prefix": "/resume "}, SelectOptions: options})
}

func (h *backendHost) emitTasksSnapshot() error {
	event := BackendEvent{Type: "tasks_snapshot", Tasks: taskSnapshots(tasks.DefaultManager().ListTasks(""))}
	emit, err := h.shouldEmitSnapshot(&h.lastTasksSnapshot, event)
	if err != nil {
		return err
	}
	if !emit {
		return nil
	}
	return h.emitEvent(event)
}

func (h *backendHost) emitStatusSnapshot() error {
	event := BackendEvent{Type: "state_snapshot", State: statePayload(buildAppState(h.bundle)), MCPServers: mcpServerPayload(h.bundle.MCPManager.ListStatuses()), BridgeSessions: bridgeSessionPayload(bridge.DefaultManager().ListSnapshots())}
	emit, err := h.shouldEmitSnapshot(&h.lastStateSnapshot, event)
	if err != nil {
		return err
	}
	if !emit {
		return nil
	}
	return h.emitEvent(event)
}

func (h *backendHost) shouldEmitSnapshot(cache *string, event BackendEvent) (bool, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return false, err
	}
	serialized := string(payload)
	h.snapshotMu.Lock()
	defer h.snapshotMu.Unlock()
	if *cache == serialized {
		return false, nil
	}
	*cache = serialized
	return true, nil
}

func buildAppState(bundle *runtime.RuntimeBundle) state.AppState {
	settings, _ := config.LoadSettings()
	permissionMode := settings.Permission.Mode
	if permissionMode == "" {
		permissionMode = "default"
	}
	provider := api.DetectProvider(settings.Model, settings.BaseURL)
	mcpConnected := 0
	mcpFailed := 0
	for _, s := range bundle.MCPManager.ListStatuses() {
		if s.State == "connected" {
			mcpConnected++
		}
		if s.State == "failed" {
			mcpFailed++
		}
	}
	return state.AppState{Model: settings.Model, PermissionMode: permissionMode, Theme: settings.Theme, CWD: bundle.CWD, Provider: provider.Name, AuthStatus: api.AuthStatus(settings.APIKey), BaseURL: settings.BaseURL, VimEnabled: settings.VimMode, VoiceEnabled: settings.VoiceMode, VoiceAvailable: provider.VoiceSupported, VoiceReason: provider.VoiceReason, FastMode: settings.FastMode, Effort: settings.Effort, Passes: settings.Passes, MCPConnected: mcpConnected, MCPFailed: mcpFailed, BridgeSessions: len(bridge.DefaultManager().ListSnapshots()), OutputStyle: settings.OutputStyle, Keybindings: map[string]string{}}
}

// 持久会话快照
func persistSessionSnapshot(bundle *runtime.RuntimeBundle) error {
	if bundle == nil || bundle.Engine == nil {
		return nil
	}
	settings, err := config.LoadSettings()
	if err != nil {
		return err
	}
	_, err = services.SaveSessionSnapshot(
		bundle.CWD,
		settings.Model,
		settings.SystemPrompt,
		"",
		bundle.Engine.Messages(),
		bundle.Engine.TotalUsage())
	return err
}

// emitEvent 将事件发送到前端，确保同一时间只有一个事件被发送，避免输出混乱
func (h *backendHost) emitEvent(event BackendEvent) error {
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = io.WriteString(h.out, protocolPrefix+string(payload)+"\n")
	return err
}

// # 用来向前端（用户界面）发送一个权限请求弹窗
// # 包含：请求ID、工具名称（如
// # "execute_command"）、原因（如
// # "用户想删除文件"）
func (h *backendHost) permissionPrompt(toolName, reason string) bool {
	requestID := randomRequestID()
	ch := make(chan bool, 1)
	h.pendingMu.Lock()
	h.pendingP[requestID] = ch
	h.pendingMu.Unlock()
	if err := h.emitEvent(
		BackendEvent{
			Type: "modal_request",
			Modal: map[string]any{
				"kind":       "permission", // permission 表示这是一个权限请求的弹窗
				"request_id": requestID,
				"tool_name":  toolName,
				"reason":     reason,
			},
		}); err != nil {
		return false
	}
	select {
	case allowed := <-ch:
		return allowed
	case <-h.ctx.Done():
		return false
	}
}

func (h *backendHost) askUserPrompt(question string) string {
	requestID := randomRequestID()
	ch := make(chan string, 1)
	h.pendingMu.Lock()
	h.pendingQ[requestID] = ch
	h.pendingMu.Unlock()
	if err := h.emitEvent(
		BackendEvent{
			Type: "modal_request",
			Modal: map[string]any{
				"kind":       "question",
				"request_id": requestID,
				"question":   question,
			},
		}); err != nil {
		return ""
	}
	select {
	case answer := <-ch:
		return answer
	case <-h.ctx.Done():
		return ""
	}
}

func (h *backendHost) resolvePermission(requestID string, allowed bool) {
	h.pendingMu.Lock()
	ch, ok := h.pendingP[requestID]
	if ok {
		delete(h.pendingP, requestID)
	}
	h.pendingMu.Unlock()
	// 将结果发送到等待的通道中，通知 permissionPrompt 函数继续执行
	if ok {
		ch <- allowed
	}
}

func (h *backendHost) resolveQuestion(requestID, answer string) {
	h.pendingMu.Lock()
	ch, ok := h.pendingQ[requestID]
	if ok {
		delete(h.pendingQ, requestID)
	}
	h.pendingMu.Unlock()
	if ok {
		ch <- answer
	}
}

//	func randomRequestID() string {
//		buf := make([]byte, 8)
//		_, _ = rand.Read(buf)
//		return hex.EncodeToString(buf)
//	}
func randomRequestID() string {
	// 生成 UUID v4 (例如: 550e8400-e29b-41d4-a716-446655440000)
	return uuid.New().String()
}
