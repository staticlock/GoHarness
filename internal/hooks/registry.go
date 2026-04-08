package hooks

import (
	"fmt"
	"sort"
	"strings"
)

// Definition is a normalized hook definition loaded from settings/plugins.
type Definition struct {
	Type           string
	Command        string
	URL            string
	Headers        map[string]string
	Prompt         string
	Model          string
	Matcher        string
	TimeoutSeconds int
	BlockOnFailure bool
}

// Registry stores hooks grouped by event.
type Registry struct {
	hooks map[Event][]Definition
}

// NewRegistry creates an empty hook registry.
func NewRegistry() *Registry {
	return &Registry{hooks: map[Event][]Definition{}}
}

// Register appends one hook for an event.
func (r *Registry) Register(event Event, hook Definition) {
	r.hooks[event] = append(r.hooks[event], hook)
}

// Get returns all hooks for an event.
func (r *Registry) Get(event Event) []Definition {
	items := r.hooks[event]
	out := make([]Definition, len(items))
	copy(out, items)
	return out
}

// Summary returns a readable overview of configured hooks.
func (r *Registry) Summary() string {
	events := make([]string, 0, len(r.hooks))
	for event := range r.hooks {
		events = append(events, string(event))
	}
	sort.Strings(events)

	var lines []string
	for _, name := range events {
		items := r.Get(Event(name))
		if len(items) == 0 {
			continue
		}
		lines = append(lines, name+":")
		for _, hook := range items {
			detail := hook.Command
			if detail == "" {
				detail = hook.Prompt
			}
			if detail == "" {
				detail = hook.URL
			}
			suffix := ""
			if hook.Matcher != "" {
				suffix = " matcher=" + hook.Matcher
			}
			lines = append(lines, fmt.Sprintf("  - %s%s: %s", hook.Type, suffix, detail))
		}
	}
	return strings.Join(lines, "\n")
}
