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

func loadClaudeMdPrompt(cwd string) string {
	candidates := []string{
		filepath.Join(cwd, "CLAUDE.md"),
		filepath.Join(cwd, ".claude", "settings.md"),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil {
			content := string(data)
			if len(content) > 12000 {
				content = content[:12000]
			}
			return "# CLAUDE.md\n\n```md\n" + content + "\n```"
		}
	}
	return ""
}

func loadIssueContext(cwd string) string {
	path := filepath.Join(cwd, ".github", "ISSUE_CONTEXT.md")
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
	path := filepath.Join(cwd, ".github", "PR_COMMENTS.md")
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

func loadMemorySection(cwd string, maxEntrypointLines int) string {
	prompt, err := memory.LoadMemoryPrompt(cwd)
	if err != nil || prompt == "" {
		return ""
	}
	lines := strings.Split(prompt, "\n")
	if len(lines) > maxEntrypointLines && maxEntrypointLines > 0 {
		lines = lines[:maxEntrypointLines]
	}
	prompt = strings.Join(lines, "\n")
	return "# Memory\n\n```md\n" + prompt + "\n```"
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
	return strings.Join(lines, "\n")
}

// BuildSystemPrompt assembles the full system prompt matching Python's build_runtime_system_prompt.
func BuildSystemPrompt(customPrompt string, cwd string) string {
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
		memorySection := loadMemorySection(cwd, memSettings.Memory.MaxEntrypointLines)
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
