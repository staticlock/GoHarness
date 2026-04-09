package coordinator

import (
	"testing"
)

func TestTeamRegistry(t *testing.T) {
	registry := NewTeamRegistry()

	team, err := registry.CreateTeam("alpha", "demo team")
	if err != nil {
		t.Fatalf("CreateTeam failed: %v", err)
	}
	if team.Name != "alpha" {
		t.Fatalf("expected team name alpha, got %s", team.Name)
	}

	err = registry.AddAgent("alpha", "agent1")
	if err != nil {
		t.Fatalf("AddAgent failed: %v", err)
	}

	err = registry.SendMessage("alpha", "hello world")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	team, _ = registry.GetTeam("alpha")
	if len(team.Agents) != 1 || team.Agents[0] != "agent1" {
		t.Fatalf("unexpected agents: %v", team.Agents)
	}
	if len(team.Messages) != 1 || team.Messages[0] != "hello world" {
		t.Fatalf("unexpected messages: %v", team.Messages)
	}

	err = registry.DeleteTeam("alpha")
	if err != nil {
		t.Fatalf("DeleteTeam failed: %v", err)
	}

	teams := registry.ListTeams()
	if len(teams) != 0 {
		t.Fatalf("expected empty teams, got %d", len(teams))
	}
}

func TestDuplicateTeam(t *testing.T) {
	registry := NewTeamRegistry()
	_, err := registry.CreateTeam("alpha", "first")
	if err != nil {
		t.Fatalf("CreateTeam failed: %v", err)
	}

	_, err = registry.CreateTeam("alpha", "second")
	if err == nil {
		t.Fatalf("expected error for duplicate team")
	}
}

func TestGetBuiltinAgentDefinitions(t *testing.T) {
	defs := GetBuiltinAgentDefinitions()
	if len(defs) != 3 {
		t.Fatalf("expected 3 agent definitions, got %d", len(defs))
	}
	if defs[0].Name != "default" {
		t.Fatalf("expected default agent, got %s", defs[0].Name)
	}
}

func TestGetTeamRegistrySingleton(t *testing.T) {
	registry1 := GetTeamRegistry()
	registry2 := GetTeamRegistry()
	if registry1 != registry2 {
		t.Fatalf("expected singleton registry")
	}
}
