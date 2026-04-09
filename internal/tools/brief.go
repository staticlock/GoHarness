package tools

import (
	"context"
	"encoding/json"
	"strings"
)

type BriefTool struct{}

func (t *BriefTool) Name() string { return "brief" }
func (t *BriefTool) Description() string {
	return "Shorten a piece of text for compact display."
}
func (t *BriefTool) IsReadOnly() bool { return true }

func (t *BriefTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Text to shorten",
			},
			"max_chars": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum characters (20-2000)",
				"default":     200,
			},
		},
		"required": []string{"text"},
	}
}

type briefInput struct {
	Text     string `json:"text"`
	MaxChars int    `json:"max_chars,omitempty"`
}

func (t *BriefTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = ctx
	_ = execCtx

	var input briefInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	if input.MaxChars <= 0 {
		input.MaxChars = 200
	}
	if input.MaxChars < 20 {
		input.MaxChars = 20
	}
	if input.MaxChars > 2000 {
		input.MaxChars = 2000
	}

	text := strings.TrimSpace(input.Text)
	if len(text) <= input.MaxChars {
		return NewSuccessResult(text), nil
	}

	truncated := text[:input.MaxChars]
	truncated = strings.TrimRight(truncated, " \t\n")
	return NewSuccessResult(truncated + "..."), nil
}
