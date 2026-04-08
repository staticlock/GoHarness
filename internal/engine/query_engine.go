package engine

import "context"

// QueryEngine owns conversation history and executes the query loop.
type QueryEngine struct {
	ctx QueryContext

	messages   []ConversationMessage
	totalUsage UsageSnapshot
}

// NewQueryEngine constructs a query engine instance.
func NewQueryEngine(qc QueryContext) *QueryEngine {
	return &QueryEngine{ctx: qc, messages: make([]ConversationMessage, 0, 16)}
}

// Messages returns a defensive copy of conversation history.
func (e *QueryEngine) Messages() []ConversationMessage {
	out := make([]ConversationMessage, len(e.messages))
	copy(out, e.messages)
	return out
}

// TotalUsage returns cumulative token usage across turns.
func (e *QueryEngine) TotalUsage() UsageSnapshot {
	return e.totalUsage
}

// Clear clears in-memory history and usage counters.
func (e *QueryEngine) Clear() {
	e.messages = e.messages[:0]
	e.totalUsage = UsageSnapshot{}
}

// SetSystemPrompt updates system prompt for future turns.
func (e *QueryEngine) SetSystemPrompt(prompt string) {
	e.ctx.SystemPrompt = prompt
}

// SystemPrompt returns the currently active system prompt.
func (e *QueryEngine) SystemPrompt() string {
	return e.ctx.SystemPrompt
}

// SetModel updates model for future turns.
func (e *QueryEngine) SetModel(model string) {
	e.ctx.Model = model
}

// SetPermissionChecker updates permission checker for future turns.
func (e *QueryEngine) SetPermissionChecker(checker PermissionChecker) {
	e.ctx.PermissionChecker = checker
}

// LoadMessages replaces in-memory conversation history.
func (e *QueryEngine) LoadMessages(messages []ConversationMessage) {
	e.messages = append([]ConversationMessage{}, messages...)
}

// SubmitMessage appends a user prompt and runs one query loop.
func (e *QueryEngine) SubmitMessage(ctx context.Context, prompt string) (<-chan StreamEvent, <-chan error) {
	e.messages = append(e.messages, FromUserText(prompt))
	events := make(chan StreamEvent)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)

		err := RunQuery(ctx, e.ctx, &e.messages, func(event StreamEvent, usage *UsageSnapshot) error {
			if usage != nil {
				e.totalUsage.Add(*usage)
			}
			events <- event
			return nil
		})
		if err != nil {
			errs <- err
		}
	}()

	return events, errs
}
