package ui

import (
	"fmt"

	"github.com/user/goharness/internal/bridge"
	"github.com/user/goharness/internal/mcp"
	"github.com/user/goharness/internal/state"
	"github.com/user/goharness/internal/tasks"
)

// 自定义协议前缀，区分普通日志和协议消息
const protocolPrefix = "OHJSON:"

// FrontendRequest is one JSON request sent by the React frontend.
type FrontendRequest struct {
	Type      string `json:"type"`
	Line      string `json:"line,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Allowed   *bool  `json:"allowed,omitempty"`
	Answer    string `json:"answer,omitempty"`
}

// TranscriptItem is one transcript row for frontend rendering.
type TranscriptItem struct {
	Role      string         `json:"role"`
	Text      string         `json:"text"`
	ToolName  string         `json:"tool_name,omitempty"`
	ToolInput map[string]any `json:"tool_input,omitempty"`
	IsError   *bool          `json:"is_error,omitempty"`
}

// TaskSnapshot is a UI-safe task payload.
type TaskSnapshot struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Status      string            `json:"status"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
}

// BackendEvent is one JSON event sent by backend to frontend.
type BackendEvent struct {
	Type           string           `json:"type"`
	Message        string           `json:"message,omitempty"`
	Item           *TranscriptItem  `json:"item,omitempty"`
	State          map[string]any   `json:"state,omitempty"`
	Tasks          []TaskSnapshot   `json:"tasks,omitempty"`
	MCPServers     []map[string]any `json:"mcp_servers,omitempty"`
	BridgeSessions []map[string]any `json:"bridge_sessions,omitempty"`
	Commands       []string         `json:"commands,omitempty"`
	Modal          map[string]any   `json:"modal,omitempty"`
	SelectOptions  []map[string]any `json:"select_options,omitempty"`
	ToolName       string           `json:"tool_name,omitempty"`
	ToolInput      map[string]any   `json:"tool_input,omitempty"`
	Output         string           `json:"output,omitempty"`
	IsError        *bool            `json:"is_error,omitempty"`
}

func taskSnapshots(records []tasks.Record) []TaskSnapshot {
	out := make([]TaskSnapshot, 0, len(records))
	for _, record := range records {
		metadata := map[string]string{}
		for k, v := range record.Metadata {
			metadata[k] = v
		}
		out = append(out, TaskSnapshot{ID: record.ID, Type: string(record.Type), Status: string(record.Status), Description: record.Description, Metadata: metadata})
	}
	return out
}

func statePayload(appState state.AppState) map[string]any {
	return map[string]any{
		"model":           appState.Model,
		"cwd":             appState.CWD,
		"provider":        appState.Provider,
		"auth_status":     appState.AuthStatus,
		"base_url":        appState.BaseURL,
		"permission_mode": formatPermissionMode(appState.PermissionMode),
		"theme":           appState.Theme,
		"vim_enabled":     appState.VimEnabled,
		"voice_enabled":   appState.VoiceEnabled,
		"voice_available": appState.VoiceAvailable,
		"voice_reason":    appState.VoiceReason,
		"fast_mode":       appState.FastMode,
		"effort":          appState.Effort,
		"passes":          appState.Passes,
		"mcp_connected":   appState.MCPConnected,
		"mcp_failed":      appState.MCPFailed,
		"bridge_sessions": appState.BridgeSessions,
		"output_style":    appState.OutputStyle,
		"keybindings":     appState.Keybindings,
	}
}

func mcpServerPayload(statuses []mcp.ConnectionStatus) []map[string]any {
	out := make([]map[string]any, 0, len(statuses))
	for _, server := range statuses {
		out = append(out, map[string]any{
			"name":            server.Name,
			"state":           server.State,
			"detail":          server.Detail,
			"transport":       server.Transport,
			"auth_configured": server.AuthConfigured,
			"tool_count":      len(server.Tools),
			"resource_count":  len(server.Resources),
		})
	}
	return out
}

func bridgeSessionPayload(snapshots []bridge.SessionSnapshot) []map[string]any {
	out := make([]map[string]any, 0, len(snapshots))
	for _, s := range snapshots {
		out = append(out, map[string]any{
			"session_id":  s.SessionID,
			"command":     s.Command,
			"cwd":         s.CWD,
			"pid":         s.PID,
			"status":      s.Status,
			"started_at":  s.StartedAt,
			"output_path": s.OutputPath,
		})
	}
	return out
}

func formatPermissionMode(mode string) string {
	switch mode {
	case "default", "PermissionMode.DEFAULT":
		return "Default"
	case "plan", "PermissionMode.PLAN":
		return "Plan Mode"
	case "full_auto", "PermissionMode.FULL_AUTO":
		return "Auto"
	default:
		return mode
	}
}

func boolPtr(v bool) *bool { return &v }

func commandNames(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, fmt.Sprintf("/%s", item))
	}
	return out
}
