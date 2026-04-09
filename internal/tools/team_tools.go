package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/staticlock/GoHarness/internal/coordinator"
)

type TeamCreateTool struct{}

func (t *TeamCreateTool) Name() string { return "team_create" }
func (t *TeamCreateTool) Description() string {
	return "Create a lightweight in-memory team for agent tasks."
}
func (t *TeamCreateTool) IsReadOnly() bool { return false }

func (t *TeamCreateTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Team name",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Team description",
			},
		},
		"required": []string{"name"},
	}
}

type teamCreateInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func (t *TeamCreateTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input teamCreateInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	registry := coordinator.GetTeamRegistry()
	team, err := registry.CreateTeam(input.Name, input.Description)
	if err != nil {
		return ToolResult{Output: err.Error(), IsError: true}, nil
	}

	return NewSuccessResult(fmt.Sprintf("Created team %s", team.Name)), nil
}

type TeamDeleteTool struct{}

func (t *TeamDeleteTool) Name() string        { return "team_delete" }
func (t *TeamDeleteTool) Description() string { return "Delete a team by name." }
func (t *TeamDeleteTool) IsReadOnly() bool    { return false }

func (t *TeamDeleteTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Team name",
			},
		},
		"required": []string{"name"},
	}
}

type teamDeleteInput struct {
	Name string `json:"name"`
}

func (t *TeamDeleteTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input teamDeleteInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	registry := coordinator.GetTeamRegistry()
	if err := registry.DeleteTeam(input.Name); err != nil {
		return ToolResult{Output: err.Error(), IsError: true}, nil
	}

	return NewSuccessResult(fmt.Sprintf("Deleted team %s", input.Name)), nil
}
