package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/staticlock/GoHarness/internal/config"
	"github.com/staticlock/GoHarness/internal/engine"
	"github.com/staticlock/GoHarness/internal/services"
	"github.com/staticlock/GoHarness/internal/tasks"
	"github.com/staticlock/GoHarness/internal/tools"
)

func TestLookupAndHelp(t *testing.T) {
	registry := CreateDefaultRegistry()
	cmd, args := registry.Lookup("/help")
	if cmd == nil || cmd.Name != "help" || args != "" {
		t.Fatalf("expected /help lookup to resolve command")
	}
	if _, args = registry.Lookup("/status now"); args != "now" {
		t.Fatalf("expected args to be parsed, got %q", args)
	}
	help := registry.HelpText()
	if !strings.Contains(help, "/help") || !strings.Contains(help, "/exit") || !strings.Contains(help, "/usage") || !strings.Contains(help, "/context") || !strings.Contains(help, "/cost") || !strings.Contains(help, "/files") || !strings.Contains(help, "/skills") || !strings.Contains(help, "/memory") {
		t.Fatalf("help text missing expected commands: %s", help)
	}
}

func TestStatusAndClearCommand(t *testing.T) {
	registry := CreateDefaultRegistry()
	qe := engine.NewQueryEngine(engine.QueryContext{})
	qe.LoadMessages([]engine.ConversationMessage{{Role: "user", Text: "hello"}, {Role: "assistant", Text: "world"}})
	ctx := Context{Engine: qe, CWD: t.TempDir()}

	status, _ := registry.Lookup("/status")
	statusResult := status.Handler("", ctx)
	if !strings.Contains(statusResult.Message, "Messages: 2") {
		t.Fatalf("unexpected status output: %s", statusResult.Message)
	}

	clearCmd, _ := registry.Lookup("/clear")
	clearResult := clearCmd.Handler("", ctx)
	if !clearResult.ClearScreen {
		t.Fatalf("expected /clear to request clear screen")
	}
	if got := len(qe.Messages()); got != 0 {
		t.Fatalf("expected cleared messages, got %d", got)
	}
}

func TestVersionAndResumeLatestFallback(t *testing.T) {
	registry := CreateDefaultRegistry()
	cwd := t.TempDir()
	qe := engine.NewQueryEngine(engine.QueryContext{})
	ctx := Context{Engine: qe, CWD: cwd}

	_, err := services.SaveSessionSnapshot(cwd, "claude-sonnet-4-20250514", "", "", []engine.ConversationMessage{{Role: "user", Text: "saved"}, {Role: "assistant", Text: "ok"}}, engine.UsageSnapshot{})
	if err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	versionCmd, _ := registry.Lookup("/version")
	versionResult := versionCmd.Handler("", ctx)
	if strings.TrimSpace(versionResult.Message) == "" {
		t.Fatalf("unexpected version output: %s", versionResult.Message)
	}

	resumeCmd, _ := registry.Lookup("/resume latest")
	resumeResult := resumeCmd.Handler("latest", ctx)
	if !strings.Contains(resumeResult.Message, "Restored") {
		t.Fatalf("unexpected resume output: %s", resumeResult.Message)
	}
	if got := len(qe.Messages()); got == 0 {
		t.Fatalf("expected messages to be restored")
	}
}

func TestUsageAndContextCommands(t *testing.T) {
	registry := CreateDefaultRegistry()
	qe := engine.NewQueryEngine(engine.QueryContext{SystemPrompt: "System prompt from runtime"})
	qe.LoadMessages([]engine.ConversationMessage{{Role: "user", Text: "hello world"}, {Role: "assistant", Text: "hi"}})
	ctx := Context{Engine: qe, CWD: t.TempDir()}

	usageCmd, _ := registry.Lookup("/usage")
	usageResult := usageCmd.Handler("", ctx)
	if !strings.Contains(usageResult.Message, "Actual usage") || !strings.Contains(usageResult.Message, "Estimated conversation tokens") {
		t.Fatalf("unexpected usage output: %s", usageResult.Message)
	}

	contextCmd, _ := registry.Lookup("/context")
	contextResult := contextCmd.Handler("", ctx)
	if contextResult.Message != "System prompt from runtime" {
		t.Fatalf("unexpected context output: %s", contextResult.Message)
	}
}

func TestSummaryCompactAndStatsCommands(t *testing.T) {
	registry := CreateDefaultRegistry()
	qe := engine.NewQueryEngine(engine.QueryContext{})
	qe.LoadMessages([]engine.ConversationMessage{
		{Role: "user", Text: "first user message"},
		{Role: "assistant", Text: "first assistant message"},
		{Role: "user", Text: "second user message"},
		{Role: "assistant", Text: "second assistant message"},
	})
	r := tools.NewRegistry()
	r.Register(&tools.ReadTool{})
	ctx := Context{Engine: qe, CWD: t.TempDir(), ToolRegistry: r}

	summaryCmd, _ := registry.Lookup("/summary 2")
	summaryResult := summaryCmd.Handler("2", ctx)
	if !strings.Contains(summaryResult.Message, "Conversation summary") || !strings.Contains(summaryResult.Message, "assistant") {
		t.Fatalf("unexpected summary output: %s", summaryResult.Message)
	}

	compactCmd, _ := registry.Lookup("/compact 2")
	compactResult := compactCmd.Handler("2", ctx)
	if !strings.Contains(compactResult.Message, "Compacted conversation") {
		t.Fatalf("unexpected compact output: %s", compactResult.Message)
	}
	if got := len(qe.Messages()); got != 2 {
		t.Fatalf("expected compacted message count=2, got %d", got)
	}

	statsCmd, _ := registry.Lookup("/stats")
	statsResult := statsCmd.Handler("", ctx)
	if !strings.Contains(statsResult.Message, "tools: 1") || !strings.Contains(statsResult.Message, "messages: 2") {
		t.Fatalf("unexpected stats output: %s", statsResult.Message)
	}
}

func TestSettingsCommands(t *testing.T) {
	t.Setenv("OPENHARNESS_CONFIG_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_MODEL", "")
	t.Setenv("OPENHARNESS_MODEL", "")
	registry := CreateDefaultRegistry()
	qe := engine.NewQueryEngine(engine.QueryContext{})
	ctx := Context{Engine: qe, CWD: t.TempDir()}

	modelCmd, _ := registry.Lookup("/model set test-model")
	modelResult := modelCmd.Handler("set test-model", ctx)
	if !strings.Contains(modelResult.Message, "Model set to test-model") {
		t.Fatalf("unexpected model set output: %s", modelResult.Message)
	}
	modelShow, _ := registry.Lookup("/model show")
	if got := modelShow.Handler("show", ctx).Message; !strings.Contains(got, "test-model") {
		t.Fatalf("unexpected model show output: %s", got)
	}

	permissionsCmd, _ := registry.Lookup("/permissions set plan")
	if got := permissionsCmd.Handler("set plan", ctx).Message; !strings.Contains(got, "Permission mode set to plan") {
		t.Fatalf("unexpected permissions output: %s", got)
	}
	planCmd, _ := registry.Lookup("/plan off")
	if got := planCmd.Handler("off", ctx).Message; !strings.Contains(got, "Permission mode set to default") {
		t.Fatalf("unexpected plan output: %s", got)
	}

	fastCmd, _ := registry.Lookup("/fast on")
	if got := fastCmd.Handler("on", ctx).Message; !strings.Contains(got, "enabled") {
		t.Fatalf("unexpected fast output: %s", got)
	}

	effortCmd, _ := registry.Lookup("/effort set high")
	if got := effortCmd.Handler("set high", ctx).Message; !strings.Contains(got, "Effort set to high") {
		t.Fatalf("unexpected effort output: %s", got)
	}

	passesCmd, _ := registry.Lookup("/passes set 3")
	if got := passesCmd.Handler("set 3", ctx).Message; !strings.Contains(got, "Passes set to 3") {
		t.Fatalf("unexpected passes output: %s", got)
	}

	settings, err := config.LoadSettings()
	if err != nil {
		t.Fatalf("load settings failed: %v", err)
	}
	if settings.Model != "test-model" || settings.Permission.Mode != "default" || !settings.FastMode || settings.Effort != "high" || settings.Passes != 3 {
		t.Fatalf("settings not updated as expected: %+v", settings)
	}
}

func TestCostSessionAndFilesCommands(t *testing.T) {
	t.Setenv("OPENHARNESS_CONFIG_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_MODEL", "")
	t.Setenv("OPENHARNESS_MODEL", "")

	registry := CreateDefaultRegistry()
	cwd := t.TempDir()
	qe := engine.NewQueryEngine(engine.QueryContext{})
	qe.LoadMessages([]engine.ConversationMessage{{Role: "user", Text: "hello"}})
	ctx := Context{Engine: qe, CWD: cwd}

	settings, err := config.LoadSettings()
	if err != nil {
		t.Fatalf("load settings failed: %v", err)
	}
	settings.Model = "claude-3-7-sonnet-test"
	if err := config.SaveSettings(settings); err != nil {
		t.Fatalf("save settings failed: %v", err)
	}

	costCmd, _ := registry.Lookup("/cost")
	costResult := costCmd.Handler("", ctx)
	if !strings.Contains(costResult.Message, "Estimated cost") || !strings.Contains(costResult.Message, "claude-3-7-sonnet-test") {
		t.Fatalf("unexpected cost output: %s", costResult.Message)
	}

	_, err = services.SaveSessionSnapshot(cwd, settings.Model, "prompt", "", []engine.ConversationMessage{{Role: "user", Text: "saved"}}, engine.UsageSnapshot{})
	if err != nil {
		t.Fatalf("save session failed: %v", err)
	}

	sessionCmd, _ := registry.Lookup("/session latest")
	sessionResult := sessionCmd.Handler("latest", ctx)
	if !strings.Contains(sessionResult.Message, "Session ") || !strings.Contains(sessionResult.Message, "Messages:") {
		t.Fatalf("unexpected session output: %s", sessionResult.Message)
	}

	sessions, listErr := services.ListSessions(cwd, 10)
	if listErr != nil || len(sessions) == 0 {
		t.Fatalf("failed to list sessions: %v", listErr)
	}
	sessionShowCmd, _ := registry.Lookup("/session show " + sessions[0].SessionID)
	sessionShowResult := sessionShowCmd.Handler("show "+sessions[0].SessionID, ctx)
	if !strings.Contains(sessionShowResult.Message, sessions[0].SessionID) || !strings.Contains(sessionShowResult.Message, "Summary:") {
		t.Fatalf("unexpected session show output: %s", sessionShowResult.Message)
	}

	if err := os.WriteFile(filepath.Join(cwd, "alpha.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	if err := os.Mkdir(filepath.Join(cwd, "dir"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	filesCmd, _ := registry.Lookup("/files")
	filesResult := filesCmd.Handler("", ctx)
	if !strings.Contains(filesResult.Message, "alpha.txt") || !strings.Contains(filesResult.Message, "dir/") {
		t.Fatalf("unexpected files output: %s", filesResult.Message)
	}
}

func TestDiffAndBranchCommandsOutsideGit(t *testing.T) {
	registry := CreateDefaultRegistry()
	ctx := Context{Engine: engine.NewQueryEngine(engine.QueryContext{}), CWD: t.TempDir()}

	diffCmd, _ := registry.Lookup("/diff")
	diffResult := diffCmd.Handler("", ctx)
	if strings.TrimSpace(diffResult.Message) == "" {
		t.Fatalf("expected non-empty /diff output")
	}

	branchCmd, _ := registry.Lookup("/branch")
	branchResult := branchCmd.Handler("", ctx)
	if strings.TrimSpace(branchResult.Message) == "" {
		t.Fatalf("expected non-empty /branch output")
	}
}

func TestTasksReadOnlyCommands(t *testing.T) {
	registry := CreateDefaultRegistry()
	cwd := t.TempDir()
	qe := engine.NewQueryEngine(engine.QueryContext{})
	ctx := Context{Engine: qe, CWD: cwd}

	manager := tasks.DefaultManager()
	record := manager.CreateShellTask("echo hello", "demo task", cwd)

	tasksListCmd, _ := registry.Lookup("/tasks list")
	listResult := tasksListCmd.Handler("list", ctx)
	if !strings.Contains(listResult.Message, record.ID) {
		t.Fatalf("unexpected tasks list output: %s", listResult.Message)
	}

	tasksShowCmd, _ := registry.Lookup("/tasks show " + record.ID)
	showResult := tasksShowCmd.Handler("show "+record.ID, ctx)
	if !strings.Contains(showResult.Message, "Task "+record.ID) || !strings.Contains(showResult.Message, "demo task") {
		t.Fatalf("unexpected tasks show output: %s", showResult.Message)
	}

	tasksOutputCmd, _ := registry.Lookup("/tasks output " + record.ID)
	outputResult := tasksOutputCmd.Handler("output "+record.ID, ctx)
	if outputResult.Message != "(no output)" {
		t.Fatalf("unexpected tasks output: %s", outputResult.Message)
	}

	recordNoOutput := manager.CreateShellTask("echo none", "no output task", cwd)
	tasksOutputNone, _ := registry.Lookup("/tasks output " + recordNoOutput.ID)
	outputNoneResult := tasksOutputNone.Handler("output "+recordNoOutput.ID, ctx)
	if outputNoneResult.Message != "(no output)" {
		t.Fatalf("unexpected no-output response: %s", outputNoneResult.Message)
	}
}

func TestEnvironmentAndConfigCommands(t *testing.T) {
	t.Setenv("OPENHARNESS_CONFIG_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_MODEL", "")
	t.Setenv("OPENHARNESS_MODEL", "")

	registry := CreateDefaultRegistry()
	ctx := Context{Engine: engine.NewQueryEngine(engine.QueryContext{}), CWD: t.TempDir()}

	themeCmd, _ := registry.Lookup("/theme set dark")
	if got := themeCmd.Handler("set dark", ctx).Message; !strings.Contains(got, "Theme set to dark") {
		t.Fatalf("unexpected theme output: %s", got)
	}

	vimCmd, _ := registry.Lookup("/vim on")
	if got := vimCmd.Handler("on", ctx).Message; !strings.Contains(got, "enabled") {
		t.Fatalf("unexpected vim output: %s", got)
	}

	voiceCmd, _ := registry.Lookup("/voice toggle")
	if got := voiceCmd.Handler("toggle", ctx).Message; !strings.Contains(got, "Voice mode") {
		t.Fatalf("unexpected voice output: %s", got)
	}

	hooksCmd, _ := registry.Lookup("/hooks")
	if got := hooksCmd.Handler("", ctx).Message; strings.TrimSpace(got) == "" {
		t.Fatalf("unexpected hooks output: %q", got)
	}

	mcpCmd, _ := registry.Lookup("/mcp")
	if got := mcpCmd.Handler("", ctx).Message; strings.TrimSpace(got) == "" {
		t.Fatalf("unexpected mcp output: %q", got)
	}

	doctorCmd, _ := registry.Lookup("/doctor")
	if got := doctorCmd.Handler("", ctx).Message; !strings.Contains(got, "Doctor summary") || !strings.Contains(got, "permission_mode") {
		t.Fatalf("unexpected doctor output: %s", got)
	}

	configSetCmd, _ := registry.Lookup("/config set model model-x")
	if got := configSetCmd.Handler("set model model-x", ctx).Message; !strings.Contains(got, "Updated config model") {
		t.Fatalf("unexpected config set output: %s", got)
	}
	configShowCmd, _ := registry.Lookup("/config show")
	if got := configShowCmd.Handler("show", ctx).Message; !strings.Contains(got, "model-x") {
		t.Fatalf("unexpected config show output: %s", got)
	}
}

func TestSkillsMemoryPrivacyRateLimitAndUpgradeCommands(t *testing.T) {
	t.Setenv("OPENHARNESS_CONFIG_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_MODEL", "")
	t.Setenv("OPENHARNESS_MODEL", "")

	registry := CreateDefaultRegistry()
	cwd := t.TempDir()
	ctx := Context{Engine: engine.NewQueryEngine(engine.QueryContext{}), CWD: cwd}

	cfgDir, err := config.ConfigDir()
	if err != nil {
		t.Fatalf("config dir failed: %v", err)
	}
	userSkillsDir := filepath.Join(cfgDir, "skills")
	if err := os.MkdirAll(userSkillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userSkillsDir, "demo.md"), []byte("# Demo\nSkill content line."), 0o644); err != nil {
		t.Fatalf("write skill failed: %v", err)
	}

	skillsListCmd, _ := registry.Lookup("/skills list")
	listResult := skillsListCmd.Handler("list", ctx)
	if !strings.Contains(listResult.Message, "Demo") {
		t.Fatalf("unexpected skills list output: %s", listResult.Message)
	}
	skillsShowCmd, _ := registry.Lookup("/skills show Demo")
	showResult := skillsShowCmd.Handler("show Demo", ctx)
	if !strings.Contains(showResult.Message, "Skill content line.") {
		t.Fatalf("unexpected skills show output: %s", showResult.Message)
	}

	memoryDir, err := projectMemoryDir(cwd)
	if err != nil {
		t.Fatalf("project memory dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memoryDir, "note.md"), []byte("remember this"), 0o644); err != nil {
		t.Fatalf("write memory file failed: %v", err)
	}
	memoryListCmd, _ := registry.Lookup("/memory list")
	memoryListResult := memoryListCmd.Handler("list", ctx)
	if !strings.Contains(memoryListResult.Message, "note.md") {
		t.Fatalf("unexpected memory list output: %s", memoryListResult.Message)
	}
	memoryShowCmd, _ := registry.Lookup("/memory show note.md")
	memoryShowResult := memoryShowCmd.Handler("show note.md", ctx)
	if !strings.Contains(memoryShowResult.Message, "remember this") {
		t.Fatalf("unexpected memory show output: %s", memoryShowResult.Message)
	}

	privacyCmd, _ := registry.Lookup("/privacy-settings")
	privacyResult := privacyCmd.Handler("", ctx)
	if !strings.Contains(privacyResult.Message, "Privacy settings:") || !strings.Contains(privacyResult.Message, "session_dir") {
		t.Fatalf("unexpected privacy-settings output: %s", privacyResult.Message)
	}

	rateCmd, _ := registry.Lookup("/rate-limit-options")
	rateResult := rateCmd.Handler("", ctx)
	if !strings.Contains(rateResult.Message, "Rate limit options:") || !strings.Contains(rateResult.Message, "provider") {
		t.Fatalf("unexpected rate-limit-options output: %s", rateResult.Message)
	}

	upgradeCmd, _ := registry.Lookup("/upgrade")
	upgradeResult := upgradeCmd.Handler("", ctx)
	if !strings.Contains(upgradeResult.Message, "Upgrade instructions:") {
		t.Fatalf("unexpected upgrade output: %s", upgradeResult.Message)
	}
}

func TestOutputStyleAuthAndReleaseNotesCommands(t *testing.T) {
	t.Setenv("OPENHARNESS_CONFIG_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_MODEL", "")
	t.Setenv("OPENHARNESS_MODEL", "")

	registry := CreateDefaultRegistry()
	cwd := t.TempDir()
	ctx := Context{Engine: engine.NewQueryEngine(engine.QueryContext{}), CWD: cwd}

	styleCmd, _ := registry.Lookup("/output-style set compact")
	if got := styleCmd.Handler("set compact", ctx).Message; !strings.Contains(got, "Output style set to compact") {
		t.Fatalf("unexpected output-style set output: %s", got)
	}
	styleShowCmd, _ := registry.Lookup("/output-style show")
	if got := styleShowCmd.Handler("show", ctx).Message; !strings.Contains(got, "compact") {
		t.Fatalf("unexpected output-style show output: %s", got)
	}

	loginCmd, _ := registry.Lookup("/login sk-test")
	if got := loginCmd.Handler("sk-test", ctx).Message; !strings.Contains(got, "Stored API key") {
		t.Fatalf("unexpected login output: %s", got)
	}
	loginShowCmd, _ := registry.Lookup("/login show")
	if got := loginShowCmd.Handler("show", ctx).Message; !strings.Contains(got, "Auth status:") || !strings.Contains(got, "provider:") || !strings.Contains(got, "configured") {
		t.Fatalf("unexpected login show output: %s", got)
	}

	logoutCmd, _ := registry.Lookup("/logout")
	if got := logoutCmd.Handler("", ctx).Message; !strings.Contains(got, "cleared") {
		t.Fatalf("unexpected logout output: %s", got)
	}

	if err := os.WriteFile(filepath.Join(cwd, "RELEASE_NOTES.md"), []byte("# Notes\nhello"), 0o644); err != nil {
		t.Fatalf("write release notes failed: %v", err)
	}
	releaseCmd, _ := registry.Lookup("/release-notes")
	if got := releaseCmd.Handler("", ctx).Message; !strings.Contains(got, "# Notes") {
		t.Fatalf("unexpected release-notes output: %s", got)
	}

	configShowCmd, _ := registry.Lookup("/config show")
	configOut := configShowCmd.Handler("show", ctx).Message
	if !strings.Contains(configOut, "\"model\"") || !strings.Contains(configOut, "\"permission\"") {
		t.Fatalf("expected json config output, got: %s", configOut)
	}
	configSetBoolCmd, _ := registry.Lookup("/config set vim_mode true")
	if got := configSetBoolCmd.Handler("set vim_mode true", ctx).Message; !strings.Contains(got, "Updated config vim_mode") {
		t.Fatalf("unexpected config bool set output: %s", got)
	}
}

func TestMemoryWriteCommands(t *testing.T) {
	t.Setenv("OPENHARNESS_CONFIG_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_MODEL", "")
	t.Setenv("OPENHARNESS_MODEL", "")

	registry := CreateDefaultRegistry()
	cwd := t.TempDir()
	ctx := Context{Engine: engine.NewQueryEngine(engine.QueryContext{}), CWD: cwd}

	addCmd, _ := registry.Lookup("/memory add Important Note :: keep this")
	addResult := addCmd.Handler("add Important Note :: keep this", ctx)
	if !strings.Contains(addResult.Message, "Added memory entry") {
		t.Fatalf("unexpected memory add output: %s", addResult.Message)
	}

	showCmd, _ := registry.Lookup("/memory show important_note")
	showResult := showCmd.Handler("show important_note", ctx)
	if !strings.Contains(showResult.Message, "keep this") {
		t.Fatalf("unexpected memory show output: %s", showResult.Message)
	}

	removeCmd, _ := registry.Lookup("/memory remove important_note")
	removeResult := removeCmd.Handler("remove important_note", ctx)
	if !strings.Contains(removeResult.Message, "Removed memory entry") {
		t.Fatalf("unexpected memory remove output: %s", removeResult.Message)
	}
}

func TestTasksMutatingCommands(t *testing.T) {
	registry := CreateDefaultRegistry()
	cwd := t.TempDir()
	ctx := Context{Engine: engine.NewQueryEngine(engine.QueryContext{}), CWD: cwd}

	runCmd, _ := registry.Lookup("/tasks run echo hello")
	runResult := runCmd.Handler("run echo hello", ctx)
	if !strings.Contains(runResult.Message, "Started task ") {
		t.Fatalf("unexpected tasks run output: %s", runResult.Message)
	}
	taskID := strings.TrimPrefix(runResult.Message, "Started task ")

	updateDescCmd, _ := registry.Lookup("/tasks update " + taskID + " description new desc")
	if got := updateDescCmd.Handler("update "+taskID+" description new desc", ctx).Message; !strings.Contains(got, "description") {
		t.Fatalf("unexpected tasks update description output: %s", got)
	}

	updateProgressCmd, _ := registry.Lookup("/tasks update " + taskID + " progress 42")
	if got := updateProgressCmd.Handler("update "+taskID+" progress 42", ctx).Message; !strings.Contains(got, "42%") {
		t.Fatalf("unexpected tasks update progress output: %s", got)
	}

	updateNoteCmd, _ := registry.Lookup("/tasks update " + taskID + " note watching")
	if got := updateNoteCmd.Handler("update "+taskID+" note watching", ctx).Message; !strings.Contains(got, "note") {
		t.Fatalf("unexpected tasks update note output: %s", got)
	}

	stopCmd, _ := registry.Lookup("/tasks stop " + taskID)
	if got := stopCmd.Handler("stop "+taskID, ctx).Message; !strings.Contains(got, "Stopped task ") {
		t.Fatalf("unexpected tasks stop output: %s", got)
	}
}

func TestCopyExportAndShareCommands(t *testing.T) {
	t.Setenv("OPENHARNESS_CONFIG_DIR", t.TempDir())
	t.Setenv("OPENHARNESS_DATA_DIR", t.TempDir())

	registry := CreateDefaultRegistry()
	cwd := t.TempDir()
	qe := engine.NewQueryEngine(engine.QueryContext{})
	qe.LoadMessages([]engine.ConversationMessage{{Role: "user", Text: "hello"}, {Role: "assistant", Text: "latest response"}})
	ctx := Context{Engine: qe, CWD: cwd}

	copyCmd, _ := registry.Lookup("/copy")
	copyResult := copyCmd.Handler("", ctx)
	if !strings.Contains(copyResult.Message, "Saved copied text") {
		t.Fatalf("unexpected copy output: %s", copyResult.Message)
	}

	exportCmd, _ := registry.Lookup("/export")
	exportResult := exportCmd.Handler("", ctx)
	if !strings.Contains(exportResult.Message, "Exported transcript to") {
		t.Fatalf("unexpected export output: %s", exportResult.Message)
	}

	shareCmd, _ := registry.Lookup("/share")
	shareResult := shareCmd.Handler("", ctx)
	if !strings.Contains(shareResult.Message, "Created shareable transcript snapshot at") {
		t.Fatalf("unexpected share output: %s", shareResult.Message)
	}
}

func TestRewindTagAndSessionExtendedCommands(t *testing.T) {
	t.Setenv("OPENHARNESS_CONFIG_DIR", t.TempDir())
	registry := CreateDefaultRegistry()
	cwd := t.TempDir()
	qe := engine.NewQueryEngine(engine.QueryContext{})
	qe.LoadMessages([]engine.ConversationMessage{{Role: "user", Text: "one"}, {Role: "assistant", Text: "a"}, {Role: "user", Text: "two"}, {Role: "assistant", Text: "b"}})
	ctx := Context{Engine: qe, CWD: cwd}

	rewindCmd, _ := registry.Lookup("/rewind 1")
	rewindResult := rewindCmd.Handler("1", ctx)
	if !strings.Contains(rewindResult.Message, "Rewound 1 turn") {
		t.Fatalf("unexpected rewind output: %s", rewindResult.Message)
	}

	tagCmd, _ := registry.Lookup("/tag release")
	tagResult := tagCmd.Handler("release", ctx)
	if !strings.Contains(tagResult.Message, "Tagged session as") {
		t.Fatalf("unexpected tag output: %s", tagResult.Message)
	}

	sessionPathCmd, _ := registry.Lookup("/session path")
	if got := sessionPathCmd.Handler("path", ctx).Message; strings.TrimSpace(got) == "" {
		t.Fatalf("unexpected session path output: %q", got)
	}

	sessionClearCmd, _ := registry.Lookup("/session clear")
	if got := sessionClearCmd.Handler("clear", ctx).Message; !strings.Contains(got, "Cleared session storage") {
		t.Fatalf("unexpected session clear output: %s", got)
	}
}

func TestIssuePrCommentsFeedbackOnboardingAndCommitCommands(t *testing.T) {
	t.Setenv("OPENHARNESS_CONFIG_DIR", t.TempDir())
	t.Setenv("OPENHARNESS_DATA_DIR", t.TempDir())
	registry := CreateDefaultRegistry()
	cwd := t.TempDir()
	ctx := Context{Engine: engine.NewQueryEngine(engine.QueryContext{}), CWD: cwd}

	issueSetCmd, _ := registry.Lookup("/issue set Title :: Body")
	if got := issueSetCmd.Handler("set Title :: Body", ctx).Message; !strings.Contains(got, "Saved issue context") {
		t.Fatalf("unexpected issue set output: %s", got)
	}
	issueShowCmd, _ := registry.Lookup("/issue show")
	if got := issueShowCmd.Handler("show", ctx).Message; !strings.Contains(got, "# Title") {
		t.Fatalf("unexpected issue show output: %s", got)
	}

	prAddCmd, _ := registry.Lookup("/pr_comments add file.go:10 :: looks good")
	if got := prAddCmd.Handler("add file.go:10 :: looks good", ctx).Message; !strings.Contains(got, "Added PR comment") {
		t.Fatalf("unexpected pr_comments add output: %s", got)
	}
	prShowCmd, _ := registry.Lookup("/pr_comments show")
	if got := prShowCmd.Handler("show", ctx).Message; !strings.Contains(got, "file.go:10") {
		t.Fatalf("unexpected pr_comments show output: %s", got)
	}

	feedbackCmd, _ := registry.Lookup("/feedback hello")
	if got := feedbackCmd.Handler("hello", ctx).Message; !strings.Contains(got, "Saved feedback") {
		t.Fatalf("unexpected feedback output: %s", got)
	}

	onboardingCmd, _ := registry.Lookup("/onboarding")
	if got := onboardingCmd.Handler("", ctx).Message; !strings.Contains(got, "OpenHarness quickstart") {
		t.Fatalf("unexpected onboarding output: %s", got)
	}

	commitCmd, _ := registry.Lookup("/commit")
	if got := commitCmd.Handler("", ctx).Message; strings.TrimSpace(got) == "" {
		t.Fatalf("unexpected commit output: %q", got)
	}
}

func TestKeybindingsAndInitCommands(t *testing.T) {
	t.Setenv("OPENHARNESS_CONFIG_DIR", t.TempDir())
	registry := CreateDefaultRegistry()
	cwd := t.TempDir()
	ctx := Context{Engine: engine.NewQueryEngine(engine.QueryContext{}), CWD: cwd}

	keybindingsCmd, _ := registry.Lookup("/keybindings")
	defaultOut := keybindingsCmd.Handler("", ctx).Message
	if !strings.Contains(defaultOut, "Keybindings file:") || !strings.Contains(defaultOut, "ctrl+l -> clear") {
		t.Fatalf("unexpected keybindings default output: %s", defaultOut)
	}

	cfgDir, err := config.ConfigDir()
	if err != nil {
		t.Fatalf("config dir failed: %v", err)
	}
	customPath := filepath.Join(cfgDir, "keybindings.json")
	if err := os.WriteFile(customPath, []byte(`{"ctrl+l":"status","ctrl+x":"exit"}`), 0o644); err != nil {
		t.Fatalf("write keybindings override failed: %v", err)
	}
	overrideOut := keybindingsCmd.Handler("", ctx).Message
	if !strings.Contains(overrideOut, "ctrl+l -> status") || !strings.Contains(overrideOut, "ctrl+x -> exit") {
		t.Fatalf("unexpected keybindings override output: %s", overrideOut)
	}

	initCmd, _ := registry.Lookup("/init")
	initResult := initCmd.Handler("", ctx)
	if !strings.Contains(initResult.Message, "Initialized project files:") {
		t.Fatalf("unexpected init output: %s", initResult.Message)
	}
	for _, rel := range []string{"CLAUDE.md", filepath.Join(".openharness", "README.md"), filepath.Join(".openharness", "memory", "MEMORY.md")} {
		if _, err := os.Stat(filepath.Join(cwd, rel)); err != nil {
			t.Fatalf("expected initialized file missing: %s (%v)", rel, err)
		}
	}

	initAgain := initCmd.Handler("", ctx)
	if !strings.Contains(initAgain.Message, "already initialized") {
		t.Fatalf("unexpected second init output: %s", initAgain.Message)
	}
}

func TestPluginReloadAgentsAndBridgeCommands(t *testing.T) {
	t.Setenv("OPENHARNESS_CONFIG_DIR", t.TempDir())
	registry := CreateDefaultRegistry()
	cwd := t.TempDir()
	ctx := Context{Engine: engine.NewQueryEngine(engine.QueryContext{}), CWD: cwd}

	pluginRoot := filepath.Join(cwd, ".openharness", "plugins", "demo")
	if err := os.MkdirAll(pluginRoot, 0o755); err != nil {
		t.Fatalf("mkdir plugin root failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "plugin.json"), []byte(`{"name":"demo","enabled_by_default":true}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest failed: %v", err)
	}

	pluginListCmd, _ := registry.Lookup("/plugin list")
	if got := pluginListCmd.Handler("list", ctx).Message; !strings.Contains(got, "demo") {
		t.Fatalf("unexpected plugin list output: %s", got)
	}

	pluginDisableCmd, _ := registry.Lookup("/plugin disable demo")
	if got := pluginDisableCmd.Handler("disable demo", ctx).Message; !strings.Contains(got, "Disabled plugin demo") {
		t.Fatalf("unexpected plugin disable output: %s", got)
	}

	reloadCmd, _ := registry.Lookup("/reload-plugins")
	if got := reloadCmd.Handler("", ctx).Message; !strings.Contains(got, "Reloaded plugins") {
		t.Fatalf("unexpected reload-plugins output: %s", got)
	}

	agentRecord := tasks.DefaultManager().CreateAgentTask("do work", "agent demo", cwd, "claude-sonnet")

	agentsCmd, _ := registry.Lookup("/agents")
	if got := agentsCmd.Handler("", ctx).Message; !strings.Contains(got, agentRecord.ID) {
		t.Fatalf("unexpected agents list output: %s", got)
	}
	agentShowCmd, _ := registry.Lookup("/agents show " + agentRecord.ID)
	if got := agentShowCmd.Handler("show "+agentRecord.ID, ctx).Message; !strings.Contains(got, "metadata=") {
		t.Fatalf("unexpected agent show output: %s", got)
	}

	bridgeCmd, _ := registry.Lookup("/bridge")
	if got := bridgeCmd.Handler("", ctx).Message; !strings.Contains(got, "Bridge summary") {
		t.Fatalf("unexpected bridge output: %s", got)
	}

	bridgeEncodeCmd, _ := registry.Lookup("/bridge encode https://api.example.com token")
	encoded := strings.TrimSpace(bridgeEncodeCmd.Handler("encode https://api.example.com token", ctx).Message)
	if encoded == "" {
		t.Fatalf("expected encoded bridge secret")
	}

	bridgeDecodeCmd, _ := registry.Lookup("/bridge decode " + encoded)
	if got := bridgeDecodeCmd.Handler("decode "+encoded, ctx).Message; !strings.Contains(got, "session_ingress_token") || !strings.Contains(got, "token") {
		t.Fatalf("unexpected bridge decode output: %s", got)
	}

	bridgeSDKCmd, _ := registry.Lookup("/bridge sdk https://api.example.com sid")
	if got := bridgeSDKCmd.Handler("sdk https://api.example.com sid", ctx).Message; !strings.Contains(got, "/session_ingress/ws/sid") {
		t.Fatalf("unexpected bridge sdk output: %s", got)
	}

	bridgeSpawnCmd, _ := registry.Lookup("/bridge spawn echo bridge-ok")
	spawnOut := bridgeSpawnCmd.Handler("spawn echo bridge-ok", ctx).Message
	if !strings.Contains(spawnOut, "Spawned bridge session") {
		t.Fatalf("unexpected bridge spawn output: %s", spawnOut)
	}
	parts := strings.Fields(spawnOut)
	if len(parts) < 4 {
		t.Fatalf("unexpected bridge spawn response format: %s", spawnOut)
	}
	sessionID := parts[3]

	bridgeListCmd, _ := registry.Lookup("/bridge list")
	if got := bridgeListCmd.Handler("list", ctx).Message; !strings.Contains(got, sessionID) {
		t.Fatalf("unexpected bridge list output: %s", got)
	}

	bridgeOutputCmd, _ := registry.Lookup("/bridge output " + sessionID)
	output := ""
	for i := 0; i < 20; i++ {
		output = bridgeOutputCmd.Handler("output "+sessionID, ctx).Message
		if strings.Contains(output, "bridge-ok") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(output, "bridge-ok") {
		t.Fatalf("unexpected bridge output: %s", output)
	}

	bridgeStopCmd, _ := registry.Lookup("/bridge stop " + sessionID)
	if got := bridgeStopCmd.Handler("stop "+sessionID, ctx).Message; !strings.Contains(got, "Stopped bridge session") {
		t.Fatalf("unexpected bridge stop output: %s", got)
	}
}
