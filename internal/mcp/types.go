package mcp

// ToolInfo describes one MCP tool exposed by a server.
type ToolInfo struct {
	ServerName  string
	Name        string
	Description string
	InputSchema map[string]any
}

// ResourceInfo describes one MCP resource.
type ResourceInfo struct {
	ServerName  string
	Name        string
	URI         string
	Description string
}

// ConnectionStatus tracks runtime status for one MCP server.
type ConnectionStatus struct {
	Name           string
	State          string
	Detail         string
	Transport      string
	AuthConfigured bool
	Tools          []ToolInfo
	Resources      []ResourceInfo
}

// ServerConfig is a normalized MCP server config from settings.
type ServerConfig struct {
	Type    string
	Command string
	Args    []string
	Env     map[string]string
	CWD     string
	URL     string
	Headers map[string]string
}
