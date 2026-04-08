package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ReadTool reads file contents.
type ReadTool struct{}

func (t *ReadTool) Name() string        { return "read_file" }
func (t *ReadTool) Description() string { return "Read a text file from the local repository." }
func (t *ReadTool) IsReadOnly() bool    { return true }

func (t *ReadTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path of the file to read",
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Zero-based starting line",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Number of lines to return",
			},
		},
		"required": []string{"path"},
	}
}

type readInput struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

func (t *ReadTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input readInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	if input.Limit <= 0 {
		input.Limit = 200
	}

	path := resolvePath(execCtx.CWD, input.Path)
	data, err := os.ReadFile(path)
	if err != nil {
		return NewErrorResult(err), nil
	}

	content := string(data)

	lines := splitLines(content)
	start := input.Offset
	if start < 0 {
		start = 0
	}
	if start > len(lines) {
		start = len(lines)
	}
	end := start + input.Limit
	if end > len(lines) {
		end = len(lines)
	}
	selected := lines[start:end]
	if len(selected) == 0 {
		return NewSuccessResult(fmt.Sprintf("(no content in selected range for %s)", path)), nil
	}

	numbered := make([]string, 0, len(selected))
	for i, line := range selected {
		numbered = append(numbered, fmt.Sprintf("%6d\t%s", start+i+1, trimNewline(line)))
	}

	return NewSuccessResult(joinLinesWithNewline(numbered)), nil
}

// WriteTool writes content to a file.
type WriteTool struct{}

func (t *WriteTool) Name() string { return "write_file" }
func (t *WriteTool) Description() string {
	return "Create or overwrite a text file in the local repository."
}
func (t *WriteTool) IsReadOnly() bool { return false }

func (t *WriteTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path of the file to write",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Full file contents",
			},
			"create_directories": map[string]interface{}{
				"type":        "boolean",
				"description": "Create parent directories when needed",
			},
		},
		"required": []string{"path", "content"},
	}
}

type writeInput struct {
	Path              string `json:"path"`
	Content           string `json:"content"`
	CreateDirectories *bool  `json:"create_directories,omitempty"`
}

func (t *WriteTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input writeInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	path := resolvePath(execCtx.CWD, input.Path)

	createDirs := true
	if input.CreateDirectories != nil {
		createDirs = *input.CreateDirectories
	}
	if createDirs {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return NewErrorResult(err), nil
		}
	}

	if err := os.WriteFile(path, []byte(input.Content), 0644); err != nil {
		return NewErrorResult(err), nil
	}

	return NewSuccessResult(fmt.Sprintf("Wrote %s", path)), nil
}

// EditTool edits a file with string replacement.
type EditTool struct{}

func (t *EditTool) Name() string        { return "edit_file" }
func (t *EditTool) Description() string { return "Edit an existing file by replacing a string." }
func (t *EditTool) IsReadOnly() bool    { return false }

func (t *EditTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path of the file to edit",
			},
			"old_str": map[string]interface{}{
				"type":        "string",
				"description": "The text to replace",
			},
			"new_str": map[string]interface{}{
				"type":        "string",
				"description": "The replacement text",
			},
			"replace_all": map[string]interface{}{
				"type":        "boolean",
				"description": "Replace all occurrences instead of the first",
			},
		},
		"required": []string{"path", "old_str", "new_str"},
	}
}

type editInput struct {
	Path       string `json:"path"`
	OldString  string `json:"old_str"`
	NewString  string `json:"new_str"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

func (t *EditTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input editInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	path := resolvePath(execCtx.CWD, input.Path)
	content, err := os.ReadFile(path)
	if err != nil {
		return NewErrorResult(err), nil
	}

	original := string(content)
	if !contains(original, input.OldString) {
		return ToolResult{Output: "old_str was not found in the file", IsError: true}, nil
	}

	var newContent string
	if input.ReplaceAll {
		newContent = replaceAll(original, input.OldString, input.NewString)
	} else {
		newContent = replaceFirst(original, input.OldString, input.NewString)
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return NewErrorResult(err), nil
	}

	return NewSuccessResult(fmt.Sprintf("Updated %s", path)), nil
}

// GlobTool finds files matching a pattern.
type GlobTool struct{}

func (t *GlobTool) Name() string        { return "glob" }
func (t *GlobTool) Description() string { return "List files matching a glob pattern." }
func (t *GlobTool) IsReadOnly() bool    { return true }

func (t *GlobTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Glob pattern relative to the working directory",
			},
			"root": map[string]interface{}{
				"type":        "string",
				"description": "Optional search root",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of results",
			},
		},
		"required": []string{"pattern"},
	}
}

type globInput struct {
	Pattern string `json:"pattern"`
	Root    string `json:"root,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

func (t *GlobTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input globInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	root := execCtx.CWD
	if input.Root != "" {
		root = resolvePath(execCtx.CWD, input.Root)
	}
	if input.Limit <= 0 {
		input.Limit = 200
	}
	pattern := filepath.Join(root, input.Pattern)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return NewErrorResult(err), nil
	}
	// Make paths relative to CWD
	if len(matches) > input.Limit {
		matches = matches[:input.Limit]
	}
	for i, match := range matches {
		rel, _ := filepath.Rel(root, match)
		matches[i] = rel
	}
	if len(matches) == 0 {
		return NewSuccessResult("(no matches)"), nil
	}
	return NewSuccessResult(joinLinesWithNewline(matches)), nil
}

// Helper functions
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func joinLines(lines []string) string {
	result := ""
	for _, line := range lines {
		result += line
	}
	return result
}

func joinLinesWithNewline(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for i := 1; i < len(lines); i++ {
		out += "\n" + lines[i]
	}
	return out
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func resolvePath(base, candidate string) string {
	path := candidate
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return resolved
}

func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func replaceAll(s, old, new string) string {
	if old == "" {
		return s
	}
	out := s
	for {
		next := replaceFirst(out, old, new)
		if next == out {
			return out
		}
		out = next
	}
}

func replaceFirst(s, old, new string) string {
	idx := 0
	for i := 0; i <= len(s)-len(old); i++ {
		if s[i:i+len(old)] == old {
			idx = i
			break
		}
	}
	if idx == 0 && (len(s) < len(old) || s[:len(old)] != old) {
		return s
	}
	return s[:idx] + new + s[idx+len(old):]
}
