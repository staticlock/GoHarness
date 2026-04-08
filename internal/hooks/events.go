package hooks

// Event names supported by Go OpenHarness runtime.
type Event string

const (
	SessionStart Event = "session_start"
	SessionEnd   Event = "session_end"
	PreToolUse   Event = "pre_tool_use"
	PostToolUse  Event = "post_tool_use"
)
