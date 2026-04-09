package permissions

import (
	"path/filepath"

	"github.com/staticlock/GoHarness/internal/config"
)

// Decision is the result of evaluating one tool invocation.
type Decision struct {
	Allowed              bool
	RequiresConfirmation bool
	Reason               string
}

// Checker evaluates tool usage against configured mode and rules.
type Checker struct {
	settings  config.PermissionSettings
	pathRules []pathRule
}

type pathRule struct {
	Pattern string
	Allow   bool
}

// NewChecker builds a permission checker from settings.
func NewChecker(settings config.PermissionSettings) *Checker {
	c := &Checker{settings: settings, pathRules: make([]pathRule, 0, len(settings.PathRules))}
	for _, raw := range settings.PathRules {
		rule, ok := asPathRule(raw)
		if ok {
			c.pathRules = append(c.pathRules, rule)
		}
	}
	return c
}

// Evaluate returns whether a tool may run immediately.
func (c *Checker) Evaluate(toolName string, isReadOnly bool, filePath, command string) Decision {
	for _, denied := range c.settings.DeniedTools {
		if toolName == denied {
			return Decision{Allowed: false, Reason: toolName + " is explicitly denied"}
		}
	}

	for _, allowed := range c.settings.AllowedTools {
		if toolName == allowed {
			return Decision{Allowed: true, Reason: toolName + " is explicitly allowed"}
		}
	}

	if filePath != "" && len(c.pathRules) > 0 {
		for _, rule := range c.pathRules {
			ok, err := filepath.Match(rule.Pattern, filePath)
			if err == nil && ok && !rule.Allow {
				return Decision{Allowed: false, Reason: "Path " + filePath + " matches deny rule: " + rule.Pattern}
			}
		}
	}

	if command != "" {
		for _, pattern := range c.settings.DeniedCmds {
			ok, err := filepath.Match(pattern, command)
			if err == nil && ok {
				return Decision{Allowed: false, Reason: "Command matches deny pattern: " + pattern}
			}
		}
	}

	switch c.settings.Mode {
	case "full_auto":
		return Decision{Allowed: true, Reason: "Auto mode allows all tools"}
	case "plan":
		if isReadOnly {
			return Decision{Allowed: true, Reason: "read-only tools are allowed"}
		}
		return Decision{Allowed: false, Reason: "Plan mode blocks mutating tools until the user exits plan mode"}
	default:
		if isReadOnly {
			return Decision{Allowed: true, Reason: "read-only tools are allowed"}
		}
		return Decision{Allowed: false, RequiresConfirmation: true, Reason: "Mutating tools require user confirmation in default mode"}
	}
}

func asPathRule(raw any) (pathRule, bool) {
	m, ok := raw.(map[string]any)
	if !ok {
		return pathRule{}, false
	}
	patternRaw, ok := m["pattern"]
	if !ok {
		return pathRule{}, false
	}
	pattern, ok := patternRaw.(string)
	if !ok || pattern == "" {
		return pathRule{}, false
	}
	allow := true
	if allowRaw, ok := m["allow"]; ok {
		if b, ok := allowRaw.(bool); ok {
			allow = b
		}
	}
	return pathRule{Pattern: pattern, Allow: allow}, true
}
