package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// GrepTool searches text files with a regular expression.
type GrepTool struct{}

func (t *GrepTool) Name() string        { return "grep" }
func (t *GrepTool) Description() string { return "Search file contents with a regular expression." }
func (t *GrepTool) IsReadOnly() bool    { return true }

func (t *GrepTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern":        map[string]interface{}{"type": "string", "description": "Regular expression to search for"},
			"root":           map[string]interface{}{"type": "string", "description": "Search root directory"},
			"file_glob":      map[string]interface{}{"type": "string", "description": "File glob pattern"},
			"case_sensitive": map[string]interface{}{"type": "boolean", "description": "Use case-sensitive matching"},
			"limit":          map[string]interface{}{"type": "integer", "description": "Maximum number of matches"},
		},
		"required": []string{"pattern"},
	}
}

type grepInput struct {
	Pattern       string `json:"pattern"`
	Root          string `json:"root,omitempty"`
	FileGlob      string `json:"file_glob,omitempty"`
	CaseSensitive *bool  `json:"case_sensitive,omitempty"`
	Limit         int    `json:"limit,omitempty"`
}

func (t *GrepTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input grepInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}
	if input.Pattern == "" {
		return NewErrorResultf("pattern is required"), nil
	}
	if input.FileGlob == "" {
		input.FileGlob = "**/*"
	}
	if input.Limit <= 0 {
		input.Limit = 200
	}
	caseSensitive := true
	if input.CaseSensitive != nil {
		caseSensitive = *input.CaseSensitive
	}

	root := execCtx.CWD
	if input.Root != "" {
		root = resolvePath(execCtx.CWD, input.Root)
	}

	pattern := input.Pattern
	if !caseSensitive {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return NewErrorResult(err), nil
	}

	var matches []string
	var files []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if !matchesFileGlob(filepath.ToSlash(rel), input.FileGlob) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)

	for _, path := range files {
		if len(matches) >= input.Limit {
			break
		}
		select {
		case <-ctx.Done():
			return ToolResult{}, ctx.Err()
		default:
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if isBinary(raw) {
			continue
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			continue
		}
		s := bufio.NewScanner(bytes.NewReader(raw))
		lineNo := 0
		for s.Scan() {
			lineNo++
			line := s.Text()
			if re.MatchString(line) {
				matches = append(matches, filepath.ToSlash(rel)+":"+itoa(lineNo)+":"+line)
				if len(matches) >= input.Limit {
					break
				}
			}
		}
	}

	if len(matches) == 0 {
		return NewSuccessResult("(no matches)"), nil
	}
	return NewSuccessResult(joinLinesWithNewline(matches)), nil
}

func isBinary(raw []byte) bool {
	for _, b := range raw {
		if b == 0 {
			return true
		}
	}
	return false
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	buf := [20]byte{}
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + (i % 10))
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func matchesFileGlob(relPath, pattern string) bool {
	if pattern == "" || pattern == "**/*" || pattern == "**" {
		return true
	}
	ok, err := filepath.Match(pattern, relPath)
	if err == nil && ok {
		return true
	}
	if strings.HasPrefix(pattern, "**/") {
		suffix := strings.TrimPrefix(pattern, "**/")
		ok, err = filepath.Match(suffix, relPath)
		if err == nil && ok {
			return true
		}
		_, file := filepath.Split(relPath)
		ok, err = filepath.Match(suffix, file)
		return err == nil && ok
	}
	return false
}
