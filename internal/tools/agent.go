package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/staticlock/GoHarness/internal/coordinator"
	"github.com/staticlock/GoHarness/internal/tasks"
)

type AgentTool struct {
	Manager *tasks.Manager
}

func (t *AgentTool) Name() string        { return "agent" }
func (t *AgentTool) Description() string { return "Spawn a local background agent task." }
func (t *AgentTool) IsReadOnly() bool    { return false }

func (t *AgentTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Short description of the delegated work",
			},
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "Full prompt for the local agent",
			},
			"model": map[string]interface{}{
				"type":        "string",
				"description": "Optional model override",
			},
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Override spawn command",
			},
			"team": map[string]interface{}{
				"type":        "string",
				"description": "Optional team to attach the agent to",
			},
			"mode": map[string]interface{}{
				"type":        "string",
				"description": "Agent mode: local_agent, remote_agent, or in_process_teammate",
				"enum":        []string{"local_agent", "remote_agent", "in_process_teammate"},
			},
		},
		"required": []string{"description", "prompt"},
	}
}

type agentInput struct {
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Model       string `json:"model,omitempty"`
	Command     string `json:"command,omitempty"`
	Team        string `json:"team,omitempty"`
	Mode        string `json:"mode,omitempty"`
}

func (t *AgentTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input agentInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	if input.Mode == "" {
		input.Mode = "local_agent"
	}
	if input.Mode != "local_agent" && input.Mode != "remote_agent" && input.Mode != "in_process_teammate" {
		return ToolResult{Output: "Invalid mode. Use local_agent, remote_agent, or in_process_teammate.", IsError: true}, nil
	}

	cmd := input.Command
	createPrompt := input.Prompt

	if input.Mode == "local_agent" {
		if cmd == "" {
			execPath, err := os.Executable()
			if err != nil {
				return NewErrorResult(fmt.Errorf("failed to find executable: %w", err)), nil
			}
			apiKey := os.Getenv("ANTHROPIC_API_KEY")
			if apiKey == "" {
				return ToolResult{Output: "ANTHROPIC_API_KEY environment variable not set", IsError: true}, nil
			}
			cmd = fmt.Sprintf("%s run -p '%s'", execPath, input.Prompt)
			if input.Model != "" {
				cmd += fmt.Sprintf(" --model %s", input.Model)
			}
		}

		record := t.Manager.CreateAgentTask(input.Prompt, input.Description, execCtx.CWD, input.Model)
		record.Command = cmd
		t.Manager.UpdateTaskRecord(record)

		go func() {
			execCmd := exec.Command("bash", "-lc", cmd)
			execCmd.Dir = execCtx.CWD
			execCmd.Env = os.Environ()
			if output, err := execCmd.CombinedOutput(); err != nil {
				t.Manager.UpdateTaskStatus(record.ID, "failed")
				t.Manager.UpdateTaskOutput(record.ID, string(output))
			} else {
				t.Manager.UpdateTaskStatus(record.ID, "completed")
				t.Manager.UpdateTaskOutput(record.ID, string(output))
			}
		}()

		if input.Team != "" {
			registry := coordinator.GetTeamRegistry()
			if err := registry.AddAgent(input.Team, record.ID); err != nil {
				return ToolResult{Output: fmt.Sprintf("Spawned %s task %s but failed to add to team: %v", input.Mode, record.ID, err), IsError: false}, nil
			}
		}

		return NewSuccessResult(fmt.Sprintf("Spawned %s task %s", input.Mode, record.ID)), nil
	}

	if input.Mode == "remote_agent" {
		record := t.Manager.CreateAgentTask(createPrompt, input.Description, execCtx.CWD, input.Model)
		record.Command = "remote:" + input.Prompt[:50]
		t.Manager.UpdateTaskRecord(record)
		t.Manager.UpdateTaskMetadata(record.ID, "agent_mode", "remote_agent")

		go func() {
			t.Manager.UpdateTaskStatus(record.ID, "running")
		}()

		if input.Team != "" {
			registry := coordinator.GetTeamRegistry()
			registry.AddAgent(input.Team, record.ID)
		}

		return NewSuccessResult(fmt.Sprintf("Spawned %s task %s (remote agent needs external bridge)", input.Mode, record.ID)), nil
	}

	if input.Mode == "in_process_teammate" {
		record := t.Manager.CreateAgentTask(createPrompt, input.Description, execCtx.CWD, input.Model)
		record.Command = "in_process:" + createPrompt[:50]
		t.Manager.UpdateTaskRecord(record)
		t.Manager.UpdateTaskMetadata(record.ID, "agent_mode", "in_process_teammate")

		go func() {
			t.Manager.UpdateTaskStatus(record.ID, "running")
		}()

		if input.Team != "" {
			registry := coordinator.GetTeamRegistry()
			registry.AddAgent(input.Team, record.ID)
		}

		return NewSuccessResult(fmt.Sprintf("Spawned %s task %s (in-process teammate)", input.Mode, record.ID)), nil
	}

	return NewSuccessResult(fmt.Sprintf("Spawned %s task", input.Mode)), nil
}
