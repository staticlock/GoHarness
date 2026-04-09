package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/staticlock/GoHarness/internal/config"
	"github.com/staticlock/GoHarness/internal/mcp"
)

type McpAuthToolInput struct {
	ServerName string `json:"server_name"`
	Mode       string `json:"mode"`
	Value      string `json:"value"`
	Key        string `json:"key,omitempty"`
}

type McpAuthTool struct {
	Manager *mcp.ClientManager
}

func (t *McpAuthTool) Name() string { return "mcp_auth" }
func (t *McpAuthTool) Description() string {
	return "Configure auth for an MCP server and reconnect active sessions when possible."
}
func (t *McpAuthTool) IsReadOnly() bool { return false }

func (t *McpAuthTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"server_name": map[string]interface{}{
				"type":        "string",
				"description": "Configured MCP server name",
			},
			"mode": map[string]interface{}{
				"type":        "string",
				"description": "Auth mode: bearer, header, or env",
				"enum":        []string{"bearer", "header", "env"},
			},
			"value": map[string]interface{}{
				"type":        "string",
				"description": "Secret value to persist",
			},
			"key": map[string]interface{}{
				"type":        "string",
				"description": "Header or env key override",
			},
		},
		"required": []string{"server_name", "mode", "value"},
	}
}

func (t *McpAuthTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input McpAuthToolInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	settings, err := config.LoadSettings()
	if err != nil {
		return NewErrorResult(err), nil
	}

	serverConfig, ok := settings.MCPServers[input.ServerName]
	if !ok {
		if t.Manager != nil {
			serverConfig = t.Manager.GetServerConfig(input.ServerName)
			ok = serverConfig != nil
		}
		if !ok {
			return NewErrorResult(fmt.Errorf("unknown MCP server: %s", input.ServerName)), nil
		}
	}

	cfg, ok := serverConfig.(map[string]any)
	if !ok {
		return NewErrorResult(fmt.Errorf("unsupported MCP server config type")), nil
	}

	serverType := cfg["type"]
	if serverType == nil {
		serverType = "stdio"
	}

	var updated map[string]any
	switch serverType.(string) {
	case "stdio":
		if input.Mode != "env" && input.Mode != "bearer" {
			return NewErrorResult(fmt.Errorf("stdio MCP auth supports env or bearer modes")), nil
		}
		key := input.Key
		if key == "" {
			key = "MCP_AUTH_TOKEN"
		}
		env := cfg["env"]
		if env == nil {
			env = map[string]any{}
		}
		envMap, ok := env.(map[string]any)
		if !ok {
			envMap = map[string]any{}
		}
		if input.Mode == "bearer" {
			envMap[key] = "Bearer " + input.Value
		} else {
			envMap[key] = input.Value
		}
		updated = copyMap(cfg)
		updated["env"] = envMap

	case "http", "websocket":
		if input.Mode != "header" && input.Mode != "bearer" {
			return NewErrorResult(fmt.Errorf("http/ws MCP auth supports header or bearer modes")), nil
		}
		key := input.Key
		if key == "" {
			key = "Authorization"
		}
		headers := cfg["headers"]
		if headers == nil {
			headers = map[string]any{}
		}
		headersMap, ok := headers.(map[string]any)
		if !ok {
			headersMap = map[string]any{}
		}
		if input.Mode == "bearer" && key == "Authorization" {
			headersMap[key] = "Bearer " + input.Value
		} else {
			headersMap[key] = input.Value
		}
		updated = copyMap(cfg)
		updated["headers"] = headersMap

	default:
		return NewErrorResult(fmt.Errorf("unsupported MCP server type: %s", serverType)), nil
	}

	settings.MCPServers[input.ServerName] = updated
	if err := config.SaveSettings(settings); err != nil {
		return NewErrorResult(err), nil
	}

	if t.Manager != nil {
		t.Manager.UpdateServerConfig(input.ServerName, toServerConfig(updated))
		t.Manager.ReconnectAll()
	}

	return NewSuccessResult(fmt.Sprintf("Saved MCP auth for %s", input.ServerName)), nil
}

func copyMap(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

func toServerConfig(raw map[string]any) mcp.ServerConfig {
	return mcp.ServerConfig{
		Type:    asString(raw["type"]),
		Command: asString(raw["command"]),
		Args:    asStringSlice(raw["args"]),
		Env:     asStringMap(raw["env"]),
		CWD:     asString(raw["cwd"]),
		URL:     asString(raw["url"]),
		Headers: asStringMap(raw["headers"]),
	}
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asStringSlice(v any) []string {
	if raw, ok := v.([]any); ok {
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			out = append(out, fmt.Sprintf("%v", item))
		}
		return out
	}
	return nil
}

func asStringMap(v any) map[string]string {
	if raw, ok := v.(map[string]any); ok {
		out := map[string]string{}
		for k, val := range raw {
			out[k] = fmt.Sprintf("%v", val)
		}
		return out
	}
	return nil
}
