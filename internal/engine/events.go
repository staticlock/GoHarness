package engine

// StreamEvent is the marker interface for all query loop events.
type StreamEvent interface {
	EventType() string
}

// AssistantTextDelta streams incremental assistant text.
type AssistantTextDelta struct {
	Text string
}

func (e AssistantTextDelta) EventType() string { return "assistant_text_delta" }

// AssistantTurnComplete marks completion of a model turn.
type AssistantTurnComplete struct {
	Message ConversationMessage
	Usage   UsageSnapshot
}

func (e AssistantTurnComplete) EventType() string { return "assistant_turn_complete" }

// ToolExecutionStarted is emitted before tool execution.
type ToolExecutionStarted struct {
	ToolName  string
	ToolInput map[string]any
}

func (e ToolExecutionStarted) EventType() string { return "tool_execution_started" }

// ToolExecutionCompleted is emitted after tool execution.
type ToolExecutionCompleted struct {
	ToolName string
	Output   string
	IsError  bool
}

func (e ToolExecutionCompleted) EventType() string { return "tool_execution_completed" }
