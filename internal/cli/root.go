package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/staticlock/GoHarness/internal/ui"
)

type rootOptions struct {
	ContinueSession            bool
	Resume                     string
	Name                       string
	Model                      string
	Effort                     string
	Verbose                    bool
	MaxTurns                   int
	PrintMode                  string
	OutputFormat               string
	PermissionMode             string
	DangerouslySkipPermissions bool
	AllowedTools               []string
	DisallowedTools            []string
	SystemPrompt               string
	AppendSystemPrompt         string
	SettingsFile               string
	BaseURL                    string
	APIKey                     string
	Bare                       bool
	Debug                      bool
	MCPConfig                  []string
	CWD                        string
	BackendOnly                bool
}

// Execute runs the CLI root command.
func Execute() error {
	opts := &rootOptions{}
	root := newRootCommand(opts)
	return root.Execute()
}

func newRootCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "openharness",
		Short: "Oh my Harness! An AI-powered coding assistant.",
		Long:  "Oh my Harness! An AI-powered coding assistant.\n\nStarts an interactive session by default, use -p/--print for non-interactive output.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.DangerouslySkipPermissions {
				opts.PermissionMode = "full_auto"
			}
			if opts.BackendOnly {
				if strings.TrimSpace(opts.PrintMode) != "" {
					return errors.New("--backend-only cannot be used with --print")
				}
				return ui.RunBackendHost(context.Background(), ui.BackendHostConfig{
					CWD:          opts.CWD,
					Model:        opts.Model,
					BaseURL:      opts.BaseURL,
					SystemPrompt: opts.SystemPrompt,
					APIKey:       opts.APIKey,
				})
			}
			if strings.TrimSpace(opts.PrintMode) != "" {
				return ui.RunPrintMode(ui.PrintOptions{
					Prompt:             strings.TrimSpace(opts.PrintMode),
					OutputFormat:       defaultString(opts.OutputFormat, "text"),
					CWD:                opts.CWD,
					Model:              opts.Model,
					BaseURL:            opts.BaseURL,
					SystemPrompt:       opts.SystemPrompt,
					AppendSystemPrompt: opts.AppendSystemPrompt,
					APIKey:             opts.APIKey,
					PermissionMode:     opts.PermissionMode,
					MaxTurns:           opts.MaxTurns,
				})
			}
			if cmd.Flags().Changed("print") && strings.TrimSpace(opts.PrintMode) == "" {
				return errors.New("-p/--print requires a prompt value, e.g. -p 'your prompt'")
			}
			exitCode, err := ui.LaunchReactTUI(context.Background(), ui.LaunchOptions{
				Cwd:          opts.CWD,
				Model:        opts.Model,
				BaseURL:      opts.BaseURL,
				SystemPrompt: opts.SystemPrompt,
				APIKey:       opts.APIKey,
			})
			if err != nil {
				return err
			}
			if exitCode != 0 {
				return fmt.Errorf("react tui exited with code %d", exitCode)
			}
			return nil
		},
	}

	registerRootFlags(cmd, opts)
	cmd.AddCommand(newMCPCommand())
	cmd.AddCommand(newPluginCommand())
	cmd.AddCommand(newAuthCommand())
	return cmd
}

func registerRootFlags(cmd *cobra.Command, opts *rootOptions) {
	wd, _ := os.Getwd()
	// 继续当前目录中最新的对话会话
	cmd.Flags().BoolVarP(&opts.ContinueSession, "continue", "c", false, "Continue the most recent conversation in the current directory")
	// 通过会话ID恢复对话，或打开选择器选择要恢复的会话
	cmd.Flags().StringVarP(&opts.Resume, "resume", "r", "", "Resume a conversation by session ID, or open picker")
	// 为当前会话设置一个显示名称
	cmd.Flags().StringVarP(&opts.Name, "name", "n", "", "Set a display name for this session")
	// 指定模型别名（如 'sonnet', 'opus'）或完整的模型ID
	cmd.Flags().StringVarP(&opts.Model, "model", "m", "", "Model alias (e.g. 'sonnet', 'opus') or full model ID")
	// 设置会话的工作量级别（low, medium, high, max）
	cmd.Flags().StringVar(&opts.Effort, "effort", "", "Effort level for the session (low, medium, high, max)")
	// 覆盖配置文件中的详细输出模式设置
	cmd.Flags().BoolVar(&opts.Verbose, "verbose", false, "Override verbose mode setting from config")
	// 最大代理交互轮数（与 --print 一起使用时很有用）
	cmd.Flags().IntVar(&opts.MaxTurns, "max-turns", 0, "Maximum number of agentic turns (useful with --print)")
	// 打印响应后退出，将提示词作为值传入：-p '你的提示词'
	cmd.Flags().StringVarP(&opts.PrintMode, "print", "p", "", "Print response and exit. Pass your prompt as the value: -p 'your prompt'")
	// 与 --print 一起使用时的输出格式：text（默认）、json 或 stream-json
	cmd.Flags().StringVar(&opts.OutputFormat, "output-format", "", "Output format with --print: text (default), json, or stream-json")
	// 权限模式：default（默认）、plan（计划模式）或 full_auto（完全自动）
	cmd.Flags().StringVar(&opts.PermissionMode, "permission-mode", "", "Permission mode: default, plan, or full_auto")
	// 绕过所有权限检查（仅在沙箱环境中使用）
	cmd.Flags().BoolVar(&opts.DangerouslySkipPermissions, "dangerously-skip-permissions", false, "Bypass all permission checks (only for sandboxed environments)")
	// 允许使用的工具列表，以逗号或空格分隔的工具名称
	cmd.Flags().StringSliceVar(&opts.AllowedTools, "allowed-tools", nil, "Comma or space-separated list of tool names to allow")
	// 禁止使用的工具列表，以逗号或空格分隔的工具名称
	cmd.Flags().StringSliceVar(&opts.DisallowedTools, "disallowed-tools", nil, "Comma or space-separated list of tool names to deny")
	// 覆盖默认的系统提示词
	cmd.Flags().StringVarP(&opts.SystemPrompt, "system-prompt", "s", "", "Override the default system prompt")
	// 在默认系统提示词后面追加文本内容
	cmd.Flags().StringVar(&opts.AppendSystemPrompt, "append-system-prompt", "", "Append text to the default system prompt")
	// 加载JSON设置文件或内联JSON字符串的路径
	cmd.Flags().StringVar(&opts.SettingsFile, "settings", "", "Path to a JSON settings file or inline JSON string")
	// Anthropic兼容的API基础URL
	cmd.Flags().StringVar(&opts.BaseURL, "base-url", "", "Anthropic-compatible API base URL")
	// API密钥（覆盖配置文件和环境的设置）
	cmd.Flags().StringVarP(&opts.APIKey, "api-key", "k", "", "API key (overrides config and environment)")
	// 最小化模式：跳过钩子、插件、MCP和自动发现功能
	cmd.Flags().BoolVar(&opts.Bare, "bare", false, "Minimal mode: skip hooks, plugins, MCP, and auto-discovery")
	// 启用调试日志输出
	cmd.Flags().BoolVarP(&opts.Debug, "debug", "d", false, "Enable debug logging")
	// 从JSON文件或字符串加载MCP服务器配置
	cmd.Flags().StringSliceVar(&opts.MCPConfig, "mcp-config", nil, "Load MCP servers from JSON files or strings")
	// 会话的工作目录（默认使用当前目录）
	cmd.Flags().StringVar(&opts.CWD, "cwd", wd, "Working directory for the session")
	// 隐藏cwd参数不在帮助信息中显示
	cmd.Flags().MarkHidden("cwd")
	// 运行React终端UI的结构化后端主机
	cmd.Flags().BoolVar(&opts.BackendOnly, "backend-only", false, "Run the structured backend host for the React terminal UI")
	// 隐藏backend-only参数不在帮助信息中显示
	cmd.Flags().MarkHidden("backend-only")
}

func defaultString(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}
