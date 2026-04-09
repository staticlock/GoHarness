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
	ReactTUI                   bool
}

// Execute runs the CLI root command.
func Execute() error {
	opts := &rootOptions{}
	root := newRootCommand(opts)
	return root.Execute()
}

func newRootCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "goHarness",
		Short: "Oh my goHarness! An AI-powered coding assistant.",
		Long:  "Oh my goHarness! An AI-powered coding assistant.\n\nStarts an interactive session by default, use -p/--print for non-interactive output.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.DangerouslySkipPermissions {
				opts.PermissionMode = "full_auto"
			}
			if opts.ReactTUI {
				if strings.TrimSpace(opts.PrintMode) != "" {
					return errors.New("--react-tui cannot be used with --print")
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
			//检查名为 "print" 的标志是否在命令行中被设置
			if cmd.Flags().Changed("print") && strings.TrimSpace(opts.PrintMode) == "" {
				return errors.New("-p/--print requires a prompt value, e.g. -p 'your prompt'")
			}
			//REPL = Read Eval Print Loop（读取-求值-输出-循环）
			return ui.RunREPL(ui.ReplOptions{
				Prompt:         "",
				CWD:            opts.CWD,
				Model:          opts.Model,
				BaseURL:        opts.BaseURL,
				SystemPrompt:   opts.SystemPrompt,
				APIKey:         opts.APIKey,
				PermissionMode: opts.PermissionMode,
				MaxTurns:       opts.MaxTurns,
				BackendOnly:    opts.BackendOnly,
			})
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
	cmd.Flags().BoolVarP(&opts.ContinueSession, "continue", "c", false, "Continue the most recent conversation in the current directory")
	cmd.Flags().StringVarP(&opts.Resume, "resume", "r", "", "Resume a conversation by session ID, or open picker")
	cmd.Flags().StringVarP(&opts.Name, "name", "n", "", "Set a display name for this session")
	cmd.Flags().StringVarP(&opts.Model, "model", "m", "", "Model alias (e.g. 'sonnet', 'opus') or full model ID")
	cmd.Flags().StringVar(&opts.Effort, "effort", "", "Effort level for the session (low, medium, high, max)")
	cmd.Flags().BoolVar(&opts.Verbose, "verbose", false, "Override verbose mode setting from config")
	cmd.Flags().IntVar(&opts.MaxTurns, "max-turns", 0, "Maximum number of agentic turns (useful with --print)")
	cmd.Flags().StringVarP(&opts.PrintMode, "print", "p", "", "Print response and exit. Pass your prompt as the value: -p 'your prompt'")
	cmd.Flags().StringVar(&opts.OutputFormat, "output-format", "", "Output format with --print: text (default), json, or stream-json")
	cmd.Flags().StringVar(&opts.PermissionMode, "permission-mode", "", "Permission mode: default, plan, or full_auto")
	cmd.Flags().BoolVar(&opts.DangerouslySkipPermissions, "dangerously-skip-permissions", false, "Bypass all permission checks (only for sandboxed environments)")
	cmd.Flags().StringSliceVar(&opts.AllowedTools, "allowed-tools", nil, "Comma or space-separated list of tool names to allow")
	cmd.Flags().StringSliceVar(&opts.DisallowedTools, "disallowed-tools", nil, "Comma or space-separated list of tool names to deny")
	cmd.Flags().StringVarP(&opts.SystemPrompt, "system-prompt", "s", "", "Override the default system prompt")
	cmd.Flags().StringVar(&opts.AppendSystemPrompt, "append-system-prompt", "", "Append text to the default system prompt")
	cmd.Flags().StringVar(&opts.SettingsFile, "settings", "", "Path to a JSON settings file or inline JSON string")
	cmd.Flags().StringVar(&opts.BaseURL, "base-url", "", "Anthropic-compatible API base URL")
	cmd.Flags().StringVarP(&opts.APIKey, "api-key", "k", "", "API key (overrides config and environment)")
	cmd.Flags().BoolVar(&opts.Bare, "bare", false, "Minimal mode: skip hooks, plugins, MCP, and auto-discovery")
	cmd.Flags().BoolVarP(&opts.Debug, "debug", "d", false, "Enable debug logging")
	cmd.Flags().StringSliceVar(&opts.MCPConfig, "mcp-config", nil, "Load MCP servers from JSON files or strings")
	cmd.Flags().StringVar(&opts.CWD, "cwd", wd, "Working directory for the session")
	cmd.Flags().MarkHidden("cwd")
	cmd.Flags().BoolVar(&opts.BackendOnly, "backend-only", false, "Run the structured backend host for the React terminal UI")
	cmd.Flags().MarkHidden("backend-only")
	// 这里必须默认false ,不然后面前端使用命令启动会出问题
	cmd.Flags().BoolVar(&opts.ReactTUI, "react-tui", false, "Launch the React terminal UI frontend")
}

func defaultString(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}
