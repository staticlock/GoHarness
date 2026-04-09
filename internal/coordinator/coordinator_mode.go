package coordinator

import "sync"

type TeamRecord struct {
	Name        string
	Description string
	Agents      []string
	Messages    []string
}

type TeamRegistry struct {
	mu    sync.RWMutex
	teams map[string]*TeamRecord
}

func NewTeamRegistry() *TeamRegistry {
	return &TeamRegistry{
		teams: make(map[string]*TeamRecord),
	}
}

func (r *TeamRegistry) CreateTeam(name string, description string) (*TeamRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.teams[name]; exists {
		return nil, &TeamError{Message: "Team '" + name + "' already exists"}
	}
	team := &TeamRecord{
		Name:        name,
		Description: description,
		Agents:      []string{},
		Messages:    []string{},
	}
	r.teams[name] = team
	return team, nil
}

func (r *TeamRegistry) DeleteTeam(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.teams[name]; !exists {
		return &TeamError{Message: "Team '" + name + "' does not exist"}
	}
	delete(r.teams, name)
	return nil
}

func (r *TeamRegistry) AddAgent(teamName string, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	team, exists := r.teams[teamName]
	if !exists {
		return &TeamError{Message: "Team '" + teamName + "' does not exist"}
	}
	for _, agent := range team.Agents {
		if agent == taskID {
			return nil
		}
	}
	team.Agents = append(team.Agents, taskID)
	return nil
}

func (r *TeamRegistry) SendMessage(teamName string, message string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	team, exists := r.teams[teamName]
	if !exists {
		return &TeamError{Message: "Team '" + teamName + "' does not exist"}
	}
	team.Messages = append(team.Messages, message)
	return nil
}

func (r *TeamRegistry) ListTeams() []*TeamRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	teams := make([]*TeamRecord, 0, len(r.teams))
	for _, team := range r.teams {
		teams = append(teams, team)
	}
	return teams
}

func (r *TeamRegistry) GetTeam(name string) (*TeamRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	team, exists := r.teams[name]
	if !exists {
		return nil, &TeamError{Message: "Team '" + name + "' does not exist"}
	}
	return team, nil
}

type TeamError struct {
	Message string
}

func (e *TeamError) Error() string {
	return e.Message
}

var defaultRegistry *TeamRegistry
var registryOnce sync.Once

func GetTeamRegistry() *TeamRegistry {
	registryOnce.Do(func() {
		defaultRegistry = NewTeamRegistry()
	})
	return defaultRegistry
}
