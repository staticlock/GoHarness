package prompts

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// EnvironmentInfo captures runtime environment details for system prompt construction.
type EnvironmentInfo struct {
	OSName          string
	OSVersion       string
	PlatformMachine string
	Shell           string
	CWD             string
	Date            string
	GoVersion       string
	IsGitRepo       bool
	GitBranch       string
}

// GetEnvironmentInfo returns environment metadata used by prompt builder.
func GetEnvironmentInfo(cwd string) EnvironmentInfo {
	if strings.TrimSpace(cwd) == "" {
		cwd, _ = os.Getwd()
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = os.Getenv("ComSpec")
	}
	env := EnvironmentInfo{
		OSName:          runtime.GOOS,
		OSVersion:       runtime.GOOS,
		PlatformMachine: runtime.GOARCH,
		Shell:           shell,
		CWD:             cwd,
		Date:            time.Now().Format(time.RFC3339),
		GoVersion:       runtime.Version(),
	}
	env.IsGitRepo, env.GitBranch = detectGit(cwd)
	return env
}

func detectGit(cwd string) (bool, string) {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return false, ""
	}
	if strings.TrimSpace(string(out)) != "true" {
		return false, ""
	}
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = cwd
	branchOut, err := branchCmd.Output()
	if err != nil {
		return true, ""
	}
	return true, strings.TrimSpace(string(branchOut))
}
