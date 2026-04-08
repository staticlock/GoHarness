package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	ErrFrontendMissing   = errors.New("react terminal frontend is missing")
	ErrDependencyInstall = errors.New("failed to install frontend dependencies")
	ErrNpmNotFound       = errors.New("npm executable not found")
	ErrProcessWait       = errors.New("process exited with error")
)

// FrontendConfig 前端需要的配置
type FrontendConfig struct {
	BackendCommand []string `json:"backend_command"`
	InitialPrompt  *string  `json:"initial_prompt,omitempty"`
}

// BackendParams 后端参数
type BackendParams struct {
	Cwd          string
	Model        string
	BaseURL      string
	SystemPrompt string
	APIKey       string
}

// LaunchOptions 启动选项
type LaunchOptions struct {
	Prompt       *string // 初始提示词（可选）
	Cwd          string  // 工作目录
	Model        string  // 模型名称
	BaseURL      string  // API 基础 URL
	SystemPrompt string  // 系统提示词
	APIKey       string  // API 密钥
}

// TerminalFrontend React 终端前端管理器
type TerminalFrontend struct {
	repoRoot      string       // 项目根目录
	frontendDir   string       // 前端目录
	npmCmd        string       // npm 命令路径
	mu            sync.RWMutex // 保护 depsInstalled 的读写锁
	depsInstalled bool         // 依赖是否已安装
}

// NewTerminalFrontend 创建前端管理器
func NewTerminalFrontend() (*TerminalFrontend, error) {
	root, err := getRepoRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to get repo root: %w", err)
	}

	// 前端路径
	frontendDir := filepath.Join(root, "frontend", "terminal")
	if _, err := os.Stat(filepath.Join(frontendDir, "package.json")); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrFrontendMissing, filepath.Join(frontendDir, "package.json"))
	}
	// npm 命令路径
	npmCmd := resolveNpm()
	if npmCmd == "" {
		return nil, ErrNpmNotFound
	}

	return &TerminalFrontend{
		repoRoot:      root,
		frontendDir:   frontendDir,
		npmCmd:        npmCmd,
		depsInstalled: false,
	}, nil
}

// GetFrontendDir 返回前端目录
func (tf *TerminalFrontend) GetFrontendDir() string {
	return tf.frontendDir
}

// EnsureDependencies 确保依赖已安装
func (tf *TerminalFrontend) EnsureDependencies(ctx context.Context) error {
	tf.mu.RLock()
	if tf.depsInstalled {
		tf.mu.RUnlock()
		return nil
	}
	tf.mu.RUnlock()

	tf.mu.Lock()
	defer tf.mu.Unlock()

	// 双重检查
	if tf.depsInstalled {
		return nil
	}

	// 检查 package.json 是否存在
	packageJSON := filepath.Join(tf.frontendDir, "package.json")
	if _, err := os.Stat(packageJSON); err != nil {
		return fmt.Errorf("%w: %s", ErrFrontendMissing, packageJSON)
	}

	// 检查 node_modules 是否存在
	nodeModules := filepath.Join(tf.frontendDir, "node_modules")
	if _, err := os.Stat(nodeModules); err == nil {
		tf.depsInstalled = true
		return nil
	}

	// 安装依赖
	cmd := exec.CommandContext(ctx, tf.npmCmd, "install", "--no-fund", "--no-audit")
	cmd.Dir = tf.frontendDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", ErrDependencyInstall, stderr.String())
	}

	tf.depsInstalled = true
	return nil
}

// BuildBackendCommand 构建后端命令
func (tf *TerminalFrontend) BuildBackendCommand(params BackendParams) []string {
	exe := os.Args[0]
	if resolved, err := os.Executable(); err == nil && strings.TrimSpace(resolved) != "" {
		exe = resolved
	}
	cmd := []string{exe, "--backend-only"}

	if params.Cwd != "" {
		cmd = append(cmd, "--cwd", params.Cwd)
	}
	if params.Model != "" {
		cmd = append(cmd, "--model", params.Model)
	}
	if params.BaseURL != "" {
		cmd = append(cmd, "--base-url", params.BaseURL)
	}
	if params.SystemPrompt != "" {
		cmd = append(cmd, "--system-prompt", params.SystemPrompt)
	}
	if params.APIKey != "" {
		cmd = append(cmd, "--api-key", params.APIKey)
	}

	return cmd
}

// Launch 启动 React 终端 UI
func (tf *TerminalFrontend) Launch(ctx context.Context, opts LaunchOptions) (int, error) {
	// 确保依赖已安装
	if err := tf.EnsureDependencies(ctx); err != nil {
		return -1, err
	}

	// 准备环境变量
	env := os.Environ()

	// 构建配置
	cwd := opts.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	config := FrontendConfig{
		BackendCommand: tf.BuildBackendCommand(BackendParams{
			Cwd:          cwd,
			Model:        opts.Model,
			BaseURL:      opts.BaseURL,
			SystemPrompt: opts.SystemPrompt,
			APIKey:       opts.APIKey,
		}),
		InitialPrompt: opts.Prompt,
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return -1, fmt.Errorf("failed to marshal config: %w", err)
	}

	fmt.Println("Launching Frontend ...", string(configJSON))
	env = append(env, "OPENHARNESS_FRONTEND_CONFIG="+string(configJSON))

	// 启动前端进程
	cmd := exec.CommandContext(ctx, tf.npmCmd, "run", "start", "--silent")
	cmd.Dir = tf.frontendDir
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return -1, fmt.Errorf("failed to start frontend: %w", err)
	}
	// 等待进程结束
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return -1, fmt.Errorf("%w: %w", ErrProcessWait, err)
	}
	return 0, nil
}

// LaunchSimple 简单启动（使用默认选项）
func (tf *TerminalFrontend) LaunchSimple(ctx context.Context) (int, error) {
	return tf.Launch(ctx, LaunchOptions{})
}

// Helper functions

// getRepoRoot 获取项目根目录
func getRepoRoot() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("failed to get current file path")
	}

	if root, ok := findRepoRoot(filepath.Dir(filename)); ok {
		return root, nil
	}
	if wd, err := os.Getwd(); err == nil {
		if root, ok := findRepoRoot(wd); ok {
			return root, nil
		}
	}
	return "", errors.New("failed to locate repository root")
}
func findRepoRoot(start string) (string, bool) {
	cur := start
	for {
		if _, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil {
			return cur, true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", false
		}
		cur = parent
	}
}

// resolveNpm 解析 npm 可执行文件
func resolveNpm() string {
	// Windows 上需要 npm.cmd
	if runtime.GOOS == "windows" {
		if path, err := exec.LookPath("npm.cmd"); err == nil {
			return path
		}
	}

	// Unix-like 系统
	if path, err := exec.LookPath("npm"); err == nil {
		return path
	}

	return ""
}

// GetFrontendDir 获取前端目录（全局函数，保持 API 兼容）
func GetFrontendDir() (string, error) {
	root, err := getRepoRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "frontend", "terminal"), nil
}

// BuildBackendCommand 构建后端命令（全局函数）
func BuildBackendCommand(params BackendParams) ([]string, error) {
	exe := os.Args[0]
	if resolved, err := os.Executable(); err == nil && strings.TrimSpace(resolved) != "" {
		exe = resolved
	}
	cmd := []string{exe, "--backend-only"}

	if params.Cwd != "" {
		cmd = append(cmd, "--cwd", params.Cwd)
	}
	if params.Model != "" {
		cmd = append(cmd, "--model", params.Model)
	}
	if params.BaseURL != "" {
		cmd = append(cmd, "--base-url", params.BaseURL)
	}
	if params.SystemPrompt != "" {
		cmd = append(cmd, "--system-prompt", params.SystemPrompt)
	}
	if params.APIKey != "" {
		cmd = append(cmd, "--api-key", params.APIKey)
	}

	return cmd, nil
}

// LaunchReactTUI 启动 React TUI（全局函数）
func LaunchReactTUI(ctx context.Context, opts LaunchOptions) (int, error) {
	frontend, err := NewTerminalFrontend()
	if err != nil {
		return -1, err
	}
	return frontend.Launch(ctx, opts)
}
