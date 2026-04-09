package hooks

import (
	"fmt"

	"github.com/staticlock/GoHarness/internal/config"
)

// LoadRegistry loads hooks from settings and optional plugin hooks.
func LoadRegistry(settings config.Settings, pluginHooks map[string][]Definition) *Registry {
	registry := NewRegistry()
	for rawEvent, hooks := range settings.Hooks {
		event, ok := parseEvent(rawEvent)
		if !ok {
			continue
		}
		for _, raw := range hooks {
			if def, ok := asDefinition(raw); ok {
				registry.Register(event, def)
			}
		}
	}
	for rawEvent, hooks := range pluginHooks {
		event, ok := parseEvent(rawEvent)
		if !ok {
			continue
		}
		for _, hook := range hooks {
			registry.Register(event, hook)
		}
	}
	return registry
}

func parseEvent(raw string) (Event, bool) {
	switch Event(raw) {
	case SessionStart, SessionEnd, PreToolUse, PostToolUse:
		return Event(raw), true
	default:
		return "", false
	}
}

func asDefinition(raw any) (Definition, bool) {
	m, ok := raw.(map[string]any)
	if !ok {
		return Definition{}, false
	}
	def := Definition{
		Type:           asString(m["type"]),
		Command:        asString(m["command"]),
		URL:            asString(m["url"]),
		Prompt:         asString(m["prompt"]),
		Model:          asString(m["model"]),
		Matcher:        asString(m["matcher"]),
		TimeoutSeconds: asInt(m["timeout_seconds"], 30),
		BlockOnFailure: asBool(m["block_on_failure"], false),
	}
	if headersMap, ok := m["headers"].(map[string]any); ok {
		def.Headers = map[string]string{}
		for k, v := range headersMap {
			def.Headers[k] = fmt.Sprintf("%v", v)
		}
	}
	if def.Type == "" {
		return Definition{}, false
	}
	return def, true
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asBool(v any, d bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return d
}

func asInt(v any, d int) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return d
	}
}
