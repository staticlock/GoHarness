package ui

import (
	"fmt"
	"strings"
)

type OutputRenderer struct {
	styleName       string
	assistantBuffer string
	spinnerActive   bool
}

func NewOutputRenderer(styleName string) *OutputRenderer {
	if styleName == "" {
		styleName = "default"
	}
	return &OutputRenderer{
		styleName:     styleName,
		spinnerActive: false,
	}
}

func (r *OutputRenderer) SetStyle(styleName string) {
	r.styleName = styleName
}

func (r *OutputRenderer) PrintSystem(message string) {
	if r.styleName == "minimal" {
		fmt.Println(message)
	} else {
		fmt.Printf("[yellow]ℹ %s[/yellow]\n", message)
	}
}

func (r *OutputRenderer) PrintStatusLine(model string, inputTokens, outputTokens int, permissionMode string) {
	parts := []string{fmt.Sprintf("model: %s", model)}
	if inputTokens > 0 || outputTokens > 0 {
		parts = append(parts, fmt.Sprintf("tokens: %d↓ %d↑", inputTokens, outputTokens))
	}
	parts = append(parts, fmt.Sprintf("mode: %s", permissionMode))
	sep := " | "
	fmt.Printf("[dim]%s%s[dim]\n", strings.Join(parts, sep), sep)
}

func (r *OutputRenderer) Clear() {
	fmt.Print("\033[2J\033[H")
}

func (r *OutputRenderer) HasMarkdown(text string) bool {
	indicators := []string{"```", "## ", "### ", "- ", "* ", "1. ", "**", "__", "> "}
	for _, ind := range indicators {
		if strings.Contains(text, ind) {
			return true
		}
	}
	return false
}

func (r *OutputRenderer) SummarizeToolInput(toolName string, toolInput map[string]any) string {
	if toolInput == nil {
		return ""
	}
	lower := strings.ToLower(toolName)

	switch lower {
	case "bash":
		if cmd, ok := toolInput["command"].(string); ok {
			if len(cmd) > 120 {
				return cmd[:120]
			}
			return cmd
		}
	case "read", "fileread", "file_read":
		if fp, ok := toolInput["file_path"].(string); ok {
			return fp
		}
	case "write", "filewrite", "file_write":
		if fp, ok := toolInput["file_path"].(string); ok {
			return fp
		}
	case "edit", "fileedit", "file_edit":
		if fp, ok := toolInput["file_path"].(string); ok {
			return fp
		}
	case "grep", "greptool":
		if pat, ok := toolInput["pattern"].(string); ok {
			return "/" + pat + "/"
		}
	case "glob", "globtool":
		if pat, ok := toolInput["pattern"].(string); ok {
			return pat
		}
	}

	for k, v := range toolInput {
		return fmt.Sprintf("%s=%v", k, v)
	}
	return ""
}

func (r *OutputRenderer) ExtToLexer(ext string) string {
	mapping := map[string]string{
		"py":   "python",
		"js":   "javascript",
		"ts":   "typescript",
		"tsx":  "tsx",
		"jsx":  "jsx",
		"rs":   "rust",
		"go":   "go",
		"rb":   "ruby",
		"java": "java",
		"c":    "c",
		"cpp":  "cpp",
		"h":    "c",
		"hpp":  "cpp",
		"cs":   "csharp",
		"sh":   "bash",
		"bash": "bash",
		"zsh":  "bash",
		"json": "json",
		"yaml": "yaml",
		"yml":  "yaml",
		"toml": "toml",
		"xml":  "xml",
		"html": "html",
		"css":  "css",
		"sql":  "sql",
		"md":   "markdown",
	}
	return mapping[strings.ToLower(ext)]
}
