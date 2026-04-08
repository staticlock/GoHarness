package tools

import (
	"context"
	"encoding/json"

	"github.com/user/goharness/internal/skills"
)

// SkillTool reads bundled/user skill contents by name.
type SkillTool struct{}

func (t *SkillTool) Name() string        { return "skill" }
func (t *SkillTool) Description() string { return "Read a bundled, user, or plugin skill by name." }
func (t *SkillTool) IsReadOnly() bool    { return true }

func (t *SkillTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Skill name",
			},
		},
		"required": []string{"name"},
	}
}

type skillInput struct {
	Name string `json:"name"`
}

func (t *SkillTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = ctx
	var input skillInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}
	registry, err := skills.LoadRegistry(execCtx.CWD)
	if err != nil {
		return NewErrorResult(err), nil
	}
	skill, ok := registry.Get(input.Name)
	if !ok {
		return ToolResult{Output: "Skill not found: " + input.Name, IsError: true}, nil
	}
	return NewSuccessResult(skill.Content), nil
}
