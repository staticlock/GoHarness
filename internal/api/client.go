package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	aoption "github.com/anthropics/anthropic-sdk-go/option"
	openai "github.com/openai/openai-go"
	ooption "github.com/openai/openai-go/option"
	"github.com/user/goharness/internal/engine"
)

const maxRetries = 3

// Client is a provider-agnostic API client using official SDKs.
type Client struct {
	apiKey       string
	baseURL      string
	httpClient   *http.Client
	anthropicSDK anthropic.Client
	openaiSDK    openai.Client
}

// NewClient creates a new API client.
func NewClient(apiKey, baseURL string) *Client {
	normalizedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	hc := &http.Client{Timeout: 60 * time.Second}

	aopts := []aoption.RequestOption{
		aoption.WithAPIKey(apiKey),
		aoption.WithRequestTimeout(60 * time.Second),
		aoption.WithMaxRetries(maxRetries),
		aoption.WithHTTPClient(hc),
	}
	if normalizedBaseURL != "" {
		aopts = append(aopts, aoption.WithBaseURL(normalizedBaseURL))
	}

	oopts := []ooption.RequestOption{
		ooption.WithRequestTimeout(60 * time.Second),
		ooption.WithMaxRetries(maxRetries),
		ooption.WithHTTPClient(hc),
		ooption.WithHeader("Authorization", "Bearer "+apiKey),
	}
	if normalizedBaseURL != "" {
		oopts = append(oopts, ooption.WithBaseURL(normalizedBaseURL))
	}

	return &Client{
		apiKey:       apiKey,
		baseURL:      normalizedBaseURL,
		httpClient:   hc,
		anthropicSDK: anthropic.NewClient(aopts...),
		openaiSDK:    openai.NewClient(oopts...),
	}
}

// StreamMessage implements engine.SupportsStreamingMessages.
func (c *Client) StreamMessage(req engine.ApiMessageRequest) (<-chan engine.ApiStreamEvent, error) {
	resp, err := c.callWithRetry(req)
	if err != nil {
		return nil, err
	}

	out := make(chan engine.ApiStreamEvent, 2)
	go func() {
		defer close(out)
		if strings.TrimSpace(resp.Message.Text) != "" {
			out <- engine.ApiStreamEvent{TextDelta: resp.Message.Text}
		}
		out <- engine.ApiStreamEvent{Complete: &engine.ApiMessageCompleteEvent{Message: resp.Message, Usage: resp.Usage}}
	}()
	return out, nil
}

type callResponse struct {
	Message engine.ConversationMessage
	Usage   engine.UsageSnapshot
}

func (c *Client) callWithRetry(req engine.ApiMessageRequest) (callResponse, error) {
	result, err := c.callOnce(req)
	if err != nil {
		msg := strings.ToLower(err.Error())
		switch {
		case strings.Contains(msg, "401"), strings.Contains(msg, "403"), strings.Contains(msg, "authentication"):
			return callResponse{}, fmt.Errorf("%w: %v", ErrAuthentication, err)
		case strings.Contains(msg, "429"), strings.Contains(msg, "rate limit"):
			return callResponse{}, fmt.Errorf("%w: %v", ErrRateLimit, err)
		default:
			return callResponse{}, fmt.Errorf("%w: %v", ErrRequest, err)
		}
	}
	return result, nil
}

func (c *Client) callOnce(req engine.ApiMessageRequest) (callResponse, error) {
	if c.useOpenAI(req) {
		return c.callOpenAI(req)
	}
	return c.callAnthropic(req)
}

func (c *Client) callAnthropic(req engine.ApiMessageRequest) (callResponse, error) {
	body := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		MaxTokens: int64(req.MaxTokens),
		Messages:  toSDKMessages(req.Messages),
		Tools:     toSDKTools(req.Tools),
	}
	if strings.TrimSpace(req.SystemPrompt) != "" {
		body.System = []anthropic.TextBlockParam{{Text: req.SystemPrompt}}
	}
	resp, err := c.anthropicSDK.Messages.New(context.Background(), body)
	if err != nil {
		return callResponse{}, err
	}

	msg := engine.ConversationMessage{Role: "assistant"}
	textParts := make([]string, 0)
	toolUses := make([]engine.ToolUse, 0)
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if strings.TrimSpace(block.Text) != "" {
				textParts = append(textParts, block.Text)
			}
		case "tool_use":
			input := map[string]any{}
			if len(block.Input) > 0 {
				_ = json.Unmarshal(block.Input, &input)
			}
			toolUses = append(toolUses, engine.ToolUse{ID: block.ID, Name: block.Name, Input: input})
		}
	}
	msg.Text = strings.Join(textParts, "")
	msg.ToolUses = toolUses
	usage := engine.UsageSnapshot{InputTokens: int(resp.Usage.InputTokens), OutputTokens: int(resp.Usage.OutputTokens), TotalTokens: int(resp.Usage.InputTokens + resp.Usage.OutputTokens)}
	return callResponse{Message: msg, Usage: usage}, nil
}

func (c *Client) callOpenAI(req engine.ApiMessageRequest) (callResponse, error) {
	payload := map[string]any{
		"model":    req.Model,
		"messages": toOpenAIMessageParams(req.Messages, req.SystemPrompt),
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	if len(req.Tools) > 0 {
		payload["tools"] = toOpenAITools(req.Tools)
	}

	var resp struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := c.openaiSDK.Post(context.Background(), "/chat/completions", payload, &resp); err != nil {
		return callResponse{}, err
	}

	message := engine.ConversationMessage{Role: "assistant"}
	if len(resp.Choices) > 0 {
		message.Text = resp.Choices[0].Message.Content
		for _, tc := range resp.Choices[0].Message.ToolCalls {
			input := map[string]any{}
			if strings.TrimSpace(tc.Function.Arguments) != "" {
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
			}
			message.ToolUses = append(message.ToolUses, engine.ToolUse{ID: tc.ID, Name: tc.Function.Name, Input: input})
		}
	}
	usage := engine.UsageSnapshot{InputTokens: resp.Usage.PromptTokens, OutputTokens: resp.Usage.CompletionTokens, TotalTokens: resp.Usage.TotalTokens}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	return callResponse{Message: message, Usage: usage}, nil
}

func (c *Client) useOpenAI(req engine.ApiMessageRequest) bool {
	m := strings.ToLower(strings.TrimSpace(req.Model))
	b := strings.ToLower(strings.TrimSpace(c.baseURL))
	if strings.Contains(b, "openai") || strings.Contains(b, "openrouter") {
		return true
	}
	return strings.HasPrefix(m, "gpt") || strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3") || strings.HasPrefix(m, "o4")
}

func toSDKMessages(messages []engine.ConversationMessage) []anthropic.MessageParam {
	out := make([]anthropic.MessageParam, 0, len(messages))
	for _, msg := range messages {
		blocks := make([]anthropic.ContentBlockParamUnion, 0)
		if strings.TrimSpace(msg.Text) != "" {
			blocks = append(blocks, anthropic.NewTextBlock(msg.Text))
		}
		for _, use := range msg.ToolUses {
			blocks = append(blocks, anthropic.NewToolUseBlock(use.ID, use.Input, use.Name))
		}
		for _, result := range msg.ToolResults {
			blocks = append(blocks, anthropic.NewToolResultBlock(result.ToolUseID, result.Content, result.IsError))
		}
		if len(blocks) == 0 {
			blocks = append(blocks, anthropic.NewTextBlock(""))
		}
		if strings.EqualFold(msg.Role, "assistant") {
			out = append(out, anthropic.NewAssistantMessage(blocks...))
		} else {
			out = append(out, anthropic.NewUserMessage(blocks...))
		}
	}
	return out
}

func toSDKTools(tools []map[string]any) []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}
		description, _ := tool["description"].(string)
		schema, _ := tool["input_schema"].(map[string]any)
		props := map[string]any{}
		if p, ok := schema["properties"].(map[string]any); ok {
			props = p
		}
		required := make([]string, 0)
		switch req := schema["required"].(type) {
		case []string:
			required = append(required, req...)
		case []any:
			for _, item := range req {
				if s, ok := item.(string); ok {
					required = append(required, s)
				}
			}
		}
		toolParam := anthropic.ToolParam{
			Name: name,
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: props,
				Required:   required,
			},
		}
		if strings.TrimSpace(description) != "" {
			toolParam.Description = anthropic.String(description)
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &toolParam})
	}
	return out
}

func toOpenAIMessageParams(messages []engine.ConversationMessage, systemPrompt string) []map[string]any {
	out := make([]map[string]any, 0, len(messages)+1)
	if strings.TrimSpace(systemPrompt) != "" {
		out = append(out, map[string]any{"role": "system", "content": systemPrompt})
	}
	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		if role != "assistant" {
			role = "user"
		}
		entry := map[string]any{"role": role}
		if strings.TrimSpace(msg.Text) != "" {
			entry["content"] = msg.Text
		} else {
			entry["content"] = ""
		}
		if len(msg.ToolResults) > 0 {
			entry["role"] = "tool"
			entry["content"] = msg.ToolResults[0].Content
		}
		out = append(out, entry)
	}
	return out
}

func toOpenAITools(tools []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}
		description, _ := tool["description"].(string)
		schema, _ := tool["input_schema"].(map[string]any)
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        name,
				"description": description,
				"parameters":  schema,
			},
		})
	}
	return out
}
