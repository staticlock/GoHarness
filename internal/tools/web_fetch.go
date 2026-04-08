package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// WebFetchTool fetches one remote page and returns compact text output.
type WebFetchTool struct {
	Client *http.Client
}

func (t *WebFetchTool) Name() string { return "web_fetch" }
func (t *WebFetchTool) Description() string {
	return "Fetch one web page and return compact readable text."
}
func (t *WebFetchTool) IsReadOnly() bool { return true }

func (t *WebFetchTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "HTTP or HTTPS URL to fetch",
			},
			"max_chars": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum response characters in output",
			},
		},
		"required": []string{"url"},
	}
}

type webFetchInput struct {
	URL      string `json:"url"`
	MaxChars int    `json:"max_chars,omitempty"`
}

func (t *WebFetchTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = execCtx
	var input webFetchInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}
	if input.URL == "" {
		return NewErrorResultf("url is required"), nil
	}
	if input.MaxChars <= 0 {
		input.MaxChars = 12000
	}

	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, input.URL, nil)
	if err != nil {
		return NewErrorResult(err), nil
	}
	req.Header.Set("User-Agent", "OpenHarness/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return ToolResult{Output: fmt.Sprintf("web_fetch failed: %v", err), IsError: true}, nil
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ToolResult{Output: fmt.Sprintf("web_fetch failed: %v", err), IsError: true}, nil
	}
	body := string(bodyBytes)
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(strings.ToLower(contentType), "html") {
		body = htmlToText(body)
	}
	body = strings.TrimSpace(body)
	if len(body) > input.MaxChars {
		body = strings.TrimSpace(body[:input.MaxChars]) + "\n...[truncated]"
	}

	return NewSuccessResult(
		fmt.Sprintf("URL: %s\nStatus: %d\nContent-Type: %s\n\n%s", resp.Request.URL.String(), resp.StatusCode, defaultContentType(contentType), body),
	), nil
}

func defaultContentType(ct string) string {
	if strings.TrimSpace(ct) == "" {
		return "(unknown)"
	}
	return ct
}

func htmlToText(html string) string {
	reScript := regexp.MustCompile(`(?is)<script.*?>.*?</script>`)
	reStyle := regexp.MustCompile(`(?is)<style.*?>.*?</style>`)
	reTags := regexp.MustCompile(`(?s)<[^>]+>`)
	reSpaces := regexp.MustCompile(`[ \t\r\f\v]+`)
	text := reScript.ReplaceAllString(html, " ")
	text = reStyle.ReplaceAllString(text, " ")
	text = reTags.ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = reSpaces.ReplaceAllString(text, " ")
	return strings.TrimSpace(strings.ReplaceAll(text, " \n", "\n"))
}
