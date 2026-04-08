package tools

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
)

// AskUserQuestionTool asks the interactive user a follow-up question.
type AskUserQuestionTool struct{}

func (t *AskUserQuestionTool) Name() string { return "ask_user_question" }
func (t *AskUserQuestionTool) Description() string {
	return "Ask the interactive user a follow-up question and return the answer."
}
func (t *AskUserQuestionTool) IsReadOnly() bool { return true }

func (t *AskUserQuestionTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"question": map[string]interface{}{
				"type":        "string",
				"description": "The exact question to ask the user",
			},
		},
		"required": []string{"question"},
	}
}

type askUserQuestionInput struct {
	Question string `json:"question"`
}

func (t *AskUserQuestionTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = ctx
	var input askUserQuestionInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}
	promptRaw, ok := execCtx.Metadata["ask_user_prompt"]
	if !ok {
		return ToolResult{Output: "ask_user_question is unavailable in this session", IsError: true}, nil
	}
	answer, ok := invokeAskUserPrompt(promptRaw, input.Question)
	if !ok {
		return ToolResult{Output: "ask_user_question is unavailable in this session", IsError: true}, nil
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return NewSuccessResult("(no response)"), nil
	}
	return NewSuccessResult(answer), nil
}

func invokeAskUserPrompt(prompt any, question string) (string, bool) {
	if direct, ok := prompt.(func(string) string); ok && direct != nil {
		return direct(question), true
	}
	rv := reflect.ValueOf(prompt)
	if !rv.IsValid() || rv.Kind() != reflect.Func || rv.IsNil() {
		return "", false
	}
	rt := rv.Type()
	if rt.NumIn() != 1 || rt.In(0).Kind() != reflect.String || rt.NumOut() != 1 || rt.Out(0).Kind() != reflect.String {
		return "", false
	}
	out := rv.Call([]reflect.Value{reflect.ValueOf(question)})
	return out[0].String(), true
}
