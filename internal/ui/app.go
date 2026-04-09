package ui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/staticlock/GoHarness/internal/commands"
	"github.com/staticlock/GoHarness/internal/config"
	"github.com/staticlock/GoHarness/internal/engine"
	"github.com/staticlock/GoHarness/internal/runtime"
	"github.com/staticlock/GoHarness/internal/services"
)

// PrintOptions keeps CLI-compatible print mode options.
type PrintOptions struct {
	Prompt             string
	OutputFormat       string
	CWD                string
	Model              string
	BaseURL            string
	SystemPrompt       string
	AppendSystemPrompt string
	APIKey             string
	PermissionMode     string
	MaxTurns           int
}

// ReplOptions keeps CLI-compatible interactive options.
type ReplOptions struct {
	Prompt         string
	CWD            string
	Model          string
	PermissionMode string
	MaxTurns       int
	BackendOnly    bool
	BaseURL        string
	SystemPrompt   string
	APIKey         string
	In             io.Reader
	Out            io.Writer
}

// RunPrintMode runs one non-interactive prompt.
func RunPrintMode(opts PrintOptions) error {
	bundle, err := runtime.BuildRuntime(opts.CWD, opts.Model, opts.BaseURL, opts.SystemPrompt, opts.APIKey, opts.PermissionMode, opts.MaxTurns)
	if err != nil {
		return err
	}
	runtime.StartRuntime(context.Background(), bundle)
	defer runtime.CloseRuntime(context.Background(), bundle)

	var textParts []string
	render := func(ev engine.StreamEvent) error {
		switch e := ev.(type) {
		case engine.AssistantTextDelta:
			if opts.OutputFormat == "stream-json" {
				out, _ := json.Marshal(map[string]any{"type": e.EventType(), "text": e.Text})
				fmt.Println(string(out))
			} else {
				textParts = append(textParts, e.Text)
			}
		case engine.AssistantTurnComplete:
			if opts.OutputFormat == "stream-json" {
				out, _ := json.Marshal(map[string]any{"type": e.EventType(), "usage": e.Usage})
				fmt.Println(string(out))
			}
		case engine.ToolExecutionStarted:
			if opts.OutputFormat == "stream-json" {
				out, _ := json.Marshal(map[string]any{"type": e.EventType(), "tool_name": e.ToolName, "tool_input": e.ToolInput})
				fmt.Println(string(out))
			}
		case engine.ToolExecutionCompleted:
			if opts.OutputFormat == "stream-json" {
				out, _ := json.Marshal(map[string]any{"type": e.EventType(), "tool_name": e.ToolName, "output": e.Output, "is_error": e.IsError})
				fmt.Println(string(out))
			}
		}
		return nil
	}

	if err := runtime.HandleLine(context.Background(), bundle, opts.Prompt, render); err != nil {
		return err
	}

	if opts.OutputFormat == "json" {
		out, _ := json.Marshal(map[string]any{"output": strings.Join(textParts, ""), "prompt": opts.Prompt})
		fmt.Println(string(out))
		return nil
	}
	if opts.OutputFormat != "stream-json" {
		fmt.Println(strings.Join(textParts, ""))
	}
	return nil
}

// RunREPL starts an interactive loop that supports slash commands and model prompts.
func RunREPL(opts ReplOptions) error {
	if opts.BackendOnly {
		return RunBackendHost(context.Background(), BackendHostConfig{
			Model:          opts.Model,
			BaseURL:        opts.BaseURL,
			SystemPrompt:   opts.SystemPrompt,
			APIKey:         opts.APIKey,
			CWD:            opts.CWD,
			PermissionMode: opts.PermissionMode,
			MaxTurns:       opts.MaxTurns,
		})
	}
	bundle, err := runtime.BuildRuntime(opts.CWD, opts.Model, opts.BaseURL, opts.SystemPrompt, opts.APIKey, opts.PermissionMode, opts.MaxTurns)
	if err != nil {
		return err
	}
	runtime.StartRuntime(context.Background(), bundle)
	defer runtime.CloseRuntime(context.Background(), bundle)

	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}

	registry := commands.CreateDefaultRegistry()
	handleLine := func(line string) error {
		if cmd, args := registry.Lookup(line); cmd != nil {
			res := cmd.Handler(args, commands.Context{Engine: bundle.Engine, CWD: bundle.CWD, ToolRegistry: bundle.ToolRegistry, MCPManager: bundle.MCPManager})
			if strings.TrimSpace(res.Message) != "" {
				_, _ = io.WriteString(out, res.Message+"\n")
			}
			if saveErr := saveSessionSnapshot(bundle); saveErr != nil {
				_, _ = io.WriteString(out, "warning: session snapshot save failed: "+saveErr.Error()+"\n")
			}
			if res.ShouldExit {
				return io.EOF
			}
			return nil
		}

		streamed := false
		err := runtime.HandleLine(context.Background(), bundle, line, func(ev engine.StreamEvent) error {
			switch e := ev.(type) {
			case engine.AssistantTextDelta:
				streamed = true
				_, _ = io.WriteString(out, e.Text)
			case engine.AssistantTurnComplete:
				if !streamed {
					text := strings.TrimSpace(e.Message.Text)
					if text != "" {
						_, _ = io.WriteString(out, text)
					}
				}
				_, _ = io.WriteString(out, "\n")
			case engine.ToolExecutionStarted:
				_, _ = io.WriteString(out, "[tool] "+e.ToolName+"\n")
			case engine.ToolExecutionCompleted:
				if e.IsError {
					_, _ = io.WriteString(out, "[tool-error] "+e.ToolName+": "+e.Output+"\n")
				}
			}
			return nil
		})
		if err != nil {
			_, _ = io.WriteString(out, "error: "+err.Error()+"\n")
		}
		if saveErr := saveSessionSnapshot(bundle); saveErr != nil {
			_, _ = io.WriteString(out, "warning: session snapshot save failed: "+saveErr.Error()+"\n")
		}
		return nil
	}

	if prompt := strings.TrimSpace(opts.Prompt); prompt != "" {
		if err := handleLine(prompt); err == io.EOF {
			return nil
		}
		return nil
	}

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := handleLine(line); err == io.EOF {
			return nil
		}
	}
	return scanner.Err()
}

func saveSessionSnapshot(bundle *runtime.RuntimeBundle) error {
	if bundle == nil || bundle.Engine == nil {
		return nil
	}
	settings, err := config.LoadSettings()
	if err != nil {
		return err
	}
	_, err = services.SaveSessionSnapshot(bundle.CWD, settings.Model, settings.SystemPrompt, "", bundle.Engine.Messages(), bundle.Engine.TotalUsage())
	return err
}
