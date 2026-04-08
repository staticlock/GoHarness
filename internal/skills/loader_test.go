package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/goharness/internal/config"
)

func TestParseSkillMarkdown(t *testing.T) {
	content := "---\nname: My Skill\ndescription: Useful helper\n---\n# Ignore\nBody"
	name, desc := parseSkillMarkdown("default", content)
	if name != "My Skill" || desc != "Useful helper" {
		t.Fatalf("unexpected parsed values: %q %q", name, desc)
	}
}

func TestLoadSkillsFromDir(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "example.md")
	data := "# Example\nThis is a sample skill."
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write test skill failed: %v", err)
	}

	items, err := loadSkillsFromDir(tmp, "user")
	if err != nil {
		t.Fatalf("load skills failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one skill, got %d", len(items))
	}
	if items[0].Name != "Example" {
		t.Fatalf("unexpected skill name: %q", items[0].Name)
	}
}

func TestLoadRegistryIncludesEnabledPluginSkills(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Setenv("OPENHARNESS_CONFIG_DIR", filepath.Join(tmp, "cfg")); err != nil {
		t.Fatalf("set env failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("OPENHARNESS_CONFIG_DIR") })

	pluginDir := filepath.Join(tmp, ".openharness", "plugins", "demo")
	if err := os.MkdirAll(filepath.Join(pluginDir, "skills"), 0o755); err != nil {
		t.Fatalf("mkdir plugin failed: %v", err)
	}
	manifest := `{"name":"demo","enabled_by_default":true,"skills_dir":"skills"}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "skills", "p.md"), []byte("# Plugin Skill\nfrom plugin"), 0o644); err != nil {
		t.Fatalf("write plugin skill failed: %v", err)
	}

	settings := config.Settings{EnabledPlugins: map[string]bool{"demo": true}}
	if err := config.SaveSettings(settings); err != nil {
		t.Fatalf("save settings failed: %v", err)
	}

	registry, err := LoadRegistry(tmp)
	if err != nil {
		t.Fatalf("load registry failed: %v", err)
	}
	if _, ok := registry.Get("Plugin Skill"); !ok {
		t.Fatalf("expected plugin skill to be loaded")
	}
}
