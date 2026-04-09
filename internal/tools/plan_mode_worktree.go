package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/staticlock/GoHarness/internal/config"
)

// EnterPlanModeTool switches permission mode to plan.
type EnterPlanModeTool struct{}

func (t *EnterPlanModeTool) Name() string        { return "enter_plan_mode" }
func (t *EnterPlanModeTool) Description() string { return "Switch permission mode to plan." }
func (t *EnterPlanModeTool) IsReadOnly() bool    { return false }

func (t *EnterPlanModeTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *EnterPlanModeTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = execCtx
	settings, err := config.LoadSettings()
	if err != nil {
		return NewErrorResult(err), nil
	}

	settings.Permission.Mode = "plan"
	if err := config.SaveSettings(settings); err != nil {
		return NewErrorResult(err), nil
	}

	return NewSuccessResult("Permission mode set to plan"), nil
}

// ExitPlanModeTool switches permission mode back to default.
type ExitPlanModeTool struct{}

func (t *ExitPlanModeTool) Name() string        { return "exit_plan_mode" }
func (t *ExitPlanModeTool) Description() string { return "Switch permission mode back to default." }
func (t *ExitPlanModeTool) IsReadOnly() bool    { return false }

func (t *ExitPlanModeTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *ExitPlanModeTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = execCtx
	settings, err := config.LoadSettings()
	if err != nil {
		return NewErrorResult(err), nil
	}

	settings.Permission.Mode = "default"
	if err := config.SaveSettings(settings); err != nil {
		return NewErrorResult(err), nil
	}

	return NewSuccessResult("Permission mode set to default"), nil
}

// EnterWorktreeTool creates a git worktree.
type EnterWorktreeTool struct{}

func (t *EnterWorktreeTool) Name() string        { return "enter_worktree" }
func (t *EnterWorktreeTool) Description() string { return "Create a git worktree and return its path." }
func (t *EnterWorktreeTool) IsReadOnly() bool    { return false }

func (t *EnterWorktreeTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"branch": map[string]interface{}{
				"type":        "string",
				"description": "Target branch name for the worktree",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Optional worktree path",
			},
			"create_branch": map[string]interface{}{
				"type":        "boolean",
				"description": "Create a new branch",
			},
			"base_ref": map[string]interface{}{
				"type":        "string",
				"description": "Base ref when creating a new branch",
			},
		},
		"required": []string{"branch"},
	}
}

type enterWorktreeInput struct {
	Branch       string `json:"branch"`
	Path         string `json:"path,omitempty"`
	CreateBranch bool   `json:"create_branch,omitempty"`
	BaseRef      string `json:"base_ref,omitempty"`
}

func (t *EnterWorktreeTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input enterWorktreeInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	topLevel, err := gitOutput(execCtx.CWD, "rev-parse", "--show-toplevel")
	if err != nil {
		return ToolResult{Output: "enter_worktree requires a git repository", IsError: true}, nil
	}

	worktreePath := resolveWorktreePath(topLevel, input.Branch, input.Path)

	if err := mkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		return NewErrorResult(err), nil
	}

	var cmd *exec.Cmd
	if input.CreateBranch {
		baseRef := input.BaseRef
		if baseRef == "" {
			baseRef = "HEAD"
		}
		cmd = exec.Command("git", "worktree", "add", "-b", input.Branch, worktreePath, baseRef)
	} else {
		cmd = exec.Command("git", "worktree", "add", worktreePath, input.Branch)
	}
	cmd.Dir = topLevel

	output, err := cmd.CombinedOutput()
	if err != nil {
		return ToolResult{Output: string(output), IsError: true}, nil
	}

	result := string(output)
	if result == "" {
		result = fmt.Sprintf("Created worktree %s", worktreePath)
	}

	return NewSuccessResult(fmt.Sprintf("%s\nPath: %s", result, worktreePath)), nil
}

// ExitWorktreeTool removes a git worktree.
type ExitWorktreeTool struct{}

func (t *ExitWorktreeTool) Name() string        { return "exit_worktree" }
func (t *ExitWorktreeTool) Description() string { return "Remove a git worktree by path." }
func (t *ExitWorktreeTool) IsReadOnly() bool    { return false }

func (t *ExitWorktreeTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Worktree path to remove",
			},
		},
		"required": []string{"path"},
	}
}

type exitWorktreeInput struct {
	Path string `json:"path"`
}

func (t *ExitWorktreeTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input exitWorktreeInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	path := input.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(execCtx.CWD, path)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		path = input.Path
	}

	cmd := exec.Command("git", "worktree", "remove", "--force", path)
	cmd.Dir = execCtx.CWD

	output, err := cmd.CombinedOutput()
	result := string(output)
	if result == "" {
		result = fmt.Sprintf("Removed worktree %s", path)
	}

	if err != nil {
		return ToolResult{Output: result, IsError: true}, nil
	}

	return NewSuccessResult(result), nil
}

func gitOutput(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	result := string(output)
	result = trimNewline(result)
	return result, nil
}

func resolveWorktreePath(repoRoot, branch, path string) string {
	if path != "" {
		resolved := path
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(repoRoot, resolved)
		}
		abs, _ := filepath.Abs(resolved)
		return abs
	}
	slug := sanitizeBranchName(branch)
	if slug == "" {
		slug = "worktree"
	}
	return filepath.Join(repoRoot, ".openharness", "worktrees", slug)
}

func sanitizeBranchName(branch string) string {
	re := regexp.MustCompile(`[^A-Za-z0-9._-]`)
	result := re.ReplaceAllString(branch, "-")
	result = trimNewline(result)
	result = regexp.MustCompile("^-+|-+$").ReplaceAllString(result, "")
	return result
}

func mkdirAll(path string, perm os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.MkdirAll(path, perm)
}
