package permissions

import (
	"testing"

	"github.com/staticlock/GoHarness/internal/config"
)

func TestCheckerModes(t *testing.T) {
	checker := NewChecker(config.PermissionSettings{Mode: "default"})
	decision := checker.Evaluate("write_file", false, "", "")
	if decision.Allowed {
		t.Fatalf("default mode mutating tool should not be directly allowed")
	}
	if !decision.RequiresConfirmation {
		t.Fatalf("default mode mutating tool should require confirmation")
	}

	planChecker := NewChecker(config.PermissionSettings{Mode: "plan"})
	decision = planChecker.Evaluate("write_file", false, "", "")
	if decision.Allowed {
		t.Fatalf("plan mode mutating tool should be blocked")
	}

	autoChecker := NewChecker(config.PermissionSettings{Mode: "full_auto"})
	decision = autoChecker.Evaluate("write_file", false, "", "")
	if !decision.Allowed {
		t.Fatalf("full_auto mode should allow mutating tools")
	}
}

func TestCheckerRules(t *testing.T) {
	checker := NewChecker(config.PermissionSettings{
		Mode:        "full_auto",
		DeniedTools: []string{"bash"},
		DeniedCmds:  []string{"rm *"},
		PathRules: []any{
			map[string]any{"pattern": "secrets/*", "allow": false},
		},
	})

	if checker.Evaluate("bash", false, "", "echo hi").Allowed {
		t.Fatalf("denied tool should be blocked")
	}
	if checker.Evaluate("write_file", false, "secrets/key.txt", "").Allowed {
		t.Fatalf("deny path rule should block")
	}
	if checker.Evaluate("bash", false, "", "rm file.txt").Allowed {
		t.Fatalf("deny command pattern should block")
	}
}
