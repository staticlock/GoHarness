package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ExecutionContext is shared runtime context for hook execution.
type ExecutionContext struct {
	CWD             string
	DefaultModel    string
	PromptEvaluator PromptEvaluator
}

// PromptEvaluator evaluates prompt/agent hooks and returns raw model output text.
type PromptEvaluator func(ctx context.Context, prompt, model string, agentMode bool) (string, error)

// Executor executes hooks for lifecycle events.
type Executor struct {
	registry *Registry
	ctx      ExecutionContext
}

// NewExecutor constructs a hook executor.
func NewExecutor(registry *Registry, ctx ExecutionContext) *Executor {
	return &Executor{registry: registry, ctx: ctx}
}

// UpdateRegistry swaps the active hook registry.
func (e *Executor) UpdateRegistry(registry *Registry) {
	e.registry = registry
}

// Execute runs all matching hooks for one event.
func (e *Executor) Execute(ctx context.Context, event Event, payload map[string]any) AggregatedResult {
	if e.registry == nil {
		return AggregatedResult{}
	}
	results := make([]Result, 0)
	for _, hook := range e.registry.Get(event) {
		if !matches(hook, payload) {
			continue
		}
		results = append(results, e.executeOne(ctx, hook, event, payload))
	}
	return AggregatedResult{Results: results}
}

func (e *Executor) executeOne(ctx context.Context, hook Definition, event Event, payload map[string]any) Result {
	switch hook.Type {
	case "command":
		return e.runCommand(ctx, hook, event, payload)
	case "http":
		return e.runHTTP(ctx, hook, event, payload)
	case "prompt", "agent":
		return e.runPromptLike(ctx, hook, hook.Type == "agent", payload)
	default:
		return Result{HookType: hook.Type, Success: true, Output: "unknown hook type ignored"}
	}
}

func (e *Executor) runPromptLike(ctx context.Context, hook Definition, agentMode bool, payload map[string]any) Result {
	if e.ctx.PromptEvaluator == nil {
		return Result{HookType: hook.Type, Success: false, Blocked: hook.BlockOnFailure, Reason: "prompt hook evaluator is not configured"}
	}
	prompt := injectArguments(hook.Prompt, payload)
	model := strings.TrimSpace(hook.Model)
	if model == "" {
		model = strings.TrimSpace(e.ctx.DefaultModel)
	}
	text, err := e.ctx.PromptEvaluator(ctx, prompt, model, agentMode)
	if err != nil {
		return Result{HookType: hook.Type, Success: false, Blocked: hook.BlockOnFailure, Reason: err.Error()}
	}
	parsed := parsePromptHookJSON(text)
	if parsed.OK {
		return Result{HookType: hook.Type, Success: true, Output: text}
	}
	reason := parsed.Reason
	if strings.TrimSpace(reason) == "" {
		reason = "hook rejected the event"
	}
	return Result{HookType: hook.Type, Success: false, Output: text, Blocked: hook.BlockOnFailure, Reason: reason}
}

func (e *Executor) runCommand(ctx context.Context, hook Definition, event Event, payload map[string]any) Result {
	command := injectArguments(hook.Command, payload)
	timeout := hook.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := shellCommand(cmdCtx, command)
	cmd.Dir = filepath.Clean(e.ctx.CWD)
	cmd.Env = append(os.Environ(), "OPENHARNESS_HOOK_EVENT="+string(event), "OPENHARNESS_HOOK_PAYLOAD="+mustJSON(payload))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if cmdCtx.Err() == context.DeadlineExceeded {
		return Result{HookType: hook.Type, Success: false, Blocked: hook.BlockOnFailure, Reason: fmt.Sprintf("command hook timed out after %ds", timeout)}
	}
	output := strings.TrimSpace(strings.TrimSpace(stdout.String()) + "\n" + strings.TrimSpace(stderr.String()))
	success := err == nil
	reason := output
	if reason == "" && !success {
		reason = "command hook failed"
	}
	return Result{HookType: hook.Type, Success: success, Output: output, Blocked: hook.BlockOnFailure && !success, Reason: reason}
}

func (e *Executor) runHTTP(ctx context.Context, hook Definition, event Event, payload map[string]any) Result {
	timeout := hook.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}
	httpClient := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	body, _ := json.Marshal(map[string]any{"event": event, "payload": payload})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		return Result{HookType: hook.Type, Success: false, Blocked: hook.BlockOnFailure, Reason: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hook.Headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return Result{HookType: hook.Type, Success: false, Blocked: hook.BlockOnFailure, Reason: err.Error()}
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	output := buf.String()
	reason := output
	if reason == "" {
		reason = fmt.Sprintf("http hook returned %d", resp.StatusCode)
	}
	return Result{HookType: hook.Type, Success: success, Output: output, Blocked: hook.BlockOnFailure && !success, Reason: reason, Metadata: map[string]any{"status_code": resp.StatusCode}}
}

func matches(hook Definition, payload map[string]any) bool {
	if strings.TrimSpace(hook.Matcher) == "" {
		return true
	}
	subject := asSubject(payload)
	ok, err := filepath.Match(hook.Matcher, subject)
	return err == nil && ok
}

func asSubject(payload map[string]any) string {
	if v, ok := payload["tool_name"]; ok {
		return fmt.Sprintf("%v", v)
	}
	if v, ok := payload["prompt"]; ok {
		return fmt.Sprintf("%v", v)
	}
	if v, ok := payload["event"]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func injectArguments(template string, payload map[string]any) string {
	return strings.ReplaceAll(template, "$ARGUMENTS", mustJSON(payload))
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "/bin/bash", "-lc", command)
}

type promptHookResponse struct {
	OK     bool   `json:"ok"`
	Reason string `json:"reason"`
}

func parsePromptHookJSON(text string) promptHookResponse {
	var parsed promptHookResponse
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		if parsed.OK || strings.TrimSpace(parsed.Reason) != "" {
			return parsed
		}
	}
	lowered := strings.ToLower(strings.TrimSpace(text))
	if lowered == "ok" || lowered == "true" || lowered == "yes" {
		return promptHookResponse{OK: true}
	}
	if strings.TrimSpace(text) == "" {
		return promptHookResponse{OK: false, Reason: "hook returned invalid JSON"}
	}
	return promptHookResponse{OK: false, Reason: strings.TrimSpace(text)}
}
