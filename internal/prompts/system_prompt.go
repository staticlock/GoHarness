package prompts

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/staticlock/GoHarness/internal/config"
	"github.com/staticlock/GoHarness/internal/memory"
	"github.com/staticlock/GoHarness/internal/skills"
)

const baseSystemPrompt = "You are an AI assistant integrated into an interactive CLI coding tool. " +
	"You help users with software engineering tasks including writing code, debugging, explaining code, " +
	"running commands, and managing files.\n\nBe concise and direct. " +
	"Prefer short responses unless detail is requested. Use markdown formatting when appropriate."

func formatFastModeSection() string {
	settings, err := config.LoadSettings()
	if err != nil {
		return ""
	}
	if !settings.FastMode {
		return ""
	}
	return "# Session Mode\nFast mode is enabled. Prefer concise replies, minimal tool use, and quicker progress over exhaustive exploration."
}

func formatReasoningSettingsSection() string {
	settings, err := config.LoadSettings()
	if err != nil {
		return ""
	}
	lines := []string{
		"# Reasoning Settings",
		"- Effort: " + settings.Effort,
		"- Passes: " + formatPasses(settings.Passes),
	}
	return strings.Join(lines, "\n")
}

func formatPasses(passes int) string {
	switch passes {
	case 0:
		return "auto"
	default:
		return string(rune('0' + passes))
	}
}

func formatSkillsSection(cwd string) string {
	registry, err := skills.LoadRegistry(cwd)
	if err != nil || registry == nil {
		return ""
	}
	list := registry.List()
	if len(list) == 0 {
		return ""
	}
	lines := []string{"# Available Skills", "", "The following skills are available via the `skill` tool. When a user's request matches a skill, invoke it with `skill(name=\"<skill_name>\")` to load detailed instructions before proceeding.", ""}
	for _, s := range list {
		lines = append(lines, "- **"+s.Name+"**: "+s.Description)
	}
	return strings.Join(lines, "\n")
}

func discoverClaudeMdFiles(cwd string) []string {
	var results []string
	seen := map[string]bool{}

	current, err := filepath.Abs(cwd)
	if err != nil {
		return nil
	}

	for {
		for _, candidate := range []string{
			filepath.Join(current, "CLAUDE.md"),
			filepath.Join(current, ".claude", "CLAUDE.md"),
		} {
			if _, err := os.Stat(candidate); err == nil && !seen[candidate] {
				results = append(results, candidate)
				seen[candidate] = true
			}
		}
		rulesDir := filepath.Join(current, ".claude", "rules")
		if entries, err := os.ReadDir(rulesDir); err == nil {
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".md") {
					path := filepath.Join(rulesDir, entry.Name())
					if !seen[path] {
						results = append(results, path)
						seen[path] = true
					}
				}
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return results
}

func loadClaudeMdPrompt(cwd string) string {
	files := discoverClaudeMdFiles(cwd)
	if len(files) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "# Project Instructions")
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		if len(content) > 12000 {
			content = content[:12000] + "\n...[truncated]..."
		}
		relPath, _ := filepath.Rel(cwd, path)
		lines = append(lines, "", "## "+relPath, "```md", strings.TrimSpace(content), "```")
	}
	return strings.Join(lines, "\n")
}

func loadIssueContext(cwd string) string {
	projectDir := filepath.Join(cwd, ".openharness")
	path := filepath.Join(projectDir, "issue.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(data)
	if len(content) > 12000 {
		content = content[:12000]
	}
	return "# Issue Context\n\n```md\n" + content + "\n```"
}

func loadPRComments(cwd string) string {
	projectDir := filepath.Join(cwd, ".openharness")
	path := filepath.Join(projectDir, "pr_comments.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(data)
	if len(content) > 12000 {
		content = content[:12000]
	}
	return "# Pull Request Comments\n\n```md\n" + content + "\n```"
}

func loadMemorySection(cwd string, maxEntrypointLines int, latestUserPrompt string, maxRelevantFiles int) string {
	prompt, err := memory.LoadMemoryPrompt(cwd)
	if err != nil && prompt == "" {
		return ""
	}

	var lines []string
	lines = append(lines, "# Memory")
	lines = append(lines, "- Persistent memory directory: ~/.openharness/data/memory/")
	lines = append(lines, "- Use this directory to store durable user or project context that should survive future sessions.")
	lines = append(lines, "- Prefer concise topic files plus an index entry in MEMORY.md.")

	hasContent := false
	if prompt != "" {
		promptLines := strings.Split(prompt, "\n")
		if maxEntrypointLines > 0 && len(promptLines) > maxEntrypointLines {
			promptLines = promptLines[:maxEntrypointLines]
		}
		lines = append(lines, "", "## MEMORY.md", "```md")
		lines = append(lines, promptLines...)
		lines = append(lines, "```")
		hasContent = true
	}

	if !hasContent {
		lines = append(lines, "", "## MEMORY.md", "(not created yet)")
	}

	if latestUserPrompt != "" && maxRelevantFiles > 0 {
		relevant := memory.FindRelevantMemories(latestUserPrompt, cwd, maxRelevantFiles)
		if len(relevant) > 0 {
			lines = append(lines, "", "# Relevant Memories")
			for _, header := range relevant {
				data, err := os.ReadFile(header.Path)
				if err == nil {
					content := string(data)
					if len(content) > 8000 {
						content = content[:8000] + "\n...[truncated]..."
					}
					lines = append(lines, "", "## "+header.Title, "```md", strings.TrimSpace(content), "```")
				}
			}
		}
	}

	return strings.Join(lines, "\n")
}

func formatEnvironmentSection(env EnvironmentInfo) string {
	lines := []string{
		"# Environment",
		"- OS: " + env.OSName + " " + env.OSVersion,
		"- Architecture: " + env.PlatformMachine,
		"- Shell: " + env.Shell,
		"- Working directory: " + env.CWD,
		"- Date: " + env.Date,
		"- Go: " + env.GoVersion,
	}
	if env.IsGitRepo {
		gitLine := "- Git: yes"
		if strings.TrimSpace(env.GitBranch) != "" {
			gitLine += " (branch: " + env.GitBranch + ")"
		}
		lines = append(lines, gitLine)
	}
	if env.Hostname != "" {
		lines = append(lines, "- Hostname: "+env.Hostname)
	}
	return strings.Join(lines, "\n")
}

// BuildSystemPrompt assembles the full system prompt matching Python's build_runtime_system_prompt.
func BuildSystemPrompt(customPrompt string, cwd string, latestUserPrompt string) string {
	env := GetEnvironmentInfo(cwd)
	base := baseSystemPrompt
	if strings.TrimSpace(customPrompt) != "" {
		base = customPrompt
	}

	var sections []string

	// Base prompt
	sections = append(sections, base)

	// Environment section
	sections = append(sections, formatEnvironmentSection(env))

	// Session Mode (Fast mode)
	fastMode := formatFastModeSection()
	if fastMode != "" {
		sections = append(sections, fastMode)
	}

	// Reasoning Settings
	sections = append(sections, formatReasoningSettingsSection())

	// Skills
	skillsSection := formatSkillsSection(cwd)
	if skillsSection != "" {
		sections = append(sections, skillsSection)
	}

	// CLAUDE.md
	claudeMd := loadClaudeMdPrompt(cwd)
	if claudeMd != "" {
		sections = append(sections, claudeMd)
	}

	// Issue Context
	issueCtx := loadIssueContext(cwd)
	if issueCtx != "" {
		sections = append(sections, issueCtx)
	}

	// PR Comments
	prComments := loadPRComments(cwd)
	if prComments != "" {
		sections = append(sections, prComments)
	}

	// Memory (if enabled in settings)
	if memSettings, err := config.LoadSettings(); err == nil && memSettings.Memory.Enabled {
		memorySection := loadMemorySection(cwd, memSettings.Memory.MaxEntrypointLines, latestUserPrompt, memSettings.Memory.MaxFiles)
		if memorySection != "" {
			sections = append(sections, memorySection)
		}
	}

	// Join all sections
	var result []string
	for _, s := range sections {
		if strings.TrimSpace(s) != "" {
			result = append(result, s)
		}
	}
	return strings.Join(result, "\n\n")
}
