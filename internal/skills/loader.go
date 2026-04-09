package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/staticlock/GoHarness/internal/config"
)

// UserSkillsDir returns ~/.openharness/skills with env override support.
func UserSkillsDir() (string, error) {
	cfgDir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cfgDir, "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// LoadRegistry loads bundled and user skills.
func LoadRegistry(cwd string) (*Registry, error) {
	registry := NewRegistry()
	for _, skill := range loadBundledSkills(cwd) {
		registry.Register(skill)
	}
	userSkills, err := loadUserSkills()
	if err != nil {
		return nil, err
	}
	for _, skill := range userSkills {
		registry.Register(skill)
	}
	if settings, err := config.LoadSettings(); err == nil {
		for _, skill := range loadPluginSkills(cwd, settings.EnabledPlugins) {
			registry.Register(skill)
		}
	}
	return registry, nil
}

func loadUserSkills() ([]Definition, error) {
	dir, err := UserSkillsDir()
	if err != nil {
		return nil, err
	}
	return loadSkillsFromDir(dir, "user")
}

func loadBundledSkills(cwd string) []Definition {
	candidates := []string{
		filepath.Join(cwd, "src", "openharness", "skills", "bundled", "content"),
		filepath.Join(cwd, "..", "src", "openharness", "skills", "bundled", "content"),
	}
	for _, dir := range candidates {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			skills, _ := loadSkillsFromDir(dir, "bundled")
			return skills
		}
	}
	return nil
}

func loadSkillsFromDir(dir, source string) ([]Definition, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Definition
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		defaultName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		name, desc := parseSkillMarkdown(defaultName, content)
		out = append(out, Definition{Name: name, Description: desc, Content: content, Source: source, Path: path})
	}
	return out, nil
}

type pluginManifest struct {
	Name             string `json:"name"`
	EnabledByDefault bool   `json:"enabled_by_default"`
	SkillsDir        string `json:"skills_dir"`
}

func loadPluginSkills(cwd string, enabledPlugins map[string]bool) []Definition {
	pluginRoots := []string{}
	if cfgDir, err := config.ConfigDir(); err == nil {
		pluginRoots = append(pluginRoots, filepath.Join(cfgDir, "plugins"))
	}
	pluginRoots = append(pluginRoots, filepath.Join(cwd, ".openharness", "plugins"))

	all := []Definition{}
	for _, root := range pluginRoots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pluginDir := filepath.Join(root, entry.Name())
			manifestPath := findPluginManifest(pluginDir)
			if manifestPath == "" {
				continue
			}
			manifestBytes, err := os.ReadFile(manifestPath)
			if err != nil {
				continue
			}
			m := pluginManifest{SkillsDir: "skills"}
			if err := json.Unmarshal(manifestBytes, &m); err != nil || m.Name == "" {
				continue
			}
			enabled, ok := enabledPlugins[m.Name]
			if !ok {
				enabled = m.EnabledByDefault
			}
			if !enabled {
				continue
			}
			skills, err := loadSkillsFromDir(filepath.Join(pluginDir, m.SkillsDir), "plugin")
			if err == nil {
				all = append(all, skills...)
			}
		}
	}
	return all
}

func findPluginManifest(pluginDir string) string {
	candidates := []string{
		filepath.Join(pluginDir, "plugin.json"),
		filepath.Join(pluginDir, ".claude-plugin", "plugin.json"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func parseSkillMarkdown(defaultName, content string) (string, string) {
	name := defaultName
	description := ""
	lines := strings.Split(content, "\n")

	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				for _, fm := range lines[1:i] {
					fm = strings.TrimSpace(fm)
					if strings.HasPrefix(fm, "name:") {
						v := strings.Trim(strings.TrimSpace(strings.TrimPrefix(fm, "name:")), "\"'")
						if v != "" {
							name = v
						}
					}
					if strings.HasPrefix(fm, "description:") {
						v := strings.Trim(strings.TrimSpace(strings.TrimPrefix(fm, "description:")), "\"'")
						if v != "" {
							description = v
						}
					}
				}
				break
			}
		}
	}

	if description == "" {
		for _, line := range lines {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "# ") {
				if name == defaultName {
					name = strings.TrimSpace(strings.TrimPrefix(trim, "# "))
				}
				continue
			}
			if trim != "" && !strings.HasPrefix(trim, "---") && !strings.HasPrefix(trim, "#") {
				description = trim
				if len(description) > 200 {
					description = description[:200]
				}
				break
			}
		}
	}

	if description == "" {
		description = "Skill: " + name
	}
	return name, description
}
