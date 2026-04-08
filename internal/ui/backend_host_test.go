package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/user/goharness/internal/bridge"
	"github.com/user/goharness/internal/engine"
)

func TestRunBackendHostHandlesSlashCommandAndShutdown(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"submit_line","line":"/help"}`,
	}, "\n") + "\n"
	var out strings.Builder

	err := RunBackendHost(context.Background(), BackendHostConfig{CWD: t.TempDir(), In: strings.NewReader(input), Out: &out})
	if err != nil {
		t.Fatalf("backend host run failed: %v", err)
	}

	lines := protocolLines(t, out.String())
	if len(lines) < 2 {
		t.Fatalf("expected protocol events, got %d", len(lines))
	}
	if lines[0]["type"] != "ready" {
		t.Fatalf("expected first event type=ready, got %#v", lines[0]["type"])
	}
	if !containsTranscriptText(lines, "Available commands:") {
		t.Fatalf("expected /help output in transcript")
	}
}

func TestRunBackendHostStreamsAssistantForNormalPrompt(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"submit_line","line":"hello from backend host test"}`,
	}, "\n") + "\n"
	var out strings.Builder

	err := RunBackendHost(context.Background(), BackendHostConfig{CWD: t.TempDir(), In: strings.NewReader(input), Out: &out, APIClient: assistantOnceClient{}})
	if err != nil {
		t.Fatalf("backend host run failed: %v", err)
	}

	lines := protocolLines(t, out.String())
	if !containsEvent(lines, "assistant_complete") {
		t.Fatalf("expected assistant_complete event")
	}
	if !containsEvent(lines, "line_complete") {
		t.Fatalf("expected line_complete event")
	}
}

func TestRunBackendHostReadyIncludesBridgeSessions(t *testing.T) {
	sessionID := "bridge-test-session"
	bridge.DefaultManager().Upsert(bridge.SessionSnapshot{SessionID: sessionID, Command: "echo hi", CWD: t.TempDir(), PID: 1234, Status: "running", StartedAt: 1, OutputPath: "out.log"})
	t.Cleanup(func() { bridge.DefaultManager().Remove(sessionID) })

	var out strings.Builder
	err := RunBackendHost(context.Background(), BackendHostConfig{CWD: t.TempDir(), In: strings.NewReader(""), Out: &out})
	if err != nil {
		t.Fatalf("backend host run failed: %v", err)
	}

	lines := protocolLines(t, out.String())
	if len(lines) == 0 || lines[0]["type"] != "ready" {
		t.Fatalf("expected ready event, got %+v", lines)
	}
	bridgeSessions, _ := lines[0]["bridge_sessions"].([]any)
	if len(bridgeSessions) == 0 {
		t.Fatalf("expected bridge sessions in ready payload")
	}
}

func TestRunBackendHostPermissionModalRoundTrip(t *testing.T) {
	configDir, err := os.MkdirTemp("", "oh-config-perm-")
	if err != nil {
		t.Fatalf("mkdir temp config failed: %v", err)
	}
	if err := os.Setenv("OPENHARNESS_CONFIG_DIR", configDir); err != nil {
		t.Fatalf("set env failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("OPENHARNESS_CONFIG_DIR") })
	cwd, err := os.MkdirTemp("", "oh-backend-perm-")
	if err != nil {
		t.Fatalf("mkdir temp cwd failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	inR, inW := io.Pipe()
	out := newLineCaptureWriter()
	done := make(chan error, 1)
	go func() {
		done <- RunBackendHost(ctx, BackendHostConfig{CWD: cwd, In: inR, Out: out, APIClient: permissionModalClient{}, PermissionMode: "default"})
	}()

	var sawModal, sawComplete, sentShutdown bool
	seenTypes := []string{}
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for backend events (saw types: %v)", seenTypes)
		case line := <-out.Lines:
			if !strings.HasPrefix(line, protocolPrefix) {
				continue
			}
			var event map[string]any
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, protocolPrefix)), &event); err != nil {
				t.Fatalf("failed to decode event: %v", err)
			}
			typeRaw, _ := event["type"].(string)
			seenTypes = append(seenTypes, typeRaw)
			if typeRaw == "ready" {
				go func() {
					_, _ = fmt.Fprintln(inW, `{"type":"submit_line","line":"please write"}`)
				}()
				continue
			}
			if typeRaw == "modal_request" {
				modal, _ := event["modal"].(map[string]any)
				requestID, _ := modal["request_id"].(string)
				kind, _ := modal["kind"].(string)
				if kind == "permission" && requestID != "" {
					sawModal = true
					go func(id string) {
						_, _ = fmt.Fprintf(inW, "{\"type\":\"permission_response\",\"request_id\":\"%s\",\"allowed\":true}\n", id)
					}(requestID)
				}
			}
			if typeRaw == "assistant_complete" {
				message, _ := event["message"].(string)
				if strings.TrimSpace(message) == "" {
					continue
				}
				sawComplete = true
				if !sentShutdown {
					sentShutdown = true
					go func() {
						_, _ = fmt.Fprintln(inW, `{"type":"shutdown"}`)
						_ = inW.Close()
					}()
				}
			}
			if typeRaw == "shutdown" {
				goto donePermission
			}
		}
	}

donePermission:
	if !sawModal {
		t.Fatalf("expected permission modal_request event (saw types: %v)", seenTypes)
	}
	if !sawComplete {
		t.Fatalf("expected assistant_complete event after permission_response")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("backend host run failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting backend host completion")
	}
	return
}

func TestRunBackendHostQuestionModalRoundTrip(t *testing.T) {
	configDir, err := os.MkdirTemp("", "oh-config-q-")
	if err != nil {
		t.Fatalf("mkdir temp config failed: %v", err)
	}
	if err := os.Setenv("OPENHARNESS_CONFIG_DIR", configDir); err != nil {
		t.Fatalf("set env failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("OPENHARNESS_CONFIG_DIR") })
	cwd, err := os.MkdirTemp("", "oh-backend-q-")
	if err != nil {
		t.Fatalf("mkdir temp cwd failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	inR, inW := io.Pipe()
	out := newLineCaptureWriter()
	done := make(chan error, 1)
	go func() {
		done <- RunBackendHost(ctx, BackendHostConfig{CWD: cwd, In: inR, Out: out, APIClient: questionModalClient{}, PermissionMode: "default"})
	}()

	var sawModal, sawComplete, sentShutdown bool
	seenTypes := []string{}
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for backend events (saw types: %v)", seenTypes)
		case line := <-out.Lines:
			if !strings.HasPrefix(line, protocolPrefix) {
				continue
			}
			var event map[string]any
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, protocolPrefix)), &event); err != nil {
				t.Fatalf("failed to decode event: %v", err)
			}
			typeRaw, _ := event["type"].(string)
			seenTypes = append(seenTypes, typeRaw)
			if typeRaw == "ready" {
				go func() {
					_, _ = fmt.Fprintln(inW, `{"type":"submit_line","line":"ask me"}`)
				}()
				continue
			}
			if typeRaw == "modal_request" {
				modal, _ := event["modal"].(map[string]any)
				requestID, _ := modal["request_id"].(string)
				kind, _ := modal["kind"].(string)
				if kind == "question" && requestID != "" {
					sawModal = true
					go func(id string) {
						_, _ = fmt.Fprintf(inW, "{\"type\":\"question_response\",\"request_id\":\"%s\",\"answer\":\"yes\"}\n", id)
					}(requestID)
				}
			}
			if typeRaw == "assistant_complete" {
				message, _ := event["message"].(string)
				if strings.TrimSpace(message) == "" {
					continue
				}
				sawComplete = true
				if !sentShutdown {
					sentShutdown = true
					go func() {
						_, _ = fmt.Fprintln(inW, `{"type":"shutdown"}`)
						_ = inW.Close()
					}()
				}
			}
			if typeRaw == "shutdown" {
				goto doneQuestion
			}
		}
	}

doneQuestion:
	if !sawModal {
		t.Fatalf("expected question modal_request event (saw types: %v)", seenTypes)
	}
	if !sawComplete {
		t.Fatalf("expected assistant_complete event after question_response")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("backend host run failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting backend host completion")
	}
	return
}

type lineCaptureWriter struct {
	mu    sync.Mutex
	buf   string
	Lines chan string
}

func newLineCaptureWriter() *lineCaptureWriter {
	return &lineCaptureWriter{Lines: make(chan string, 1024)}
}

func (w *lineCaptureWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf += string(p)
	for {
		idx := strings.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := w.buf[:idx]
		w.buf = w.buf[idx+1:]
		w.Lines <- line
	}
	return len(p), nil
}

type assistantOnceClient struct{}

func (assistantOnceClient) StreamMessage(req engine.ApiMessageRequest) (<-chan engine.ApiStreamEvent, error) {
	out := make(chan engine.ApiStreamEvent, 1)
	go func() {
		defer close(out)
		out <- engine.ApiStreamEvent{Complete: &engine.ApiMessageCompleteEvent{Message: engine.ConversationMessage{Role: "assistant", Text: "hello from stub"}}}
	}()
	return out, nil
}

type permissionModalClient struct{}

func (permissionModalClient) StreamMessage(req engine.ApiMessageRequest) (<-chan engine.ApiStreamEvent, error) {
	out := make(chan engine.ApiStreamEvent, 1)
	go func() {
		defer close(out)
		if len(req.Messages) > 0 {
			last := req.Messages[len(req.Messages)-1]
			if last.Role == "user" && len(last.ToolResults) > 0 {
				out <- engine.ApiStreamEvent{Complete: &engine.ApiMessageCompleteEvent{Message: engine.ConversationMessage{Role: "assistant", Text: "done"}}}
				return
			}
		}
		out <- engine.ApiStreamEvent{Complete: &engine.ApiMessageCompleteEvent{Message: engine.ConversationMessage{Role: "assistant", ToolUses: []engine.ToolUse{{ID: "1", Name: "write_file", Input: map[string]any{"path": "x.txt", "content": "hello"}}}}}}
	}()
	return out, nil
}

type questionModalClient struct{}

func (questionModalClient) StreamMessage(req engine.ApiMessageRequest) (<-chan engine.ApiStreamEvent, error) {
	out := make(chan engine.ApiStreamEvent, 1)
	go func() {
		defer close(out)
		if len(req.Messages) > 0 {
			last := req.Messages[len(req.Messages)-1]
			if last.Role == "user" && len(last.ToolResults) > 0 {
				if strings.Contains(last.ToolResults[0].Content, "yes") {
					out <- engine.ApiStreamEvent{Complete: &engine.ApiMessageCompleteEvent{Message: engine.ConversationMessage{Role: "assistant", Text: "got answer"}}}
					return
				}
				out <- engine.ApiStreamEvent{Complete: &engine.ApiMessageCompleteEvent{Message: engine.ConversationMessage{Role: "assistant", Text: "missing answer"}}}
				return
			}
		}
		out <- engine.ApiStreamEvent{Complete: &engine.ApiMessageCompleteEvent{Message: engine.ConversationMessage{Role: "assistant", ToolUses: []engine.ToolUse{{ID: "q1", Name: "ask_user_question", Input: map[string]any{"question": "Need confirmation?"}}}}}}
	}()
	return out, nil
}

func protocolLines(t *testing.T, output string) []map[string]any {
	t.Helper()
	rows := strings.Split(strings.TrimSpace(output), "\n")
	events := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if !strings.HasPrefix(row, protocolPrefix) {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(row, protocolPrefix)), &event); err != nil {
			t.Fatalf("failed to decode event: %v", err)
		}
		events = append(events, event)
	}
	return events
}

func containsEvent(events []map[string]any, eventType string) bool {
	for _, event := range events {
		if event["type"] == eventType {
			return true
		}
	}
	return false
}

func containsTranscriptText(events []map[string]any, want string) bool {
	for _, event := range events {
		if event["type"] != "transcript_item" {
			continue
		}
		item, ok := event["item"].(map[string]any)
		if !ok {
			continue
		}
		text, _ := item["text"].(string)
		if strings.Contains(text, want) {
			return true
		}
	}
	return false
}
func TestRandomRequestID(t *testing.T) {
	fmt.Println("Random request ID:", randomRequestID())
}
