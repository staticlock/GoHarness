package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type WebSearchTool struct{}

func (t *WebSearchTool) Name() string { return "web_search" }
func (t *WebSearchTool) Description() string {
	return "Search the web and return compact top results with titles, URLs, and snippets."
}
func (t *WebSearchTool) IsReadOnly() bool { return true }

func (t *WebSearchTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of results (1-10)",
				"default":     5,
			},
			"search_url": map[string]interface{}{
				"type":        "string",
				"description": "Optional override for the search endpoint",
			},
		},
		"required": []string{"query"},
	}
}

type webSearchInput struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
	SearchURL  string `json:"search_url,omitempty"`
}

func (t *WebSearchTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input webSearchInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	if input.MaxResults <= 0 {
		input.MaxResults = 5
	}
	if input.MaxResults > 10 {
		input.MaxResults = 10
	}

	endpoint := "https://html.duckduckgo.com/html/"
	if input.SearchURL != "" {
		endpoint = input.SearchURL
	}

	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return NewErrorResult(err), nil
	}
	req.Header.Set("User-Agent", "GoHarness/0.1")
	q := req.URL.Query()
	q.Add("q", input.Query)
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return NewErrorResultf("web_search failed: %v", err), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return NewErrorResultf("web_search failed: status %d", resp.StatusCode), nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NewErrorResult(err), nil
	}

	results := parseSearchResults(string(body), input.MaxResults)
	if len(results) == 0 {
		return NewSuccessResult("No search results found."), nil
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Search results for: %s", input.Query))
	for i, r := range results {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, r.Title))
		lines = append(lines, fmt.Sprintf("   URL: %s", r.URL))
		if r.Snippet != "" {
			lines = append(lines, fmt.Sprintf("   %s", r.Snippet))
		}
	}

	return NewSuccessResult(strings.Join(lines, "\n")), nil
}

type searchResult struct {
	Title   string
	URL     string
	Snippet string
}

func parseSearchResults(body string, limit int) []searchResult {
	snippetPattern := regexp.MustCompile(`<(?:a|div|span)[^>]+class="[^"]*(?:result__snippet|result-snippet)[^"]*"[^>]*>(?P<snippet>.*?)</(?:a|div|span)>`)
	snippets := make([]string, 0)
	for _, match := range snippetPattern.FindAllStringSubmatch(body, -1) {
		snippet := extractGroup(match, "snippet")
		snippets = append(snippets, cleanHTML(snippet))
	}

	anchorPattern := regexp.MustCompile(`<a(?P<attrs>[^>]+)>(?P<title>.*?)</a>`)
	results := make([]searchResult, 0)
	snippetIdx := 0

	for _, match := range anchorPattern.FindAllStringSubmatch(body, -1) {
		attrs := extractGroup(match, "attrs")
		title := extractGroup(match, "title")

		classMatch := regexp.MustCompile(`class="(?P<class>[^"]+)"`).FindStringSubmatch(attrs)
		if classMatch == nil {
			continue
		}
		classNames := extractGroup(classMatch, "class")
		if !strings.Contains(classNames, "result__a") && !strings.Contains(classNames, "result-link") {
			continue
		}

		hrefMatch := regexp.MustCompile(`href="(?P<href>[^"]+)"`).FindStringSubmatch(attrs)
		if hrefMatch == nil {
			continue
		}

		resultURL := normalizeURL(extractGroup(hrefMatch, "href"))
		if resultURL == "" {
			continue
		}

		snippet := ""
		if snippetIdx < len(snippets) {
			snippet = snippets[snippetIdx]
		}
		snippetIdx++

		results = append(results, searchResult{
			Title:   cleanHTML(title),
			URL:     resultURL,
			Snippet: snippet,
		})

		if len(results) >= limit {
			break
		}
	}

	return results
}

func extractGroup(match []string, name string) string {
	for i := range match {
		if strings.HasPrefix(match[i], name+":") || i == 0 {
			continue
		}
		return match[i]
	}
	return ""
}

func normalizeURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	if strings.HasSuffix(parsed.Hostname(), "duckduckgo.com") && strings.HasPrefix(parsed.Path, "/l/") {
		params, _ := url.ParseQuery(parsed.RawQuery)
		if target := params.Get("uddg"); target != "" {
			return target
		}
	}

	if strings.HasPrefix(raw, "/") {
		return "https://duckduckgo.com" + raw
	}

	return raw
}

func cleanHTML(fragment string) string {
	re := regexp.MustCompile(`(?s)<[^>]+>`)
	text := re.ReplaceAllString(fragment, " ")
	text = html.UnescapeString(text)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}
