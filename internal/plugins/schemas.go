package plugins

import "encoding/json"

// PluginManifest 插件清单
type PluginManifest struct {
	Name             string          `json:"name"`
	Version          string          `json:"version,omitempty"`
	Description      string          `json:"description,omitempty"`
	EnabledByDefault bool            `json:"enabled_by_default,omitempty"`
	SkillsDir        string          `json:"skills_dir,omitempty"`
	HooksFile        string          `json:"hooks_file,omitempty"`
	MCPFile          string          `json:"mcp_file,omitempty"`
	Author           json.RawMessage `json:"author,omitempty"`
	Commands         json.RawMessage `json:"commands,omitempty"`
	Agents           json.RawMessage `json:"agents,omitempty"`
	Skills           json.RawMessage `json:"skills,omitempty"`
	Hooks            json.RawMessage `json:"hooks,omitempty"`
}

func (m *PluginManifest) Validate() error {
	if m.Name == "" {
		return &PluginError{Message: "name is required"}
	}
	return nil
}

type PluginError struct {
	Message string
}

func (e *PluginError) Error() string {
	return e.Message
}
