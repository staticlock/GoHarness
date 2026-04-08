package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/user/goharness/internal/tools"
)

// QueryContext carries dependencies and options for one query run.
type QueryContext struct {
	APIClient         SupportsStreamingMessages
	ToolRegistry      *tools.Registry
	PermissionChecker PermissionChecker
	CWD               string
	Model             string
	SystemPrompt      string
	MaxTokens         int
	PermissionPrompt  PermissionPrompt
	AskUserPrompt     AskUserPrompt
	MaxTurns          int
	HookExecutor      HookExecutor
	ToolMetadata      map[string]any
}

// RunQuery executes the tool-aware conversation loop.
func RunQuery(
	ctx context.Context,
	qc QueryContext,
	messages *[]ConversationMessage,
	emit func(StreamEvent, *UsageSnapshot) error,
) error {
	maxTurns := qc.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 16
	}

	for turn := 0; turn < maxTurns; turn++ {
		var finalMessage *ConversationMessage
		usage := UsageSnapshot{}

		stream, err := qc.APIClient.StreamMessage(ApiMessageRequest{
			Model:        qc.Model,
			Messages:     append([]ConversationMessage{}, *messages...),
			SystemPrompt: qc.SystemPrompt,
			MaxTokens:    qc.MaxTokens,
			Tools:        qc.ToolRegistry.ToAPISchema(),
		})
		if err != nil {
			return err
		}

		streamClosed := false
		for !streamClosed {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ev, ok := <-stream:
				if !ok {
					if finalMessage == nil {
						return errors.New("model stream finished without a final message")
					}
					streamClosed = true
					continue
				}
				if ev.TextDelta != "" {
					if err := emit(AssistantTextDelta{Text: ev.TextDelta}, nil); err != nil {
						return err
					}
				}
				if ev.Complete != nil {
					m := ev.Complete.Message
					finalMessage = &m
					usage = ev.Complete.Usage
				}
			}
		}

		*messages = append(*messages, *finalMessage)
		if err := emit(AssistantTurnComplete{Message: *finalMessage, Usage: usage}, &usage); err != nil {
			return err
		}

		if len(finalMessage.ToolUses) == 0 {
			return nil
		}

		toolCalls := finalMessage.ToolUses
		toolResults := make([]ToolResultBlock, len(toolCalls))

		if len(toolCalls) == 1 {
			tc := toolCalls[0]
			if err := emit(ToolExecutionStarted{ToolName: tc.Name, ToolInput: tc.Input}, nil); err != nil {
				return err
			}
			result := executeToolCall(ctx, qc, tc.Name, tc.ID, tc.Input)
			toolResults[0] = result
			if err := emit(ToolExecutionCompleted{ToolName: tc.Name, Output: result.Content, IsError: result.IsError}, nil); err != nil {
				return err
			}
		} else {
			for _, tc := range toolCalls {
				if err := emit(ToolExecutionStarted{ToolName: tc.Name, ToolInput: tc.Input}, nil); err != nil {
					return err
				}
			}
			var wg sync.WaitGroup
			for i, tc := range toolCalls {
				wg.Add(1)
				go func(idx int, call ToolUse) {
					defer wg.Done()
					toolResults[idx] = executeToolCall(ctx, qc, call.Name, call.ID, call.Input)
				}(i, tc)
			}
			wg.Wait()
			for i, tc := range toolCalls {
				result := toolResults[i]
				if err := emit(ToolExecutionCompleted{ToolName: tc.Name, Output: result.Content, IsError: result.IsError}, nil); err != nil {
					return err
				}
			}
		}

		*messages = append(*messages, ConversationMessage{Role: "user", ToolResults: toolResults})
	}

	return fmt.Errorf("exceeded maximum turn limit (%d)", maxTurns)
}

func executeToolCall(
	ctx context.Context,
	qc QueryContext,
	toolName, toolUseID string,
	toolInput map[string]any,
) ToolResultBlock {
	if qc.HookExecutor != nil {
		if blocked, reason := qc.HookExecutor.PreToolUse(toolName, toolInput); blocked {
			if reason == "" {
				reason = fmt.Sprintf("pre_tool_use hook blocked %s", toolName)
			}
			return ToolResultBlock{ToolUseID: toolUseID, Content: reason, IsError: true}
		}
	}

	tool, ok := qc.ToolRegistry.Get(toolName)
	if !ok {
		return ToolResultBlock{ToolUseID: toolUseID, Content: "Unknown tool: " + toolName, IsError: true}
	}

	filePath, _ := toolInput["path"].(string)
	if filePath == "" {
		filePath, _ = toolInput["file_path"].(string)
	}
	command, _ := toolInput["command"].(string)

	if qc.PermissionChecker != nil {
		decision := qc.PermissionChecker.Evaluate(toolName, tool.IsReadOnly(), filePath, command)
		if !decision.Allowed {
			if decision.RequiresConfirmation && qc.PermissionPrompt != nil {
				if !qc.PermissionPrompt(toolName, decision.Reason) {
					return ToolResultBlock{ToolUseID: toolUseID, Content: "Permission denied for " + toolName, IsError: true}
				}
			} else {
				reason := decision.Reason
				if reason == "" {
					reason = "Permission denied for " + toolName
				}
				return ToolResultBlock{ToolUseID: toolUseID, Content: reason, IsError: true}
			}
		}
	}

	raw, err := json.Marshal(toolInput)
	if err != nil {
		return ToolResultBlock{ToolUseID: toolUseID, Content: "Invalid input for " + toolName + ": " + err.Error(), IsError: true}
	}

	metadata := map[string]any{
		"tool_registry":   qc.ToolRegistry,
		"ask_user_prompt": qc.AskUserPrompt,
	}
	for k, v := range qc.ToolMetadata {
		metadata[k] = v
	}

	result, execErr := tool.Execute(ctx, raw, tools.ToolExecutionContext{
		CWD:      filepath.Clean(qc.CWD),
		Metadata: metadata,
	})
	if execErr != nil {
		result = tools.NewErrorResult(execErr)
	}

	if qc.HookExecutor != nil {
		qc.HookExecutor.PostToolUse(toolName, toolInput, result)
	}

	return ToolResultBlock{ToolUseID: toolUseID, Content: result.Output, IsError: result.IsError}
}
