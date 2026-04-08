package state

// AppState is the shared mutable UI/session state snapshot.
type AppState struct {
	Model          string            `json:"model"`
	PermissionMode string            `json:"permission_mode"`
	Theme          string            `json:"theme"`
	CWD            string            `json:"cwd"`
	Provider       string            `json:"provider"`
	AuthStatus     string            `json:"auth_status"`
	BaseURL        string            `json:"base_url"`
	VimEnabled     bool              `json:"vim_enabled"`
	VoiceEnabled   bool              `json:"voice_enabled"`
	VoiceAvailable bool              `json:"voice_available"`
	VoiceReason    string            `json:"voice_reason"`
	FastMode       bool              `json:"fast_mode"`
	Effort         string            `json:"effort"`
	Passes         int               `json:"passes"`
	MCPConnected   int               `json:"mcp_connected"`
	MCPFailed      int               `json:"mcp_failed"`
	BridgeSessions int               `json:"bridge_sessions"`
	OutputStyle    string            `json:"output_style"`
	Keybindings    map[string]string `json:"keybindings"`
}
