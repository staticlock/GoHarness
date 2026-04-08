package commands

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/user/goharness/internal/api"
	"github.com/user/goharness/internal/bridge"
	"github.com/user/goharness/internal/config"
	"github.com/user/goharness/internal/engine"
	"github.com/user/goharness/internal/hooks"
	"github.com/user/goharness/internal/mcp"
	"github.com/user/goharness/internal/permissions"
	"github.com/user/goharness/internal/plugins"
	"github.com/user/goharness/internal/services"
	"github.com/user/goharness/internal/skills"
	"github.com/user/goharness/internal/tasks"
	"github.com/user/goharness/internal/tools"
)

// Result is the return payload for one slash command execution.
type Result struct {
	Message     string
	ShouldExit  bool
	ClearScreen bool
}

// Context carries runtime objects used by command handlers.
type Context struct {
	Engine       *engine.QueryEngine
	CWD          string
	ToolRegistry *tools.Registry
	MCPManager   *mcp.ClientManager
}

// Handler executes one slash command.
type Handler func(args string, ctx Context) Result

// Command defines one slash command.
type Command struct {
	Name        string
	Description string
	Handler     Handler
}

// Registry stores slash command handlers.
type Registry struct {
	commands map[string]Command
	order    []string
}

// NewRegistry creates an empty command registry.
func NewRegistry() *Registry {
	return &Registry{commands: map[string]Command{}, order: []string{}}
}

// Register adds or replaces one command.
func (r *Registry) Register(cmd Command) {
	if _, exists := r.commands[cmd.Name]; !exists {
		r.order = append(r.order, cmd.Name)
	}
	r.commands[cmd.Name] = cmd
}

// Lookup parses slash input and returns command + args.
func (r *Registry) Lookup(rawInput string) (*Command, string) {
	if !strings.HasPrefix(rawInput, "/") {
		return nil, ""
	}
	name, args, _ := strings.Cut(strings.TrimPrefix(strings.TrimSpace(rawInput), "/"), " ")
	cmd, ok := r.commands[name]
	if !ok {
		return nil, ""
	}
	return &cmd, strings.TrimSpace(args)
}

// ListCommands returns all registered commands in registration order.
func (r *Registry) ListCommands() []Command {
	out := make([]Command, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.commands[name])
	}
	return out
}

// HelpText returns a user-facing command list.
func (r *Registry) HelpText() string {
	lines := []string{"Available commands:"}
	for _, cmd := range r.ListCommands() {
		lines = append(lines, fmt.Sprintf("/%-12s %s", cmd.Name, cmd.Description))
	}
	return strings.Join(lines, "\n")
}

// CreateDefaultRegistry builds the minimum parity command set used by Go runtime paths.
func CreateDefaultRegistry() *Registry {
	registry := NewRegistry()

	registry.Register(Command{
		Name:        "help",
		Description: "Show available slash commands",
		Handler: func(_ string, ctx Context) Result {
			_ = ctx
			return Result{Message: registry.HelpText()}
		},
	})

	registry.Register(Command{
		Name:        "version",
		Description: "Show OpenHarness version",
		Handler: func(_ string, ctx Context) Result {
			_ = ctx
			return Result{Message: "OpenHarness go"}
		},
	})

	registry.Register(Command{
		Name:        "clear",
		Description: "Clear in-memory conversation",
		Handler: func(_ string, ctx Context) Result {
			ctx.Engine.Clear()
			return Result{Message: "Conversation cleared.", ClearScreen: true}
		},
	})

	registry.Register(Command{
		Name:        "status",
		Description: "Show message count and token usage",
		Handler: func(_ string, ctx Context) Result {
			usage := ctx.Engine.TotalUsage()
			return Result{Message: fmt.Sprintf("Messages: %d\nUsage: input=%d output=%d total=%d", len(ctx.Engine.Messages()), usage.InputTokens, usage.OutputTokens, usage.TotalTokens)}
		},
	})

	registry.Register(Command{
		Name:        "usage",
		Description: "Show token usage and estimated conversation size",
		Handler: func(_ string, ctx Context) Result {
			usage := ctx.Engine.TotalUsage()
			estimated := estimateConversationTokens(ctx.Engine.Messages())
			return Result{Message: fmt.Sprintf("Actual usage: input=%d output=%d\nEstimated conversation tokens: %d\nMessages: %d", usage.InputTokens, usage.OutputTokens, estimated, len(ctx.Engine.Messages()))}
		},
	})

	registry.Register(Command{
		Name:        "cost",
		Description: "Show token usage and estimated cost",
		Handler: func(_ string, ctx Context) Result {
			usage := ctx.Engine.TotalUsage()
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			model := settings.Model
			estimatedCost := "unavailable"
			switch {
			case strings.HasPrefix(model, "claude-3-5-sonnet"), strings.HasPrefix(model, "claude-3-7-sonnet"):
				estimated := (float64(usage.InputTokens)*3.0 + float64(usage.OutputTokens)*15.0) / 1_000_000.0
				estimatedCost = fmt.Sprintf("$%.4f (estimated)", estimated)
			case strings.HasPrefix(model, "claude-3-opus"):
				estimated := (float64(usage.InputTokens)*15.0 + float64(usage.OutputTokens)*75.0) / 1_000_000.0
				estimatedCost = fmt.Sprintf("$%.4f (estimated)", estimated)
			}
			return Result{Message: fmt.Sprintf("Model: %s\nInput tokens: %d\nOutput tokens: %d\nTotal tokens: %d\nEstimated cost: %s", model, usage.InputTokens, usage.OutputTokens, usage.TotalTokens, estimatedCost)}
		},
	})

	registry.Register(Command{
		Name:        "context",
		Description: "Show the active runtime system prompt",
		Handler: func(_ string, ctx Context) Result {
			prompt := strings.TrimSpace(ctx.Engine.SystemPrompt())
			if prompt == "" {
				return Result{Message: "(no system prompt configured)"}
			}
			return Result{Message: prompt}
		},
	})

	registry.Register(Command{
		Name:        "summary",
		Description: "Summarize recent conversation history",
		Handler: func(args string, ctx Context) Result {
			maxMessages := 8
			if strings.TrimSpace(args) != "" {
				parsed, err := strconv.Atoi(strings.TrimSpace(args))
				if err != nil || parsed <= 0 {
					return Result{Message: "Usage: /summary [MAX_MESSAGES]"}
				}
				maxMessages = parsed
			}
			return Result{Message: summarizeMessages(ctx.Engine.Messages(), maxMessages)}
		},
	})

	registry.Register(Command{
		Name:        "compact",
		Description: "Compact older conversation history",
		Handler: func(args string, ctx Context) Result {
			preserveRecent := 6
			if strings.TrimSpace(args) != "" {
				parsed, err := strconv.Atoi(strings.TrimSpace(args))
				if err != nil || parsed <= 0 {
					return Result{Message: "Usage: /compact [PRESERVE_RECENT]"}
				}
				preserveRecent = parsed
			}
			messages := ctx.Engine.Messages()
			before := len(messages)
			if before <= preserveRecent {
				return Result{Message: fmt.Sprintf("Compacted conversation from %d messages to %d.", before, before)}
			}
			kept := append([]engine.ConversationMessage{}, messages[before-preserveRecent:]...)
			ctx.Engine.LoadMessages(kept)
			return Result{Message: fmt.Sprintf("Compacted conversation from %d messages to %d.", before, len(kept))}
		},
	})

	registry.Register(Command{
		Name:        "stats",
		Description: "Show session statistics",
		Handler: func(_ string, ctx Context) Result {
			toolCount := 0
			if ctx.ToolRegistry != nil {
				toolCount = len(ctx.ToolRegistry.List())
			}
			messages := ctx.Engine.Messages()
			return Result{Message: fmt.Sprintf("Session stats:\n- messages: %d\n- estimated_tokens: %d\n- tools: %d", len(messages), estimateConversationTokens(messages), toolCount)}
		},
	})

	registry.Register(Command{
		Name:        "resume",
		Description: "Resume latest or specified saved session",
		Handler: func(args string, ctx Context) Result {
			target := strings.TrimSpace(args)
			if target == "" {
				sessions, err := services.ListSessions(ctx.CWD, 10)
				if err != nil || len(sessions) == 0 {
					latest, latestErr := services.LoadLatestSession(ctx.CWD)
					if latestErr != nil || latest == nil {
						return Result{Message: "No saved sessions found for this project."}
					}
					ctx.Engine.LoadMessages(latest.Messages)
					return Result{Message: fmt.Sprintf("Restored %d messages from the latest session.", len(latest.Messages))}
				}
				lines := []string{"Saved sessions:"}
				for _, s := range sessions {
					summary := strings.TrimSpace(s.Summary)
					if summary == "" {
						summary = "(no summary)"
					}
					lines = append(lines, fmt.Sprintf("  %s  %dmsg  %s", s.SessionID, s.MessageCount, summary))
				}
				lines = append(lines, "", "Use /resume <session_id> to restore a specific session.")
				return Result{Message: strings.Join(lines, "\n")}
			}
			snapshot, err := services.LoadSessionByID(ctx.CWD, target)
			if err != nil || snapshot == nil {
				return Result{Message: "Session not found: " + target}
			}
			ctx.Engine.LoadMessages(snapshot.Messages)
			return Result{Message: fmt.Sprintf("Restored %d messages from session %s", len(snapshot.Messages), snapshot.SessionID)}
		},
	})

	registry.Register(Command{
		Name:        "export",
		Description: "Export the current transcript",
		Handler: func(_ string, ctx Context) Result {
			path, err := exportSessionMarkdown(ctx.CWD, ctx.Engine.Messages())
			if err != nil {
				return Result{Message: "Failed to export transcript: " + err.Error()}
			}
			return Result{Message: "Exported transcript to " + path}
		},
	})

	registry.Register(Command{
		Name:        "share",
		Description: "Create a shareable transcript snapshot",
		Handler: func(_ string, ctx Context) Result {
			path, err := exportSessionMarkdown(ctx.CWD, ctx.Engine.Messages())
			if err != nil {
				return Result{Message: "Failed to create shareable transcript: " + err.Error()}
			}
			return Result{Message: "Created shareable transcript snapshot at " + path}
		},
	})

	registry.Register(Command{
		Name:        "copy",
		Description: "Copy the latest response or provided text",
		Handler: func(args string, ctx Context) Result {
			text := strings.TrimSpace(args)
			if text == "" {
				text = lastMessageText(ctx.Engine.Messages())
			}
			if strings.TrimSpace(text) == "" {
				return Result{Message: "Nothing to copy."}
			}
			dataDir, err := config.DataDir()
			if err != nil {
				return Result{Message: "Failed to resolve clipboard fallback path: " + err.Error()}
			}
			path := filepath.Join(dataDir, "last_copy.txt")
			if writeErr := os.WriteFile(path, []byte(text), 0o644); writeErr != nil {
				return Result{Message: "Failed to save copied text: " + writeErr.Error()}
			}
			return Result{Message: "Clipboard unavailable. Saved copied text to " + path}
		},
	})

	registry.Register(Command{
		Name:        "session",
		Description: "Inspect current session storage",
		Handler: func(args string, ctx Context) Result {
			tokens := strings.Fields(args)
			sessionDir, err := services.ProjectSessionDir(ctx.CWD)
			if err != nil {
				return Result{Message: "Failed to resolve session directory: " + err.Error()}
			}
			if len(tokens) == 0 || (tokens[0] == "show" && len(tokens) == 1) {
				latest, latestErr := services.LoadLatestSession(ctx.CWD)
				if latestErr != nil || latest == nil {
					return Result{Message: fmt.Sprintf("Session directory: %s\nLatest snapshot: (none)", sessionDir)}
				}
				return Result{Message: fmt.Sprintf("Session directory: %s\nLatest snapshot: %s (%d messages)", sessionDir, latest.SessionID, len(latest.Messages))}
			}
			if tokens[0] == "list" || tokens[0] == "ls" {
				sessions, listErr := services.ListSessions(ctx.CWD, 10)
				if listErr != nil || len(sessions) == 0 {
					return Result{Message: "No saved sessions found for this project."}
				}
				lines := []string{"Saved sessions:"}
				for _, s := range sessions {
					lines = append(lines, fmt.Sprintf("  %s  %dmsg  %s", s.SessionID, s.MessageCount, strings.TrimSpace(s.Summary)))
				}
				return Result{Message: strings.Join(lines, "\n")}
			}
			if tokens[0] == "latest" {
				latest, latestErr := services.LoadLatestSession(ctx.CWD)
				if latestErr != nil || latest == nil {
					return Result{Message: "No latest session snapshot."}
				}
				return Result{Message: fmt.Sprintf("Session %s\nMessages: %d\nModel: %s", latest.SessionID, len(latest.Messages), latest.Model)}
			}
			if tokens[0] == "path" {
				return Result{Message: sessionDir}
			}
			if tokens[0] == "clear" {
				_ = os.RemoveAll(sessionDir)
				if mkErr := os.MkdirAll(sessionDir, 0o755); mkErr != nil {
					return Result{Message: "Failed to clear session storage: " + mkErr.Error()}
				}
				return Result{Message: "Cleared session storage at " + sessionDir}
			}
			if tokens[0] == "tag" && len(tokens) >= 2 {
				tagName := safeTagName(strings.Join(tokens[1:], " "))
				if tagName == "" {
					return Result{Message: "Usage: /session tag NAME"}
				}
				settings, loadErr := config.LoadSettings()
				if loadErr != nil {
					return Result{Message: "Failed to load settings: " + loadErr.Error()}
				}
				if _, saveErr := services.SaveSessionSnapshot(ctx.CWD, settings.Model, ctx.Engine.SystemPrompt(), "", ctx.Engine.Messages(), ctx.Engine.TotalUsage()); saveErr != nil {
					return Result{Message: "Failed to save snapshot before tagging: " + saveErr.Error()}
				}
				exportPath, exportErr := exportSessionMarkdown(ctx.CWD, ctx.Engine.Messages())
				if exportErr != nil {
					return Result{Message: "Failed to export snapshot before tagging: " + exportErr.Error()}
				}
				latestJSON := filepath.Join(sessionDir, "latest.json")
				taggedJSON := filepath.Join(sessionDir, tagName+".json")
				taggedMD := filepath.Join(sessionDir, tagName+".md")
				if copyErr := copyFile(latestJSON, taggedJSON); copyErr != nil {
					return Result{Message: "Failed to tag session json: " + copyErr.Error()}
				}
				if copyErr := copyFile(exportPath, taggedMD); copyErr != nil {
					return Result{Message: "Failed to tag session markdown: " + copyErr.Error()}
				}
				return Result{Message: "Tagged session as " + tagName + ":\n- " + taggedJSON + "\n- " + taggedMD}
			}
			if (tokens[0] == "show" || tokens[0] == "id") && len(tokens) == 2 {
				snapshot, loadErr := services.LoadSessionByID(ctx.CWD, strings.TrimSpace(tokens[1]))
				if loadErr != nil || snapshot == nil {
					return Result{Message: "Session not found: " + strings.TrimSpace(tokens[1])}
				}
				return Result{Message: fmt.Sprintf("Session %s\nMessages: %d\nModel: %s\nSummary: %s", snapshot.SessionID, len(snapshot.Messages), snapshot.Model, strings.TrimSpace(snapshot.Summary))}
			}
			return Result{Message: "Usage: /session [show|ls|list|path|latest|show ID|id ID|tag NAME|clear]"}
		},
	})

	registry.Register(Command{
		Name:        "rewind",
		Description: "Remove the latest conversation turn(s)",
		Handler: func(args string, ctx Context) Result {
			turns := 1
			if strings.TrimSpace(args) != "" {
				parsed, err := strconv.Atoi(strings.TrimSpace(args))
				if err != nil || parsed <= 0 {
					return Result{Message: "Usage: /rewind [TURNS]"}
				}
				turns = parsed
			}
			before := len(ctx.Engine.Messages())
			updated := rewindTurns(ctx.Engine.Messages(), turns)
			ctx.Engine.LoadMessages(updated)
			removed := before - len(updated)
			return Result{Message: fmt.Sprintf("Rewound %d turn(s); removed %d message(s).", turns, removed)}
		},
	})

	registry.Register(Command{
		Name:        "tag",
		Description: "Create a named snapshot of the current session",
		Handler: func(args string, ctx Context) Result {
			name := strings.TrimSpace(args)
			if name == "" {
				return Result{Message: "Usage: /tag NAME"}
			}
			return registry.commands["session"].Handler("tag "+name, ctx)
		},
	})

	registry.Register(Command{
		Name:        "files",
		Description: "List files in the current workspace",
		Handler: func(args string, ctx Context) Result {
			tokens := strings.Fields(args)
			root := ctx.CWD
			limit := 50
			if len(tokens) >= 1 {
				root = filepath.Join(ctx.CWD, tokens[0])
			}
			if len(tokens) >= 2 {
				parsed, err := strconv.Atoi(tokens[1])
				if err != nil || parsed <= 0 {
					return Result{Message: "Usage: /files [PATH] [LIMIT]"}
				}
				limit = parsed
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				return Result{Message: "Failed to list files: " + err.Error()}
			}
			items := make([]string, 0, len(entries))
			for _, e := range entries {
				name := e.Name()
				if e.IsDir() {
					name += "/"
				}
				items = append(items, name)
			}
			sort.Strings(items)
			if len(items) == 0 {
				return Result{Message: "(no files)"}
			}
			if len(items) > limit {
				items = items[:limit]
			}
			return Result{Message: strings.Join(items, "\n")}
		},
	})

	registry.Register(Command{
		Name:        "tasks",
		Description: "Inspect background tasks",
		Handler: func(args string, ctx Context) Result {
			manager := tasks.DefaultManager()
			tokens := strings.Fields(args)
			if len(tokens) == 0 || tokens[0] == "list" {
				records := manager.ListTasks("")
				if len(records) == 0 {
					return Result{Message: "No background tasks."}
				}
				lines := make([]string, 0, len(records))
				for _, record := range records {
					lines = append(lines, fmt.Sprintf("%s %s %s %s", record.ID, record.Type, record.Status, record.Description))
				}
				return Result{Message: strings.Join(lines, "\n")}
			}
			if tokens[0] == "show" && len(tokens) == 2 {
				record, ok := manager.GetTask(strings.TrimSpace(tokens[1]))
				if !ok {
					return Result{Message: "No task found with ID: " + strings.TrimSpace(tokens[1])}
				}
				return Result{Message: fmt.Sprintf("Task %s\nType: %s\nStatus: %s\nDescription: %s\nCWD: %s\nCommand: %s", record.ID, record.Type, record.Status, record.Description, record.CWD, record.Command)}
			}
			if tokens[0] == "run" && len(tokens) >= 2 {
				command := strings.TrimSpace(args[len("run "):])
				record := manager.CreateShellTask(command, command, ctx.CWD)
				return Result{Message: "Started task " + record.ID}
			}
			if tokens[0] == "stop" && len(tokens) == 2 {
				record, err := manager.StopTask(strings.TrimSpace(tokens[1]))
				if err != nil {
					return Result{Message: err.Error()}
				}
				return Result{Message: "Stopped task " + record.ID}
			}
			if tokens[0] == "update" && len(tokens) >= 4 {
				taskID := strings.TrimSpace(tokens[1])
				field := strings.TrimSpace(tokens[2])
				value := strings.TrimSpace(strings.Join(tokens[3:], " "))
				if value == "" {
					return Result{Message: "Usage: /tasks update ID [description TEXT|progress NUMBER|note TEXT]"}
				}
				switch field {
				case "description":
					record, err := manager.UpdateTask(taskID, &value, nil, nil)
					if err != nil {
						return Result{Message: err.Error()}
					}
					return Result{Message: "Updated task " + record.ID + " description"}
				case "progress":
					progress, parseErr := strconv.Atoi(value)
					if parseErr != nil {
						return Result{Message: "Progress must be an integer between 0 and 100."}
					}
					record, err := manager.UpdateTask(taskID, nil, &progress, nil)
					if err != nil {
						return Result{Message: err.Error()}
					}
					return Result{Message: fmt.Sprintf("Updated task %s progress to %d%%", record.ID, progress)}
				case "note":
					record, err := manager.UpdateTask(taskID, nil, nil, &value)
					if err != nil {
						return Result{Message: err.Error()}
					}
					return Result{Message: "Updated task " + record.ID + " note"}
				default:
					return Result{Message: "Usage: /tasks update ID [description TEXT|progress NUMBER|note TEXT]"}
				}
			}
			if tokens[0] == "output" && len(tokens) == 2 {
				output, err := manager.ReadTaskOutput(strings.TrimSpace(tokens[1]))
				if err != nil {
					return Result{Message: err.Error()}
				}
				if strings.TrimSpace(output) != "" {
					return Result{Message: output}
				}
				return Result{Message: "(no output)"}
			}
			return Result{Message: "Usage: /tasks [list|run CMD|stop ID|show ID|update ID description TEXT|update ID progress NUMBER|update ID note TEXT|output ID]"}
		},
	})

	registry.Register(Command{
		Name:        "skills",
		Description: "List or show available skills",
		Handler: func(args string, ctx Context) Result {
			registry, err := skills.LoadRegistry(ctx.CWD)
			if err != nil {
				return Result{Message: "Failed to load skills: " + err.Error()}
			}
			tokens := strings.Fields(args)
			if len(tokens) == 0 || tokens[0] == "list" {
				items := registry.List()
				if len(items) == 0 {
					return Result{Message: "No skills available."}
				}
				sort.Slice(items, func(i, j int) bool {
					return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
				})
				lines := []string{"Skills:"}
				for _, item := range items {
					lines = append(lines, fmt.Sprintf("- %s [%s]", item.Name, item.Source))
				}
				return Result{Message: strings.Join(lines, "\n")}
			}
			if tokens[0] == "show" && len(tokens) >= 2 {
				name := strings.Join(tokens[1:], " ")
				item, ok := registry.Get(name)
				if !ok {
					return Result{Message: "Skill not found: " + name}
				}
				return Result{Message: item.Content}
			}
			return Result{Message: "Usage: /skills [list|show NAME]"}
		},
	})

	registry.Register(Command{
		Name:        "memory",
		Description: "Inspect and manage project memory",
		Handler: func(args string, ctx Context) Result {
			memoryDir, err := projectMemoryDir(ctx.CWD)
			if err != nil {
				return Result{Message: "Failed to resolve memory directory: " + err.Error()}
			}
			entrypoint := filepath.Join(memoryDir, "MEMORY.md")
			tokens := strings.Fields(args)
			if len(tokens) == 0 {
				return Result{Message: fmt.Sprintf("Memory directory: %s\nEntrypoint: %s", memoryDir, entrypoint)}
			}
			if tokens[0] == "list" {
				entries, readErr := os.ReadDir(memoryDir)
				if readErr != nil {
					return Result{Message: "No memory files."}
				}
				files := make([]string, 0, len(entries))
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					files = append(files, entry.Name())
				}
				sort.Strings(files)
				if len(files) == 0 {
					return Result{Message: "No memory files."}
				}
				return Result{Message: strings.Join(files, "\n")}
			}
			if tokens[0] == "show" && len(tokens) >= 2 {
				rel := strings.Join(tokens[1:], " ")
				if filepath.IsAbs(rel) || strings.Contains(rel, "..") {
					return Result{Message: "Invalid memory file path."}
				}
				path := filepath.Join(memoryDir, rel)
				if _, statErr := os.Stat(path); statErr != nil && filepath.Ext(path) == "" {
					path += ".md"
				}
				content, readErr := os.ReadFile(path)
				if readErr != nil {
					return Result{Message: "Memory entry not found: " + rel}
				}
				return Result{Message: string(content)}
			}
			if tokens[0] == "add" {
				rest := strings.TrimSpace(strings.TrimPrefix(args, "add"))
				title, content, found := strings.Cut(rest, "::")
				if !found || strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
					return Result{Message: "Usage: /memory add TITLE :: CONTENT"}
				}
				fileName := memorySlug(strings.TrimSpace(title)) + ".md"
				path := filepath.Join(memoryDir, fileName)
				if writeErr := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); writeErr != nil {
					return Result{Message: "Failed to add memory entry: " + writeErr.Error()}
				}
				indexContent := "# Memory Index\n"
				if existing, readErr := os.ReadFile(entrypoint); readErr == nil {
					indexContent = string(existing)
				}
				if !strings.Contains(indexContent, fileName) {
					indexContent = strings.TrimRight(indexContent, "\n") + "\n- [" + strings.TrimSpace(title) + "](" + fileName + ")\n"
					_ = os.WriteFile(entrypoint, []byte(indexContent), 0o644)
				}
				return Result{Message: "Added memory entry " + fileName}
			}
			if tokens[0] == "remove" && len(tokens) >= 2 {
				rel := strings.Join(tokens[1:], " ")
				if filepath.IsAbs(rel) || strings.Contains(rel, "..") {
					return Result{Message: "Invalid memory file path."}
				}
				path := filepath.Join(memoryDir, rel)
				if _, statErr := os.Stat(path); statErr != nil && filepath.Ext(path) == "" {
					path += ".md"
				}
				if _, statErr := os.Stat(path); statErr != nil {
					return Result{Message: "Memory entry not found: " + rel}
				}
				if removeErr := os.Remove(path); removeErr != nil {
					return Result{Message: "Failed to remove memory entry: " + removeErr.Error()}
				}
				if existing, readErr := os.ReadFile(entrypoint); readErr == nil {
					name := filepath.Base(path)
					lines := strings.Split(string(existing), "\n")
					kept := make([]string, 0, len(lines))
					for _, line := range lines {
						if strings.Contains(line, name) {
							continue
						}
						kept = append(kept, line)
					}
					_ = os.WriteFile(entrypoint, []byte(strings.TrimRight(strings.Join(kept, "\n"), "\n")+"\n"), 0o644)
				}
				return Result{Message: "Removed memory entry " + rel}
			}
			return Result{Message: "Usage: /memory [list|show NAME|add TITLE :: CONTENT|remove NAME]"}
		},
	})

	registry.Register(Command{
		Name:        "model",
		Description: "Show or update the default model",
		Handler: func(args string, ctx Context) Result {
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			tokens := strings.Fields(args)
			if len(tokens) == 0 || tokens[0] == "show" {
				return Result{Message: "Model: " + settings.Model}
			}
			if len(tokens) == 2 && tokens[0] == "set" {
				settings.Model = tokens[1]
				if err := config.SaveSettings(settings); err != nil {
					return Result{Message: "Failed to save settings: " + err.Error()}
				}
				ctx.Engine.SetModel(tokens[1])
				return Result{Message: "Model set to " + tokens[1]}
			}
			return Result{Message: "Usage: /model [show|set MODEL]"}
		},
	})

	registry.Register(Command{
		Name:        "theme",
		Description: "Show or update the theme",
		Handler: func(args string, ctx Context) Result {
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			tokens := strings.Fields(args)
			if len(tokens) == 0 || tokens[0] == "show" {
				return Result{Message: "Theme: " + settings.Theme}
			}
			if len(tokens) == 2 && tokens[0] == "set" {
				settings.Theme = tokens[1]
				if err := config.SaveSettings(settings); err != nil {
					return Result{Message: "Failed to save settings: " + err.Error()}
				}
				return Result{Message: "Theme set to " + tokens[1]}
			}
			return Result{Message: "Usage: /theme [show|set THEME]"}
		},
	})

	registry.Register(Command{
		Name:        "output-style",
		Description: "Show or update output style",
		Handler: func(args string, ctx Context) Result {
			_ = ctx
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			tokens := strings.Fields(args)
			if len(tokens) == 0 || tokens[0] == "show" {
				return Result{Message: "Output style: " + settings.OutputStyle}
			}
			if len(tokens) == 2 && tokens[0] == "set" {
				settings.OutputStyle = tokens[1]
				if err := config.SaveSettings(settings); err != nil {
					return Result{Message: "Failed to save settings: " + err.Error()}
				}
				return Result{Message: "Output style set to " + tokens[1]}
			}
			return Result{Message: "Usage: /output-style [show|set NAME]"}
		},
	})

	registry.Register(Command{
		Name:        "permissions",
		Description: "Show or update permission mode",
		Handler: func(args string, ctx Context) Result {
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			tokens := strings.Fields(args)
			if len(tokens) == 0 || tokens[0] == "show" {
				return Result{Message: "Permission mode: " + settings.Permission.Mode}
			}
			if len(tokens) == 2 && tokens[0] == "set" {
				mode := strings.TrimSpace(tokens[1])
				if mode != "default" && mode != "plan" && mode != "full_auto" {
					return Result{Message: "Usage: /permissions [show|set default|plan|full_auto]"}
				}
				settings.Permission.Mode = mode
				if err := config.SaveSettings(settings); err != nil {
					return Result{Message: "Failed to save settings: " + err.Error()}
				}
				ctx.Engine.SetPermissionChecker(commandPermissionChecker{checker: permissions.NewChecker(settings.Permission)})
				return Result{Message: "Permission mode set to " + mode}
			}
			return Result{Message: "Usage: /permissions [show|set default|plan|full_auto]"}
		},
	})

	registry.Register(Command{
		Name:        "plan",
		Description: "Toggle plan permission mode",
		Handler: func(args string, ctx Context) Result {
			action := strings.TrimSpace(args)
			if action == "" {
				return Result{Message: "Usage: /plan [on|off]"}
			}
			if action == "on" {
				return registry.commands["permissions"].Handler("set plan", ctx)
			}
			if action == "off" {
				return registry.commands["permissions"].Handler("set default", ctx)
			}
			return Result{Message: "Usage: /plan [on|off]"}
		},
	})

	registry.Register(Command{
		Name:        "fast",
		Description: "Show or update fast mode",
		Handler: func(args string, ctx Context) Result {
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			action := strings.TrimSpace(args)
			if action == "" || action == "show" {
				if settings.FastMode {
					return Result{Message: "Fast mode: on"}
				}
				return Result{Message: "Fast mode: off"}
			}
			var next bool
			switch action {
			case "on":
				next = true
			case "off":
				next = false
			case "toggle":
				next = !settings.FastMode
			default:
				return Result{Message: "Usage: /fast [show|on|off|toggle]"}
			}
			settings.FastMode = next
			if err := config.SaveSettings(settings); err != nil {
				return Result{Message: "Failed to save settings: " + err.Error()}
			}
			if next {
				return Result{Message: "Fast mode enabled."}
			}
			return Result{Message: "Fast mode disabled."}
		},
	})

	registry.Register(Command{
		Name:        "effort",
		Description: "Show or update reasoning effort",
		Handler: func(args string, ctx Context) Result {
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			tokens := strings.Fields(args)
			if len(tokens) == 0 || tokens[0] == "show" {
				return Result{Message: "Effort: " + settings.Effort}
			}
			if len(tokens) == 2 && tokens[0] == "set" {
				value := strings.TrimSpace(tokens[1])
				if value != "low" && value != "medium" && value != "high" {
					return Result{Message: "Usage: /effort [show|set low|medium|high]"}
				}
				settings.Effort = value
				if err := config.SaveSettings(settings); err != nil {
					return Result{Message: "Failed to save settings: " + err.Error()}
				}
				return Result{Message: "Effort set to " + value}
			}
			return Result{Message: "Usage: /effort [show|set low|medium|high]"}
		},
	})

	registry.Register(Command{
		Name:        "passes",
		Description: "Show or update reasoning pass count",
		Handler: func(args string, ctx Context) Result {
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			tokens := strings.Fields(args)
			if len(tokens) == 0 || tokens[0] == "show" {
				return Result{Message: fmt.Sprintf("Passes: %d", settings.Passes)}
			}
			if len(tokens) == 2 && tokens[0] == "set" {
				value, parseErr := strconv.Atoi(tokens[1])
				if parseErr != nil || value <= 0 {
					return Result{Message: "Usage: /passes [show|set NUMBER]"}
				}
				settings.Passes = value
				if err := config.SaveSettings(settings); err != nil {
					return Result{Message: "Failed to save settings: " + err.Error()}
				}
				return Result{Message: fmt.Sprintf("Passes set to %d", value)}
			}
			return Result{Message: "Usage: /passes [show|set NUMBER]"}
		},
	})

	registry.Register(Command{
		Name:        "vim",
		Description: "Show or update Vim mode",
		Handler: func(args string, ctx Context) Result {
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			action := strings.TrimSpace(args)
			if action == "" || action == "show" {
				if settings.VimMode {
					return Result{Message: "Vim mode: on"}
				}
				return Result{Message: "Vim mode: off"}
			}
			var enabled bool
			switch action {
			case "on":
				enabled = true
			case "off":
				enabled = false
			case "toggle":
				enabled = !settings.VimMode
			default:
				return Result{Message: "Usage: /vim [show|on|off|toggle]"}
			}
			settings.VimMode = enabled
			if err := config.SaveSettings(settings); err != nil {
				return Result{Message: "Failed to save settings: " + err.Error()}
			}
			if enabled {
				return Result{Message: "Vim mode enabled."}
			}
			return Result{Message: "Vim mode disabled."}
		},
	})

	registry.Register(Command{
		Name:        "voice",
		Description: "Show or update voice mode",
		Handler: func(args string, ctx Context) Result {
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			action := strings.TrimSpace(args)
			if action == "" || action == "show" {
				if settings.VoiceMode {
					return Result{Message: "Voice mode: on"}
				}
				return Result{Message: "Voice mode: off"}
			}
			var enabled bool
			switch action {
			case "on":
				enabled = true
			case "off":
				enabled = false
			case "toggle":
				enabled = !settings.VoiceMode
			default:
				return Result{Message: "Usage: /voice [show|on|off|toggle]"}
			}
			settings.VoiceMode = enabled
			if err := config.SaveSettings(settings); err != nil {
				return Result{Message: "Failed to save settings: " + err.Error()}
			}
			if enabled {
				return Result{Message: "Voice mode enabled."}
			}
			return Result{Message: "Voice mode disabled."}
		},
	})

	registry.Register(Command{
		Name:        "hooks",
		Description: "Show configured hooks",
		Handler: func(_ string, ctx Context) Result {
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			pluginHooks, _ := plugins.LoadPluginExtensions(settings, ctx.CWD)
			registry := hooks.LoadRegistry(settings, pluginHooks)
			summary := strings.TrimSpace(registry.Summary())
			if summary == "" {
				return Result{Message: "No hooks configured."}
			}
			return Result{Message: summary}
		},
	})

	registry.Register(Command{
		Name:        "plugin",
		Description: "Manage plugins",
		Handler: func(args string, ctx Context) Result {
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			tokens := strings.Fields(args)
			action := "list"
			if len(tokens) > 0 {
				action = tokens[0]
			}
			switch action {
			case "list":
				items := discoverPluginStatuses(ctx.CWD, settings.EnabledPlugins)
				if len(items) == 0 {
					return Result{Message: "No plugins discovered."}
				}
				lines := []string{"Plugins:"}
				for _, item := range items {
					state := "disabled"
					if item.Enabled {
						state = "enabled"
					}
					lines = append(lines, fmt.Sprintf("- %s [%s]", item.Name, state))
				}
				return Result{Message: strings.Join(lines, "\n")}
			case "enable":
				if len(tokens) != 2 {
					return Result{Message: "Usage: /plugin [list|enable NAME|disable NAME]"}
				}
				if settings.EnabledPlugins == nil {
					settings.EnabledPlugins = map[string]bool{}
				}
				settings.EnabledPlugins[strings.TrimSpace(tokens[1])] = true
				if err := config.SaveSettings(settings); err != nil {
					return Result{Message: "Failed to save settings: " + err.Error()}
				}
				return Result{Message: "Enabled plugin " + strings.TrimSpace(tokens[1])}
			case "disable":
				if len(tokens) != 2 {
					return Result{Message: "Usage: /plugin [list|enable NAME|disable NAME]"}
				}
				if settings.EnabledPlugins == nil {
					settings.EnabledPlugins = map[string]bool{}
				}
				settings.EnabledPlugins[strings.TrimSpace(tokens[1])] = false
				if err := config.SaveSettings(settings); err != nil {
					return Result{Message: "Failed to save settings: " + err.Error()}
				}
				return Result{Message: "Disabled plugin " + strings.TrimSpace(tokens[1])}
			default:
				return Result{Message: "Usage: /plugin [list|enable NAME|disable NAME]"}
			}
		},
	})

	registry.Register(Command{
		Name:        "reload-plugins",
		Description: "Reload plugin discovery for this workspace",
		Handler: func(_ string, ctx Context) Result {
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			items := discoverPluginStatuses(ctx.CWD, settings.EnabledPlugins)
			if len(items) == 0 {
				return Result{Message: "No plugins discovered."}
			}
			lines := []string{"Reloaded plugins:"}
			for _, item := range items {
				state := "disabled"
				if item.Enabled {
					state = "enabled"
				}
				lines = append(lines, fmt.Sprintf("- %s [%s]", item.Name, state))
			}
			return Result{Message: strings.Join(lines, "\n")}
		},
	})

	registry.Register(Command{
		Name:        "agents",
		Description: "List or inspect agent and teammate tasks",
		Handler: func(args string, ctx Context) Result {
			manager := tasks.DefaultManager()
			tokens := strings.Fields(args)
			if len(tokens) >= 2 && tokens[0] == "show" {
				record, ok := manager.GetTask(strings.TrimSpace(tokens[1]))
				if !ok || (record.Type != tasks.TaskTypeLocalAgent) {
					return Result{Message: "No agent found with ID: " + strings.TrimSpace(tokens[1])}
				}
				output, _ := manager.ReadTaskOutput(record.ID)
				if strings.TrimSpace(output) == "" {
					output = "(no output)"
				}
				return Result{Message: fmt.Sprintf("%s %s %s %s\nmetadata=%v\noutput:\n%s", record.ID, record.Type, record.Status, record.Description, record.Metadata, output)}
			}
			records := manager.ListTasks("")
			lines := []string{}
			for _, record := range records {
				if record.Type != tasks.TaskTypeLocalAgent {
					continue
				}
				lines = append(lines, fmt.Sprintf("%s %s %s %s", record.ID, record.Type, record.Status, record.Description))
			}
			if len(lines) == 0 {
				return Result{Message: "No active or recorded agents."}
			}
			return Result{Message: strings.Join(lines, "\n")}
		},
	})

	registry.Register(Command{
		Name:        "bridge",
		Description: "Inspect bridge helpers and sessions",
		Handler: func(args string, ctx Context) Result {
			manager := bridge.DefaultManager()
			tokens := strings.Fields(args)
			if len(tokens) == 0 || tokens[0] == "show" {
				sessions := manager.ListSnapshots()
				return Result{Message: strings.Join([]string{
					"Bridge summary:",
					"- backend host: available",
					fmt.Sprintf("- cwd: %s", ctx.CWD),
					fmt.Sprintf("- sessions: %d", len(sessions)),
					"- utilities: show, encode, decode, sdk, spawn, list, output, stop",
				}, "\n")}
			}
			if tokens[0] == "encode" && len(tokens) == 3 {
				raw, err := bridge.EncodeWorkSecret(bridge.WorkSecret{Version: 1, APIBaseURL: strings.TrimSpace(tokens[1]), SessionIngressToken: strings.TrimSpace(tokens[2])})
				if err != nil {
					return Result{Message: err.Error()}
				}
				return Result{Message: raw}
			}
			if tokens[0] == "decode" && len(tokens) == 2 {
				secret, err := bridge.DecodeWorkSecret(strings.TrimSpace(tokens[1]))
				if err != nil {
					return Result{Message: err.Error()}
				}
				payload, _ := json.MarshalIndent(secret, "", "  ")
				return Result{Message: string(payload)}
			}
			if tokens[0] == "sdk" && len(tokens) == 3 {
				return Result{Message: bridge.BuildSDKURL(strings.TrimSpace(tokens[1]), strings.TrimSpace(tokens[2]))}
			}
			if tokens[0] == "list" {
				sessions := manager.ListSnapshots()
				if len(sessions) == 0 {
					return Result{Message: "No bridge sessions."}
				}
				lines := make([]string, 0, len(sessions))
				for _, s := range sessions {
					lines = append(lines, fmt.Sprintf("%s [%s] pid=%d %s", s.SessionID, s.Status, s.PID, s.Command))
				}
				return Result{Message: strings.Join(lines, "\n")}
			}
			if tokens[0] == "spawn" && len(tokens) >= 2 {
				command := strings.TrimSpace(args[len("spawn "):])
				session, err := manager.Spawn(command, ctx.CWD)
				if err != nil {
					return Result{Message: err.Error()}
				}
				return Result{Message: fmt.Sprintf("Spawned bridge session %s pid=%d", session.SessionID, session.PID)}
			}
			if tokens[0] == "output" && len(tokens) == 2 {
				out, err := manager.ReadOutput(strings.TrimSpace(tokens[1]))
				if err != nil {
					return Result{Message: err.Error()}
				}
				if strings.TrimSpace(out) == "" {
					return Result{Message: "(no output)"}
				}
				return Result{Message: out}
			}
			if tokens[0] == "stop" && len(tokens) == 2 {
				if err := manager.Stop(strings.TrimSpace(tokens[1])); err != nil {
					return Result{Message: err.Error()}
				}
				return Result{Message: "Stopped bridge session " + strings.TrimSpace(tokens[1])}
			}
			return Result{Message: "Usage: /bridge [show|encode API_BASE_URL TOKEN|decode SECRET|sdk API_BASE_URL SESSION_ID|spawn CMD|list|output SESSION_ID|stop SESSION_ID]"}
		},
	})

	registry.Register(Command{
		Name:        "mcp",
		Description: "Show MCP status",
		Handler: func(_ string, ctx Context) Result {
			if ctx.MCPManager == nil {
				settings, err := config.LoadSettings()
				if err != nil {
					return Result{Message: "Failed to load settings: " + err.Error()}
				}
				if len(settings.MCPServers) == 0 {
					return Result{Message: "No MCP servers configured."}
				}
				return Result{Message: fmt.Sprintf("Configured MCP servers: %d", len(settings.MCPServers))}
			}
			statuses := ctx.MCPManager.ListStatuses()
			if len(statuses) == 0 {
				return Result{Message: "No MCP servers configured."}
			}
			lines := make([]string, 0, len(statuses)+1)
			lines = append(lines, "MCP servers:")
			for _, s := range statuses {
				detail := strings.TrimSpace(s.Detail)
				if detail != "" {
					lines = append(lines, fmt.Sprintf("- %s: %s (%s)", s.Name, s.State, detail))
					continue
				}
				lines = append(lines, fmt.Sprintf("- %s: %s", s.Name, s.State))
			}
			return Result{Message: strings.Join(lines, "\n")}
		},
	})

	registry.Register(Command{
		Name:        "doctor",
		Description: "Show environment diagnostics",
		Handler: func(_ string, ctx Context) Result {
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			provider := "anthropic-compatible"
			if strings.TrimSpace(settings.BaseURL) != "" {
				provider = "custom-compatible"
			}
			return Result{Message: strings.Join([]string{
				"Doctor summary:",
				"- cwd: " + ctx.CWD,
				"- model: " + settings.Model,
				"- permission_mode: " + settings.Permission.Mode,
				"- theme: " + settings.Theme,
				"- output_style: " + settings.OutputStyle,
				"- vim_mode: " + onOff(settings.VimMode),
				"- voice_mode: " + onOff(settings.VoiceMode),
				"- fast_mode: " + onOff(settings.FastMode),
				"- effort: " + settings.Effort,
				fmt.Sprintf("- passes: %d", settings.Passes),
				"- provider: " + provider,
			}, "\n")}
		},
	})

	registry.Register(Command{
		Name:        "privacy-settings",
		Description: "Show local privacy and storage settings",
		Handler: func(_ string, ctx Context) Result {
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			configDir, _ := config.ConfigDir()
			dataDir, _ := config.DataDir()
			sessionDir, _ := services.ProjectSessionDir(ctx.CWD)
			projectConfigDir := filepath.Join(ctx.CWD, ".openharness")
			lines := []string{
				"Privacy settings:",
				"- user_config_dir: " + configDir,
				"- project_config_dir: " + projectConfigDir,
				"- data_dir: " + dataDir,
				"- session_dir: " + sessionDir,
				"- api_base_url: " + settings.BaseURL,
				"- network: enabled only for provider and explicit web/MCP calls",
				"- storage: local files under ~/.openharness and project .openharness",
			}
			return Result{Message: strings.Join(lines, "\n")}
		},
	})

	registry.Register(Command{
		Name:        "login",
		Description: "Show auth status or store an API key",
		Handler: func(args string, ctx Context) Result {
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			value := strings.TrimSpace(args)
			if value == "" || value == "show" {
				provider := api.DetectProvider(settings.Model, settings.BaseURL)
				masked := "(not configured)"
				if strings.TrimSpace(settings.APIKey) != "" {
					key := strings.TrimSpace(settings.APIKey)
					if len(key) > 10 {
						masked = key[:6] + "..." + key[len(key)-4:]
					} else {
						masked = key
					}
				}
				return Result{Message: strings.Join([]string{
					"Auth status:",
					"- provider: " + provider.Name,
					"- auth_status: " + api.AuthStatus(settings.APIKey),
					"- base_url: " + defaultIfEmpty(settings.BaseURL, "(default)"),
					"- model: " + settings.Model,
					"- api_key: " + masked,
					"Usage: /login API_KEY",
				}, "\n")}
			}
			settings.APIKey = value
			if err := config.SaveSettings(settings); err != nil {
				return Result{Message: "Failed to save settings: " + err.Error()}
			}
			return Result{Message: "Stored API key in ~/.openharness/settings.json"}
		},
	})

	registry.Register(Command{
		Name:        "logout",
		Description: "Clear the stored API key",
		Handler: func(_ string, ctx Context) Result {
			_ = ctx
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			settings.APIKey = ""
			if err := config.SaveSettings(settings); err != nil {
				return Result{Message: "Failed to save settings: " + err.Error()}
			}
			return Result{Message: "API key cleared."}
		},
	})

	registry.Register(Command{
		Name:        "rate-limit-options",
		Description: "Show ways to reduce provider rate pressure",
		Handler: func(_ string, ctx Context) Result {
			_ = ctx
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			provider := "anthropic-compatible"
			if strings.Contains(strings.ToLower(settings.BaseURL), "moonshot") {
				provider = "moonshot-compatible"
			}
			lines := []string{
				"Rate limit options:",
				"- provider: " + provider,
				"- reduce /passes or switch /effort low for lighter requests",
				"- enable /fast for shorter responses and less tool churn",
				"- use /compact to shrink long transcripts before retrying",
				"- prefer background /tasks for long-running local work",
			}
			return Result{Message: strings.Join(lines, "\n")}
		},
	})

	registry.Register(Command{
		Name:        "upgrade",
		Description: "Show upgrade instructions",
		Handler: func(_ string, ctx Context) Result {
			_ = ctx
			return Result{Message: strings.Join([]string{
				"Current version: OpenHarness go",
				"Upgrade instructions:",
				"- uv sync --extra dev",
				"- uv pip install -e .",
				"- npm --prefix frontend/terminal install",
			}, "\n")}
		},
	})

	registry.Register(Command{
		Name:        "release-notes",
		Description: "Show recent OpenHarness release notes",
		Handler: func(_ string, ctx Context) Result {
			path := filepath.Join(ctx.CWD, "RELEASE_NOTES.md")
			if content, err := os.ReadFile(path); err == nil {
				return Result{Message: string(content)}
			}
			return Result{Message: strings.Join([]string{
				"# Release Notes",
				"",
				"- React TUI is now the default oh interface.",
				"- Continued Python to Go backend migration for commands and runtime wiring.",
			}, "\n")}
		},
	})

	registry.Register(Command{
		Name:        "keybindings",
		Description: "Show resolved keybindings",
		Handler: func(_ string, ctx Context) Result {
			_ = ctx
			path, bindings, err := loadKeybindings()
			if err != nil {
				return Result{Message: "Failed to load keybindings: " + err.Error()}
			}
			keys := make([]string, 0, len(bindings))
			for key := range bindings {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			lines := []string{"Keybindings file: " + path}
			for _, key := range keys {
				lines = append(lines, key+" -> "+bindings[key])
			}
			return Result{Message: strings.Join(lines, "\n")}
		},
	})

	registry.Register(Command{
		Name:        "init",
		Description: "Initialize project OpenHarness files",
		Handler: func(_ string, ctx Context) Result {
			created := []string{}
			claudePath := filepath.Join(ctx.CWD, "CLAUDE.md")
			if _, err := os.Stat(claudePath); err != nil {
				content := strings.Join([]string{
					"# Project Instructions",
					"",
					"- Use OpenHarness tools deliberately.",
					"- Keep changes minimal and verify with tests when possible.",
				}, "\n") + "\n"
				if writeErr := os.WriteFile(claudePath, []byte(content), 0o644); writeErr == nil {
					created = append(created, "CLAUDE.md")
				}
			}

			projectDir := filepath.Join(ctx.CWD, ".openharness")
			items := []struct {
				relPath string
				content string
			}{
				{relPath: filepath.Join(".openharness", "README.md"), content: "# Project OpenHarness Config\n\nThis directory stores project-specific OpenHarness state.\n"},
				{relPath: filepath.Join(".openharness", "memory", "MEMORY.md"), content: "# Project Memory\n\nAdd reusable project knowledge here.\n"},
				{relPath: filepath.Join(".openharness", "plugins", ".gitkeep"), content: ""},
				{relPath: filepath.Join(".openharness", "skills", ".gitkeep"), content: ""},
			}
			for _, item := range items {
				full := filepath.Join(ctx.CWD, item.relPath)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					continue
				}
				if _, err := os.Stat(full); err == nil {
					continue
				}
				if writeErr := os.WriteFile(full, []byte(item.content), 0o644); writeErr == nil {
					rel, relErr := filepath.Rel(ctx.CWD, full)
					if relErr != nil {
						rel = full
					}
					created = append(created, rel)
				}
			}
			_ = projectDir
			if len(created) == 0 {
				return Result{Message: "Project already initialized for OpenHarness."}
			}
			return Result{Message: "Initialized project files:\n- " + strings.Join(created, "\n- ")}
		},
	})

	registry.Register(Command{
		Name:        "feedback",
		Description: "Save CLI feedback to the local feedback log",
		Handler: func(args string, ctx Context) Result {
			_ = ctx
			dataDir, err := config.DataDir()
			if err != nil {
				return Result{Message: "Failed to resolve feedback log path: " + err.Error()}
			}
			path := filepath.Join(dataDir, "feedback.log")
			if strings.TrimSpace(args) == "" {
				return Result{Message: "Feedback log: " + path + "\nUsage: /feedback TEXT"}
			}
			line := "[" + time.Now().UTC().Format(time.RFC3339) + "] " + strings.TrimSpace(args) + "\n"
			f, openErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if openErr != nil {
				return Result{Message: "Failed to open feedback log: " + openErr.Error()}
			}
			defer f.Close()
			if _, writeErr := f.WriteString(line); writeErr != nil {
				return Result{Message: "Failed to write feedback log: " + writeErr.Error()}
			}
			return Result{Message: "Saved feedback to " + path}
		},
	})

	registry.Register(Command{
		Name:        "onboarding",
		Description: "Show the quickstart guide",
		Handler: func(_ string, ctx Context) Result {
			_ = ctx
			return Result{Message: strings.Join([]string{
				"OpenHarness quickstart:",
				"1. Ask for a coding task in plain language.",
				"2. Use /help to inspect commands.",
				"3. Use /doctor to inspect runtime state.",
				"4. Use /tasks for background work and /memory for project memory.",
				"5. Use /login to store an API key if needed.",
			}, "\n")}
		},
	})

	registry.Register(Command{
		Name:        "config",
		Description: "Show or update configuration",
		Handler: func(args string, ctx Context) Result {
			_ = ctx
			settings, err := config.LoadSettings()
			if err != nil {
				return Result{Message: "Failed to load settings: " + err.Error()}
			}
			tokens := strings.Fields(args)
			if len(tokens) == 0 || tokens[0] == "show" {
				payload, err := json.MarshalIndent(settings, "", "  ")
				if err != nil {
					return Result{Message: "Failed to render config: " + err.Error()}
				}
				return Result{Message: string(payload)}
			}
			if len(tokens) >= 3 && tokens[0] == "set" {
				key := tokens[1]
				value := strings.Join(tokens[2:], " ")
				switch key {
				case "model":
					settings.Model = value
				case "max_tokens":
					n, parseErr := strconv.Atoi(value)
					if parseErr != nil {
						return Result{Message: "max_tokens must be an integer"}
					}
					settings.MaxTokens = n
				case "base_url":
					settings.BaseURL = value
				case "system_prompt":
					settings.SystemPrompt = value
				case "theme":
					settings.Theme = value
				case "output_style":
					settings.OutputStyle = value
				case "api_key":
					settings.APIKey = value
				case "vim_mode":
					b, ok := parseBool(value)
					if !ok {
						return Result{Message: "vim_mode must be true/false"}
					}
					settings.VimMode = b
				case "voice_mode":
					b, ok := parseBool(value)
					if !ok {
						return Result{Message: "voice_mode must be true/false"}
					}
					settings.VoiceMode = b
				case "fast_mode":
					b, ok := parseBool(value)
					if !ok {
						return Result{Message: "fast_mode must be true/false"}
					}
					settings.FastMode = b
				case "effort":
					settings.Effort = value
				case "passes":
					n, parseErr := strconv.Atoi(value)
					if parseErr != nil {
						return Result{Message: "passes must be an integer"}
					}
					settings.Passes = n
				case "verbose":
					b, ok := parseBool(value)
					if !ok {
						return Result{Message: "verbose must be true/false"}
					}
					settings.Verbose = b
				case "permission_mode":
					if value != "default" && value != "plan" && value != "full_auto" {
						return Result{Message: "permission_mode must be default|plan|full_auto"}
					}
					settings.Permission.Mode = value
				default:
					return Result{Message: "Unknown config key: " + key}
				}
				if err := config.SaveSettings(settings); err != nil {
					return Result{Message: "Failed to save settings: " + err.Error()}
				}
				return Result{Message: "Updated config " + key}
			}
			return Result{Message: "Usage: /config [show|set KEY VALUE]"}
		},
	})

	registry.Register(Command{
		Name:        "diff",
		Description: "Show git diff output",
		Handler: func(args string, ctx Context) Result {
			if strings.TrimSpace(args) == "full" {
				ok, out := runGitCommand(ctx.CWD, "diff", "HEAD")
				if ok {
					if strings.TrimSpace(out) == "" {
						return Result{Message: "(no diff)"}
					}
					return Result{Message: out}
				}
				return Result{Message: out}
			}
			ok, out := runGitCommand(ctx.CWD, "diff", "--stat")
			if ok {
				if strings.TrimSpace(out) == "" {
					return Result{Message: "(no diff)"}
				}
				return Result{Message: out}
			}
			return Result{Message: out}
		},
	})

	registry.Register(Command{
		Name:        "commit",
		Description: "Show status or create a git commit",
		Handler: func(args string, ctx Context) Result {
			message := strings.TrimSpace(args)
			if message == "" {
				ok, out := runGitCommand(ctx.CWD, "status", "--short")
				if !ok {
					return Result{Message: out}
				}
				if strings.TrimSpace(out) == "" {
					return Result{Message: "(working tree clean)"}
				}
				return Result{Message: out}
			}
			ok, status := runGitCommand(ctx.CWD, "status", "--short")
			if !ok {
				return Result{Message: status}
			}
			if strings.TrimSpace(status) == "" {
				return Result{Message: "Nothing to commit."}
			}
			if ok, out := runGitCommand(ctx.CWD, "add", "-A"); !ok {
				return Result{Message: out}
			}
			_, out := runGitCommand(ctx.CWD, "commit", "-m", message)
			return Result{Message: out}
		},
	})

	registry.Register(Command{
		Name:        "issue",
		Description: "Show or update project issue context",
		Handler: func(args string, ctx Context) Result {
			path := filepath.Join(ctx.CWD, ".openharness", "issue.md")
			tokens := strings.Fields(args)
			action := "show"
			if len(tokens) > 0 {
				action = tokens[0]
			}
			switch action {
			case "show":
				content, err := os.ReadFile(path)
				if err != nil {
					return Result{Message: "No issue context. File path: " + path}
				}
				return Result{Message: string(content)}
			case "set":
				rest := strings.TrimSpace(strings.TrimPrefix(args, "set"))
				title, body, found := strings.Cut(rest, "::")
				if !found || strings.TrimSpace(title) == "" || strings.TrimSpace(body) == "" {
					return Result{Message: "Usage: /issue set TITLE :: BODY"}
				}
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return Result{Message: "Failed to write issue context: " + err.Error()}
				}
				content := "# " + strings.TrimSpace(title) + "\n\n" + strings.TrimSpace(body) + "\n"
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					return Result{Message: "Failed to write issue context: " + err.Error()}
				}
				return Result{Message: "Saved issue context to " + path}
			case "clear":
				if _, err := os.Stat(path); err == nil {
					_ = os.Remove(path)
					return Result{Message: "Cleared issue context."}
				}
				return Result{Message: "No issue context to clear."}
			default:
				return Result{Message: "Usage: /issue [show|set TITLE :: BODY|clear]"}
			}
		},
	})

	registry.Register(Command{
		Name:        "pr_comments",
		Description: "Show or update project PR comments context",
		Handler: func(args string, ctx Context) Result {
			path := filepath.Join(ctx.CWD, ".openharness", "pr_comments.md")
			tokens := strings.Fields(args)
			action := "show"
			if len(tokens) > 0 {
				action = tokens[0]
			}
			switch action {
			case "show":
				content, err := os.ReadFile(path)
				if err != nil {
					return Result{Message: "No PR comments context. File path: " + path}
				}
				return Result{Message: string(content)}
			case "add":
				rest := strings.TrimSpace(strings.TrimPrefix(args, "add"))
				location, comment, found := strings.Cut(rest, "::")
				if !found || strings.TrimSpace(location) == "" || strings.TrimSpace(comment) == "" {
					return Result{Message: "Usage: /pr_comments add FILE[:LINE] :: COMMENT"}
				}
				existing := "# PR Comments\n"
				if content, err := os.ReadFile(path); err == nil {
					existing = string(content)
					if !strings.HasSuffix(existing, "\n") {
						existing += "\n"
					}
				}
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return Result{Message: "Failed to update PR comments: " + err.Error()}
				}
				existing += "- " + strings.TrimSpace(location) + ": " + strings.TrimSpace(comment) + "\n"
				if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
					return Result{Message: "Failed to update PR comments: " + err.Error()}
				}
				return Result{Message: "Added PR comment to " + path}
			case "clear":
				if _, err := os.Stat(path); err == nil {
					_ = os.Remove(path)
					return Result{Message: "Cleared PR comments context."}
				}
				return Result{Message: "No PR comments context to clear."}
			default:
				return Result{Message: "Usage: /pr_comments [show|add FILE[:LINE] :: COMMENT|clear]"}
			}
		},
	})

	registry.Register(Command{
		Name:        "branch",
		Description: "Show git branch information",
		Handler: func(args string, ctx Context) Result {
			action := strings.TrimSpace(args)
			if action == "" || action == "show" {
				ok, out := runGitCommand(ctx.CWD, "branch", "--show-current")
				if !ok {
					return Result{Message: out}
				}
				if strings.TrimSpace(out) == "" {
					return Result{Message: "Current branch: (detached HEAD)"}
				}
				return Result{Message: "Current branch: " + strings.TrimSpace(out)}
			}
			if action == "list" {
				_, out := runGitCommand(ctx.CWD, "branch", "--format", "%(refname:short)")
				if strings.TrimSpace(out) == "" {
					return Result{Message: "(no branches)"}
				}
				return Result{Message: out}
			}
			return Result{Message: "Usage: /branch [show|list]"}
		},
	})

	registry.Register(Command{
		Name:        "exit",
		Description: "Exit current session",
		Handler: func(_ string, ctx Context) Result {
			_ = ctx
			return Result{ShouldExit: true}
		},
	})

	return registry
}

func summarizeMessages(messages []engine.ConversationMessage, maxMessages int) string {
	if len(messages) == 0 {
		return "No conversation content to summarize."
	}
	start := 0
	if maxMessages > 0 && len(messages) > maxMessages {
		start = len(messages) - maxMessages
	}
	lines := []string{"Conversation summary:"}
	for _, m := range messages[start:] {
		text := strings.TrimSpace(m.Text)
		if text == "" {
			if len(m.ToolUses) > 0 {
				text = fmt.Sprintf("%d tool call(s)", len(m.ToolUses))
			} else if len(m.ToolResults) > 0 {
				text = fmt.Sprintf("%d tool result(s)", len(m.ToolResults))
			} else {
				continue
			}
		}
		if len(text) > 120 {
			text = text[:120] + "..."
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", m.Role, text))
	}
	if len(lines) == 1 {
		return "No conversation content to summarize."
	}
	return strings.Join(lines, "\n")
}

func estimateConversationTokens(messages []engine.ConversationMessage) int {
	chars := 0
	for _, m := range messages {
		chars += len(m.Text)
		for _, tc := range m.ToolUses {
			chars += len(tc.Name) + len(tc.ID)
		}
		for _, tr := range m.ToolResults {
			chars += len(tr.ToolUseID) + len(tr.Content)
		}
	}
	if chars == 0 {
		return 0
	}
	return int(math.Ceil(float64(chars) / 4.0))
}

func runGitCommand(cwd string, args ...string) (bool, string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			return false, err.Error()
		}
		return false, text
	}
	return true, text
}

func onOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func projectMemoryDir(cwd string) (string, error) {
	dataDir, err := config.DataDir()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.Abs(cwd)
	if err != nil {
		resolved = cwd
	}
	digestRaw := sha1.Sum([]byte(resolved))
	digest := hex.EncodeToString(digestRaw[:])[:12]
	dir := filepath.Join(dataDir, "memory", filepath.Base(resolved)+"-"+digest)
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return "", mkErr
	}
	return dir, nil
}

func memorySlug(title string) string {
	trimmed := strings.TrimSpace(strings.ToLower(title))
	if trimmed == "" {
		return "memory"
	}
	b := strings.Builder{}
	lastUnderscore := false
	for _, r := range trimmed {
		isAlpha := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if isAlpha || isDigit {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteRune('_')
			lastUnderscore = true
		}
	}
	slug := strings.Trim(b.String(), "_")
	if slug == "" {
		return "memory"
	}
	return slug
}

func lastMessageText(messages []engine.ConversationMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(messages[i].Text); t != "" {
			return t
		}
	}
	return ""
}

func exportSessionMarkdown(cwd string, messages []engine.ConversationMessage) (string, error) {
	exportDir := filepath.Join(cwd, ".openharness", "exports")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		return "", err
	}
	name := "session-" + time.Now().Format("20060102-150405") + ".md"
	path := filepath.Join(exportDir, name)
	lines := []string{"# OpenHarness Session", ""}
	for _, msg := range messages {
		text := strings.TrimSpace(msg.Text)
		if text == "" {
			if len(msg.ToolUses) > 0 {
				lines = append(lines, "## "+titleRole(msg.Role), "(tool call message)", "")
			} else if len(msg.ToolResults) > 0 {
				lines = append(lines, "## "+titleRole(msg.Role), "(tool result message)", "")
			}
			continue
		}
		lines = append(lines, "## "+titleRole(msg.Role), text, "")
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func titleRole(role string) string {
	if role == "" {
		return "Message"
	}
	return strings.ToUpper(role[:1]) + role[1:]
}

func rewindTurns(messages []engine.ConversationMessage, turns int) []engine.ConversationMessage {
	updated := append([]engine.ConversationMessage{}, messages...)
	for i := 0; i < turns && len(updated) > 0; i++ {
		for len(updated) > 0 {
			last := updated[len(updated)-1]
			updated = updated[:len(updated)-1]
			if last.Role == "user" && strings.TrimSpace(last.Text) != "" {
				break
			}
		}
	}
	return updated
}

func safeTagName(name string) string {
	s := memorySlug(name)
	if s == "" {
		return ""
	}
	return s
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func loadKeybindings() (string, map[string]string, error) {
	cfgDir, err := config.ConfigDir()
	if err != nil {
		return "", nil, err
	}
	path := filepath.Join(cfgDir, "keybindings.json")
	bindings := map[string]string{
		"ctrl+l": "clear",
		"ctrl+k": "toggle_vim",
		"ctrl+v": "toggle_voice",
		"ctrl+t": "tasks",
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return path, bindings, nil
		}
		return "", nil, readErr
	}
	var overrides map[string]string
	if err := json.Unmarshal(data, &overrides); err != nil {
		return "", nil, err
	}
	for key, value := range overrides {
		bindings[key] = value
	}
	return path, bindings, nil
}

type commandPermissionChecker struct {
	checker *permissions.Checker
}

func (a commandPermissionChecker) Evaluate(toolName string, isReadOnly bool, filePath, command string) engine.PermissionDecision {
	d := a.checker.Evaluate(toolName, isReadOnly, filePath, command)
	return engine.PermissionDecision{Allowed: d.Allowed, RequiresConfirmation: d.RequiresConfirmation, Reason: d.Reason}
}

type pluginStatus struct {
	Name    string
	Enabled bool
}

type pluginManifestLite struct {
	Name             string `json:"name"`
	EnabledByDefault bool   `json:"enabled_by_default"`
}

func discoverPluginStatuses(cwd string, enabledPlugins map[string]bool) []pluginStatus {
	roots := []string{}
	if cfgDir, err := config.ConfigDir(); err == nil {
		roots = append(roots, filepath.Join(cfgDir, "plugins"))
	}
	roots = append(roots, filepath.Join(cwd, ".openharness", "plugins"))

	seen := map[string]bool{}
	items := []pluginStatus{}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pluginDir := filepath.Join(root, entry.Name())
			manifestPath := ""
			for _, p := range []string{filepath.Join(pluginDir, "plugin.json"), filepath.Join(pluginDir, ".claude-plugin", "plugin.json")} {
				if _, err := os.Stat(p); err == nil {
					manifestPath = p
					break
				}
			}
			if manifestPath == "" {
				continue
			}
			data, err := os.ReadFile(manifestPath)
			if err != nil {
				continue
			}
			m := pluginManifestLite{}
			if err := json.Unmarshal(data, &m); err != nil || strings.TrimSpace(m.Name) == "" {
				continue
			}
			if seen[m.Name] {
				continue
			}
			enabled, ok := enabledPlugins[m.Name]
			if !ok {
				enabled = m.EnabledByDefault
			}
			items = append(items, pluginStatus{Name: m.Name, Enabled: enabled})
			seen[m.Name] = true
		}
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) })
	return items
}

func defaultIfEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func parseBool(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}
