package prompts

import "strings"

const baseSystemPrompt = "You are an AI assistant integrated into an interactive CLI coding tool. " +
	"You help users with software engineering tasks including writing code, debugging, explaining code, " +
	"running commands, and managing files.\n\nBe concise and direct. " +
	"Prefer short responses unless detail is requested. Use markdown formatting when appropriate."

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

// BuildSystemPrompt assembles the full system prompt with environment context.
func BuildSystemPrompt(customPrompt string, cwd string) string {
	env := GetEnvironmentInfo(cwd)
	base := baseSystemPrompt
	if strings.TrimSpace(customPrompt) != "" {
		base = customPrompt
	}
	return base + "\n\n" + formatEnvironmentSection(env)
}
