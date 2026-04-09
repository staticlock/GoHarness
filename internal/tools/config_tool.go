package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/staticlock/GoHarness/internal/config"
)

type ConfigTool struct {
	Manager *config.ConfigManager
}

func (t *ConfigTool) Name() string { return "config" }
func (t *ConfigTool) Description() string {
	return "Read or update GoHarness settings."
}
func (t *ConfigTool) IsReadOnly() bool { return false }

func (t *ConfigTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action: show or set",
				"default":     "show",
			},
			"key": map[string]interface{}{
				"type":        "string",
				"description": "Configuration key to view or update",
			},
			"value": map[string]interface{}{
				"type":        "string",
				"description": "New value for the key (used with action=set)",
			},
		},
	}
}

type configInput struct {
	Action string `json:"action,omitempty"`
	Key    string `json:"key,omitempty"`
	Value  string `json:"value,omitempty"`
}

func (t *ConfigTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	_ = ctx
	_ = execCtx

	var input configInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	manager := t.Manager
	if manager == nil {
		manager = config.NewConfigManager()
		if err := manager.Load(); err != nil {
			return NewErrorResult(err), nil
		}
	}

	if input.Action == "" {
		input.Action = "show"
	}

	if input.Action == "show" {
		cfg := manager.Get()
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return NewErrorResult(err), nil
		}
		return NewSuccessResult(string(data)), nil
	}

	if input.Action == "set" {
		if input.Key == "" || input.Value == "" {
			return NewErrorResultf("key and value are required for action=set"), nil
		}

		err := manager.UpdateField(input.Key, input.Value)
		if err != nil {
			return NewErrorResult(err), nil
		}

		return NewSuccessResult(fmt.Sprintf("Updated %s", input.Key)), nil
	}

	return NewErrorResultf("Usage: action=show or action=set with key/value"), nil
}
