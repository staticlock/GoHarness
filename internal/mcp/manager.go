package mcp

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// TransportClient is the minimal MCP transport contract used by the manager.
type TransportClient interface {
	ListTools() ([]ToolInfo, error)
	ListResources() ([]ResourceInfo, error)
	CallTool(toolName string, arguments map[string]any) (string, error)
	ReadResource(uri string) (string, error)
	Close() error
}

type transportFactory func(serverName string, cfg ServerConfig) (TransportClient, error)

// ClientManager manages MCP server statuses and tool/resource exposure.
type ClientManager struct {
	serverConfigs map[string]ServerConfig
	statuses      map[string]ConnectionStatus
	transports    map[string]TransportClient
	newTransport  transportFactory
}

// NewClientManager builds a manager with pending statuses.
func NewClientManager(serverConfigs map[string]ServerConfig) *ClientManager {
	statuses := map[string]ConnectionStatus{}
	for name, cfg := range serverConfigs {
		statuses[name] = ConnectionStatus{Name: name, State: "pending", Transport: cfg.Type, AuthConfigured: len(cfg.Env) > 0 || len(cfg.Headers) > 0}
	}
	return &ClientManager{
		serverConfigs: serverConfigs,
		statuses:      statuses,
		transports:    map[string]TransportClient{},
		newTransport:  defaultTransportFactory,
	}
}

// SetTransportFactory overrides transport creation (useful for tests).
func (m *ClientManager) SetTransportFactory(factory func(serverName string, cfg ServerConfig) (TransportClient, error)) {
	if factory == nil {
		m.newTransport = defaultTransportFactory
		return
	}
	m.newTransport = factory
}

// ConnectAll initializes configured MCP transports and refreshes statuses.
func (m *ClientManager) ConnectAll() {
	for name, cfg := range m.serverConfigs {
		client, err := m.newTransport(name, cfg)
		if err != nil {
			state := "failed"
			if strings.Contains(strings.ToLower(err.Error()), "not yet implemented") {
				state = "disabled"
			}
			m.statuses[name] = ConnectionStatus{Name: name, State: state, Transport: cfg.Type, AuthConfigured: len(cfg.Env) > 0 || len(cfg.Headers) > 0, Detail: userDebugMessage("failed to connect MCP server", err)}
			continue
		}
		tools, toolsErr := client.ListTools()
		resources, resourcesErr := client.ListResources()
		if toolsErr != nil || resourcesErr != nil {
			_ = client.Close()
			detail := ""
			if toolsErr != nil {
				detail = toolsErr.Error()
			}
			if detail == "" && resourcesErr != nil {
				detail = resourcesErr.Error()
			}
			m.statuses[name] = ConnectionStatus{Name: name, State: "failed", Transport: cfg.Type, AuthConfigured: len(cfg.Env) > 0 || len(cfg.Headers) > 0, Detail: userDebugMessage("failed to initialize MCP server", fmt.Errorf("%s", detail))}
			continue
		}
		m.transports[name] = client
		m.statuses[name] = ConnectionStatus{Name: name, State: "connected", Transport: cfg.Type, AuthConfigured: len(cfg.Env) > 0 || len(cfg.Headers) > 0, Tools: tools, Resources: resources}
	}
}

// ReconnectAll closes current transports and reconnects configured servers.
func (m *ClientManager) ReconnectAll() {
	m.Close()
	m.ConnectAll()
}

// Close closes active transports and resets statuses to pending.
func (m *ClientManager) Close() {
	for name, client := range m.transports {
		_ = client.Close()
		delete(m.transports, name)
	}
	for name, cfg := range m.serverConfigs {
		m.statuses[name] = ConnectionStatus{Name: name, State: "pending", Transport: cfg.Type}
	}
}

// ListStatuses returns statuses in sorted order by name.
func (m *ClientManager) ListStatuses() []ConnectionStatus {
	names := make([]string, 0, len(m.statuses))
	for name := range m.statuses {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]ConnectionStatus, 0, len(names))
	for _, name := range names {
		out = append(out, m.statuses[name])
	}
	return out
}

// ListTools returns all currently exposed MCP tools.
func (m *ClientManager) ListTools() []ToolInfo {
	var out []ToolInfo
	for _, status := range m.ListStatuses() {
		out = append(out, status.Tools...)
	}
	return out
}

// ListResources returns all currently exposed MCP resources.
func (m *ClientManager) ListResources() []ResourceInfo {
	var out []ResourceInfo
	for _, status := range m.ListStatuses() {
		out = append(out, status.Resources...)
	}
	return out
}

// CallTool invokes one MCP tool.
func (m *ClientManager) CallTool(serverName, toolName string, arguments map[string]any) (string, error) {
	status, ok := m.statuses[serverName]
	if !ok {
		return "", errors.New(userDebugMessage("MCP tool call failed: unknown server", fmt.Errorf("unknown mcp server: %s", serverName)))
	}
	if status.State != "connected" {
		return "", errors.New(userDebugMessage("MCP tool call failed: server not connected", fmt.Errorf("mcp server %s state=%s detail=%s", serverName, status.State, status.Detail)))
	}
	client, ok := m.transports[serverName]
	if !ok {
		return "", errors.New(userDebugMessage("MCP tool call failed: transport unavailable", fmt.Errorf("mcp server %s transport missing", serverName)))
	}
	out, err := client.CallTool(toolName, arguments)
	if err != nil {
		return "", errors.New(userDebugMessage("MCP tool call failed", err))
	}
	return out, nil
}

// ReadResource reads one MCP resource URI.
func (m *ClientManager) ReadResource(serverName, uri string) (string, error) {
	status, ok := m.statuses[serverName]
	if !ok {
		return "", errors.New(userDebugMessage("MCP resource read failed: unknown server", fmt.Errorf("unknown mcp server: %s", serverName)))
	}
	if status.State != "connected" {
		return "", errors.New(userDebugMessage("MCP resource read failed: server not connected", fmt.Errorf("mcp server %s state=%s detail=%s", serverName, status.State, status.Detail)))
	}
	client, ok := m.transports[serverName]
	if !ok {
		return "", errors.New(userDebugMessage("MCP resource read failed: transport unavailable", fmt.Errorf("mcp server %s transport missing", serverName)))
	}
	out, err := client.ReadResource(uri)
	if err != nil {
		return "", errors.New(userDebugMessage("MCP resource read failed", err))
	}
	return out, nil
}

func defaultTransportFactory(serverName string, cfg ServerConfig) (TransportClient, error) {
	switch cfg.Type {
	case "stdio", "":
		return newStdioClient(serverName, cfg)
	case "http":
		return newHTTPClient(serverName, cfg)
	case "ws":
		return newWSClient(serverName, cfg)
	default:
		return nil, fmt.Errorf("unsupported MCP transport: %s", cfg.Type)
	}
}

func userDebugMessage(userText string, debugErr error) string {
	if debugErr == nil {
		return userText
	}
	return userText + " | debug: " + debugErr.Error()
}
