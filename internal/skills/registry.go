package skills

import "strings"

// Registry stores loaded skills by normalized name.
type Registry struct {
	items map[string]Definition
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{items: map[string]Definition{}}
}

// Register adds/overwrites one skill.
func (r *Registry) Register(skill Definition) {
	r.items[strings.ToLower(strings.TrimSpace(skill.Name))] = skill
}

// Get fetches one skill by name.
func (r *Registry) Get(name string) (Definition, bool) {
	skill, ok := r.items[strings.ToLower(strings.TrimSpace(name))]
	return skill, ok
}

// List returns all skills.
func (r *Registry) List() []Definition {
	out := make([]Definition, 0, len(r.items))
	for _, skill := range r.items {
		out = append(out, skill)
	}
	return out
}
