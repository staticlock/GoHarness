package engine

import "github.com/user/goharness/internal/tools"

// ToolUse represents one model-requested tool invocation. 表示一个模型请求的工具调用
type ToolUse struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

// ToolResultBlock is appended as user content after tool execution. 工具结果块会作为工具执行后的用户内容附加。
type ToolResultBlock struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error"`
}

// ConversationMessage stores turn history in a transport-friendly format.
type ConversationMessage struct {
	Role        string            `json:"role"`
	Text        string            `json:"text,omitempty"`
	ToolUses    []ToolUse         `json:"tool_uses,omitempty"`
	ToolResults []ToolResultBlock `json:"tool_results,omitempty"`
}

// FromUserText creates a user message.
func FromUserText(text string) ConversationMessage {
	return ConversationMessage{Role: "user", Text: text}
}

// UsageSnapshot tracks per-turn token usage.
type UsageSnapshot struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// Add merges usage totals.
func (u *UsageSnapshot) Add(other UsageSnapshot) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.TotalTokens += other.TotalTokens
}

// ApiMessageRequest mirrors the model request payload shape needed by the loop.
type ApiMessageRequest struct {
	Model        string                `json:"model"`
	Messages     []ConversationMessage `json:"messages"`
	SystemPrompt string                `json:"system_prompt"`
	MaxTokens    int                   `json:"max_tokens"`
	Tools        []map[string]any      `json:"tools"`
}

// ApiMessageCompleteEvent is emitted when one model turn completes.
type ApiMessageCompleteEvent struct {
	Message ConversationMessage `json:"message"`
	Usage   UsageSnapshot       `json:"usage"`
}

// ApiStreamEvent is one streamed model event.
type ApiStreamEvent struct {
	TextDelta string                   `json:"text_delta,omitempty"`
	Complete  *ApiMessageCompleteEvent `json:"complete,omitempty"`
}

// SupportsStreamingMessages provides the engine's model-stream abstraction.
type SupportsStreamingMessages interface {
	StreamMessage(req ApiMessageRequest) (<-chan ApiStreamEvent, error)
}

// PermissionDecision captures policy outcomes for a tool call. 捕捉工具调用的策略结果
type PermissionDecision struct {
	Allowed              bool
	RequiresConfirmation bool
	Reason               string
}

// PermissionChecker evaluates whether a tool call may proceed. 评估工具调用是否可以继续。
type PermissionChecker interface {
	Evaluate(toolName string, isReadOnly bool, filePath, command string) PermissionDecision
}

// PermissionPrompt asks whether to allow a confirmation-gated tool call.
type PermissionPrompt func(toolName, reason string) bool

// AskUserPrompt allows tools to request user clarification.
type AskUserPrompt func(question string) string

// HookExecutor is the minimal hook surface used by the query loop.
type HookExecutor interface {
	PreToolUse(toolName string, toolInput map[string]any) (blocked bool, reason string)
	PostToolUse(toolName string, toolInput map[string]any, result tools.ToolResult)
}
