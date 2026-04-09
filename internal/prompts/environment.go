package prompts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// EnvironmentInfo mirrors Python's EnvironmentInfo dataclass.
type EnvironmentInfo struct {
	OSName          string
	OSVersion       string
	PlatformMachine string
	Shell           string
	CWD             string
	HomeDir         string
	Date            string
	PythonVersion   string
	GoVersion       string
	IsGitRepo       bool
	GitBranch       string
	Hostname        string
	Extra           map[string]string
}

func detectOS() (string, string) {
	system := runtime.GOOS
	if system == "linux" {
		return "Linux", runtime.GOOS
	} else if system == "darwin" {
		return "macOS", runtime.GOOS
	} else if system == "windows" {
		return "Windows", runtime.GOOS
	}
	return system, runtime.GOOS
}

func detectShell() string {
	shell := os.Getenv("SHELL")
	if shell != "" {
		return filepath.Base(shell)
	}
	for _, candidate := range []string{"bash", "zsh", "fish", "sh"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate
		}
	}
	return "unknown"
}

func getHomeDir() string {
	home, _ := os.UserHomeDir()
	if home != "" {
		return home
	}
	return os.Getenv("HOME")
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

// GetEnvironmentInfo mirrors Python's get_environment_info function.
func GetEnvironmentInfo(cwd string) EnvironmentInfo {
	if strings.TrimSpace(cwd) == "" {
		cwd, _ = os.Getwd()
	}
	osName, osVersion := detectOS()
	shell := detectShell()
	isGit, branch := detectGit(cwd)
	hostname, _ := os.Hostname()
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}

	env := EnvironmentInfo{
		OSName:          osName,
		OSVersion:       osVersion,
		PlatformMachine: runtime.GOARCH,
		Shell:           shell,
		CWD:             cwd,
		HomeDir:         home,
		Date:            time.Now().Format("2006-01-02"),
		PythonVersion:   runtime.Version(),
		GoVersion:       runtime.Version(),
		IsGitRepo:       isGit,
		GitBranch:       branch,
		Hostname:        hostname,
		Extra:           map[string]string{},
	}
	return env
}
