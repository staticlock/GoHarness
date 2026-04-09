package coordinator

type AgentDefinition struct {
	Name        string
	Description string
}

func GetBuiltinAgentDefinitions() []AgentDefinition {
	return []AgentDefinition{
		{Name: "default", Description: "General-purpose local coding agent"},
		{Name: "worker", Description: "Execution-focused worker agent"},
		{Name: "explorer", Description: "Read-heavy exploration agent"},
	}
}
