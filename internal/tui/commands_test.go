package tui

import (
	"strings"
	"testing"

	"github.com/mistakeknot/Masaq/compact"
	"github.com/mistakeknot/Masaq/viewport"
	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/command"
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/session"
)

func TestParseSlashCommand(t *testing.T) {
	tests := []struct {
		input string
		cmd   string
		args  []string
	}{
		{"/compact", "compact", nil},
		{"/verbose", "verbose", nil},
		{"/phase", "phase", nil},
		{"/undo", "undo", nil},
		{"/help", "help", nil},
		{"/sessions", "sessions", nil},
		{"/commit fix typo", "commit", []string{"fix", "typo"}},
		{"/settings verbose on", "settings", []string{"verbose", "on"}},
		{"/theme catppuccin", "theme", []string{"catppuccin"}},
		{"not a command", "", nil},
		{"", "", nil},
		{"/", "", nil},
	}
	for _, tt := range tests {
		cmd := ParseCommand(tt.input)
		if tt.cmd == "" {
			if cmd != nil {
				t.Errorf("ParseCommand(%q) = %+v, want nil", tt.input, cmd)
			}
			continue
		}
		if cmd == nil {
			t.Errorf("ParseCommand(%q) = nil, want %q", tt.input, tt.cmd)
			continue
		}
		if cmd.Name != tt.cmd {
			t.Errorf("ParseCommand(%q).Name = %q, want %q", tt.input, cmd.Name, tt.cmd)
		}
		if tt.args != nil && len(cmd.Args) != len(tt.args) {
			t.Errorf("ParseCommand(%q).Args = %v, want %v", tt.input, cmd.Args, tt.args)
		}
	}
}

func TestKnownCommands(t *testing.T) {
	cmds := KnownCommands()
	required := []string{
		"advance", "clear", "commit", "compact", "continue", "diff", "help",
		"model", "phase", "quit", "retry", "sessions", "settings", "ship",
		"status", "theme", "undo", "verbose", "version",
	}
	for _, name := range required {
		if _, ok := cmds[name]; !ok {
			t.Errorf("missing required command %q", name)
		}
	}
}

func TestFormatHelp(t *testing.T) {
	help := FormatHelp()
	if help == "" {
		t.Fatal("help should not be empty")
	}
	if !strings.Contains(help, "/compact") {
		t.Fatal("help should mention /compact")
	}
	if !strings.Contains(help, "/settings") {
		t.Fatal("help should mention /settings")
	}
	if !strings.Contains(help, "/theme") {
		t.Fatal("help should mention /theme")
	}
}

func TestFormatHelpSorted(t *testing.T) {
	help := FormatHelp()
	lines := strings.Split(strings.TrimSpace(help), "\n")
	var cmdLines []string
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) != "" {
			cmdLines = append(cmdLines, line)
		}
	}
	for i := 1; i < len(cmdLines); i++ {
		if cmdLines[i] < cmdLines[i-1] {
			t.Errorf("commands not sorted: %q should come before %q", cmdLines[i], cmdLines[i-1])
		}
	}
}

func newTestModel() *appModel {
	return &appModel{
		compact:    compact.New(80),
		viewport:   viewport.New(80, 20),
		session:    session.New("test", "", "", 20),
		settings:   defaultSettings(),
		phase:      "build",
		modelName:  "opus",
		skaffenVer: "0.2.0",
		masaqVer:   "0.1.0",
	}
}

func TestExecuteCommand_Help(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "help"})
	if result.IsError {
		t.Fatal("help should not be an error")
	}
	if !strings.Contains(result.Message, "/help") {
		t.Fatal("help output should list commands")
	}
}

func TestExecuteCommand_Quit(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "quit"})
	if !result.Quit {
		t.Fatal("quit should set Quit=true")
	}
}

func TestExecuteCommand_Clear(t *testing.T) {
	m := newTestModel()
	m.viewport.AppendContent("some content")
	result := m.executeCommand(&Command{Name: "clear"})
	if result.IsError {
		t.Fatal("clear should not error")
	}
}

func TestExecuteCommand_CompactSmallContext(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "compact"})
	if result.IsError {
		t.Fatalf("compact on small context should not error: %s", result.Message)
	}
	if !strings.Contains(result.Message, "nothing to compact") {
		t.Fatalf("should say nothing to compact, got: %s", result.Message)
	}
}

func TestExecuteCommand_CompactLargeContext(t *testing.T) {
	m := newTestModel()
	m.contextPct = 75.0
	m.turns = 10
	// Add enough messages to trigger compaction
	for i := 0; i < 20; i++ {
		m.session.Save(agent.Turn{
			Messages: []provider.Message{
				{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "hello"}}},
				{Role: provider.RoleAssistant, Content: []provider.ContentBlock{{Type: "text", Text: "world"}}},
			},
		})
	}
	before := m.session.MessageCount()
	result := m.executeCommand(&Command{Name: "compact"})
	if result.IsError {
		t.Fatalf("compact should not error: %s", result.Message)
	}
	after := m.session.MessageCount()
	if after >= before {
		t.Fatalf("compact should reduce messages: before=%d, after=%d", before, after)
	}
	if !strings.Contains(result.Message, "Compacted") {
		t.Fatalf("should report compaction, got: %s", result.Message)
	}
}

func TestExecuteCommand_CompactNoSession(t *testing.T) {
	m := newTestModel()
	m.session = nil
	result := m.executeCommand(&Command{Name: "compact"})
	if !result.IsError {
		t.Fatal("compact without session should be an error")
	}
}

func TestExecuteCommand_Verbose(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "verbose"})
	if result.IsError {
		t.Fatalf("verbose should not error: %s", result.Message)
	}
	if !m.compact.IsVerbose() {
		t.Fatal("verbose should have set verbose=true on formatter")
	}
	if !m.settings.Verbose {
		t.Fatal("verbose should have set settings.Verbose=true")
	}
}

func TestExecuteCommand_ModelShow(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "model"})
	if result.IsError {
		t.Fatal("model should not error")
	}
	if !strings.Contains(result.Message, "opus") {
		t.Fatalf("model should show model name, got: %s", result.Message)
	}
}

func TestExecuteCommand_ModelSwitchNoAgent(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "model", Args: []string{"sonnet"}})
	if !result.IsError {
		t.Fatal("model switch without agent should be an error")
	}
}

func TestExecuteCommand_ModelInvalid(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "model", Args: []string{"gpt-4"}})
	if !result.IsError {
		t.Fatal("invalid model should be an error")
	}
	if !strings.Contains(result.Message, "Unknown model") {
		t.Fatalf("should mention unknown model, got: %s", result.Message)
	}
	if !strings.Contains(result.Message, "haiku") {
		t.Fatalf("should list available models, got: %s", result.Message)
	}
}

func TestExecuteCommand_Version(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "version"})
	if result.IsError {
		t.Fatal("version should not error")
	}
	if !strings.Contains(result.Message, "0.2.0") {
		t.Fatal("version should show skaffen version")
	}
	if !strings.Contains(result.Message, "0.1.0") {
		t.Fatal("version should show masaq version")
	}
}

func TestExecuteCommand_Status(t *testing.T) {
	m := newTestModel()
	m.turns = 5
	result := m.executeCommand(&Command{Name: "status"})
	if result.IsError {
		t.Fatal("status should not error")
	}
	if !strings.Contains(result.Message, "build") {
		t.Fatal("status should show phase")
	}
	if !strings.Contains(result.Message, "5") {
		t.Fatal("status should show turn count")
	}
}

func TestExecuteCommand_PhaseNoAgent(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "phase"})
	if result.IsError {
		t.Fatal("phase should not error without agent")
	}
	if !strings.Contains(result.Message, "build") {
		t.Fatal("phase should show current phase")
	}
}

func TestExecuteCommand_AdvanceNoAgent(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "advance"})
	if !result.IsError {
		t.Fatal("advance without agent should be an error")
	}
}

func TestExecuteCommand_SettingsOpenOverlay(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "settings"})
	if result.IsError {
		t.Fatal("settings should not error")
	}
	if result.Message != "" {
		t.Fatalf("settings with no args should return empty message (overlay opens), got: %q", result.Message)
	}
	if !m.settingsOpen {
		t.Fatal("settingsOpen should be true after /settings")
	}
	// Verify overlay contains settings entries
	entries := m.settingsOverlay.Entries()
	if len(entries) != len(settingsRegistry) {
		t.Fatalf("overlay entries = %d, want %d", len(entries), len(settingsRegistry))
	}
	// Check known keys
	keys := make(map[string]bool)
	for _, e := range entries {
		keys[e.Key] = true
	}
	for _, key := range []string{"verbose", "theme", "color-mode"} {
		if !keys[key] {
			t.Errorf("overlay should contain %q entry", key)
		}
	}
}

func TestExecuteCommand_SettingsShowOne(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "settings", Args: []string{"verbose"}})
	if result.IsError {
		t.Fatal("settings verbose should not error")
	}
	if !strings.Contains(result.Message, "off") {
		t.Fatalf("verbose should be off by default, got: %s", result.Message)
	}
}

func TestExecuteCommand_SettingsSet(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "settings", Args: []string{"show-tool-results", "on"}})
	if result.IsError {
		t.Fatalf("settings set should not error: %s", result.Message)
	}
	if !m.settings.ShowToolResults {
		t.Fatal("show-tool-results should be on")
	}
}

func TestExecuteCommand_SettingsSetVerboseSyncs(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "settings", Args: []string{"verbose", "on"}})
	if result.IsError {
		t.Fatalf("settings set verbose should not error: %s", result.Message)
	}
	if !m.compact.IsVerbose() {
		t.Fatal("setting verbose via /settings should sync to compact formatter")
	}
}

func TestExecuteCommand_SettingsUnknown(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "settings", Args: []string{"bogus"}})
	if !result.IsError {
		t.Fatal("unknown setting should be an error")
	}
}

func TestExecuteCommand_ThemeNoArgs(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "theme"})
	if result.IsError {
		t.Fatal("theme with no args should show current theme")
	}
	if !strings.Contains(result.Message, "theme") {
		t.Fatal("should mention theme")
	}
}

func TestExecuteCommand_ThemeSwitch(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "theme", Args: []string{"catppuccin"}})
	if result.IsError {
		t.Fatalf("theme catppuccin should not error: %s", result.Message)
	}
	if m.settings.Theme != "Catppuccin" {
		t.Fatalf("theme should be Catppuccin, got: %s", m.settings.Theme)
	}
}

func TestExecuteCommand_ThemeInvalid(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "theme", Args: []string{"nonexistent"}})
	if !result.IsError {
		t.Fatal("invalid theme should be an error")
	}
	if !strings.Contains(result.Message, "unknown theme") {
		t.Fatalf("should mention unknown theme, got: %s", result.Message)
	}
}

func TestExecuteCommand_UndoNoGit(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "undo"})
	if !result.IsError {
		t.Fatal("undo without git should be an error")
	}
}

func TestExecuteCommand_CommitNoGit(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "commit"})
	if !result.IsError {
		t.Fatal("commit without git should be an error")
	}
}

func TestExecuteCommand_ShipNoGit(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "ship"})
	if !result.IsError {
		t.Fatal("ship without git should be an error")
	}
}

func TestExecuteCommand_DiffNoGit(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "diff"})
	if !result.IsError {
		t.Fatal("diff without git should be an error")
	}
}

func TestExecuteCommand_Unknown(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "bogus"})
	if !result.IsError {
		t.Fatal("unknown command should be an error")
	}
	if !strings.Contains(result.Message, "Unknown command") {
		t.Fatal("should mention unknown command")
	}
}

func TestParseShellEscape(t *testing.T) {
	tests := []struct {
		input   string
		wantCmd string
		wantOk  bool
	}{
		{"!ls", "ls", true},
		{"!git status", "git status", true},
		{"! git status", "git status", true},  // space after !
		{"!", "", true},                        // bare !
		{"!!double", "!double", true},          // extra ! preserved
		{"/help", "", false},                   // slash command, not shell
		{"hello", "", false},                   // plain text
		{"  !pwd  ", "pwd", true},              // leading/trailing whitespace
		{"", "", false},                        // empty
	}
	for _, tt := range tests {
		cmd, ok := ParseShellEscape(tt.input)
		if ok != tt.wantOk {
			t.Errorf("ParseShellEscape(%q): ok = %v, want %v", tt.input, ok, tt.wantOk)
		}
		if cmd != tt.wantCmd {
			t.Errorf("ParseShellEscape(%q): cmd = %q, want %q", tt.input, cmd, tt.wantCmd)
		}
	}
}

func TestExecuteCustomCommand_Template(t *testing.T) {
	m := newTestModel()
	m.customCmds = map[string]command.Def{
		"review": {
			Name:        "review",
			Description: "Review code",
			Type:        command.TypeTemplate,
			Template:    "Please review the code.",
			Source:      "user",
		},
	}
	result := m.executeCommand(&Command{Name: "review"})
	if result.IsError {
		t.Fatalf("template command should not error: %s", result.Message)
	}
	if result.Message != "Please review the code." {
		t.Errorf("Message = %q, want template text", result.Message)
	}
}

func TestExecuteCustomCommand_Script(t *testing.T) {
	m := newTestModel()
	m.workDir = t.TempDir()
	m.customCmds = map[string]command.Def{
		"greet": {
			Name:        "greet",
			Description: "Say hello",
			Type:        command.TypeScript,
			Script:      "echo hello world",
			Source:      "project",
		},
	}
	result := m.executeCommand(&Command{Name: "greet"})
	if result.IsError {
		t.Fatalf("script command should not error: %s", result.Message)
	}
	if !strings.Contains(result.Message, "hello world") {
		t.Errorf("Message = %q, want 'hello world'", result.Message)
	}
}

func TestExecuteCustomCommand_ScriptError(t *testing.T) {
	m := newTestModel()
	m.workDir = t.TempDir()
	m.customCmds = map[string]command.Def{
		"fail": {
			Name:   "fail",
			Type:   command.TypeScript,
			Script: "exit 1",
			Source:  "user",
		},
	}
	result := m.executeCommand(&Command{Name: "fail"})
	if !result.IsError {
		t.Fatal("failing script should return error")
	}
}

func TestExecuteCustomCommand_UnknownFallback(t *testing.T) {
	m := newTestModel()
	m.customCmds = map[string]command.Def{}
	result := m.executeCommand(&Command{Name: "nonexistent"})
	if !result.IsError {
		t.Fatal("unknown command should return error")
	}
	if !strings.Contains(result.Message, "Unknown command") {
		t.Errorf("Message = %q, want unknown command error", result.Message)
	}
}

func TestHelpIncludesCustomCommands(t *testing.T) {
	m := newTestModel()
	m.customCmds = map[string]command.Def{
		"review": {
			Name:        "review",
			Description: "Review changes",
			Type:        command.TypeTemplate,
			Template:    "review",
			Source:      "user",
		},
	}
	result := m.executeCommand(&Command{Name: "help"})
	if !strings.Contains(result.Message, "/review") {
		t.Error("help should include custom command /review")
	}
	if !strings.Contains(result.Message, "Review changes") {
		t.Error("help should include custom command description")
	}
}

func TestCustomCommandDoesNotOverrideBuiltin(t *testing.T) {
	m := newTestModel()
	m.customCmds = map[string]command.Def{
		"help": {
			Name:     "help",
			Type:     command.TypeTemplate,
			Template: "custom help",
			Source:   "user",
		},
	}
	result := m.executeCommand(&Command{Name: "help"})
	// Built-in help should win since it's in the switch/case
	if !strings.Contains(result.Message, "Available commands") {
		t.Error("built-in /help should take precedence over custom /help")
	}
}

func TestExecuteCommand_RetryNoHistory(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "retry"})
	if !result.IsError {
		t.Fatal("retry with no previous prompt should be an error")
	}
	if !strings.Contains(result.Message, "No previous prompt") {
		t.Fatalf("should mention no previous prompt, got: %s", result.Message)
	}
}

func TestExecuteCommand_RetryWithHistory(t *testing.T) {
	m := newTestModel()
	m.lastPrompt = "explain the code"
	result := m.executeCommand(&Command{Name: "retry"})
	if result.IsError {
		t.Fatalf("retry with history should not error: %s", result.Message)
	}
	if result.Retry != "explain the code" {
		t.Fatalf("Retry = %q, want 'explain the code'", result.Retry)
	}
	if !strings.Contains(result.Message, "Retrying") {
		t.Fatalf("should mention retrying, got: %s", result.Message)
	}
}

func TestExecuteCommand_RetryTruncatesLongPrompt(t *testing.T) {
	m := newTestModel()
	m.lastPrompt = strings.Repeat("a", 100)
	result := m.executeCommand(&Command{Name: "retry"})
	if result.IsError {
		t.Fatal("retry should not error")
	}
	// Message should be truncated for display but Retry should have full text
	if result.Retry != m.lastPrompt {
		t.Fatal("Retry field should contain the full prompt")
	}
	if len(result.Message) > 80 {
		t.Fatal("displayed message should be truncated")
	}
}

func TestExecuteCommand_ContinueNoHistory(t *testing.T) {
	m := newTestModel()
	result := m.executeCommand(&Command{Name: "continue"})
	if !result.IsError {
		t.Fatal("continue with no previous prompt should be an error")
	}
	if !strings.Contains(result.Message, "No previous prompt") {
		t.Fatalf("should mention no previous prompt, got: %s", result.Message)
	}
}

func TestExecuteCommand_ContinueWithHistory(t *testing.T) {
	m := newTestModel()
	m.lastPrompt = "explain the code"
	result := m.executeCommand(&Command{Name: "continue"})
	if result.IsError {
		t.Fatalf("continue should not error: %s", result.Message)
	}
	if !strings.Contains(result.Retry, "explain the code") {
		t.Fatal("Retry should contain original prompt")
	}
	if !strings.Contains(result.Retry, "continue") {
		t.Fatal("Retry should contain continuation instruction")
	}
	if !strings.Contains(result.Message, "Continuing") {
		t.Fatalf("should mention continuing, got: %s", result.Message)
	}
}

func TestTruncate(t *testing.T) {
	if truncate("short", 10) != "short" {
		t.Fatal("short string should not be truncated")
	}
	if truncate("hello world", 8) != "hello..." {
		t.Fatalf("truncate = %q, want 'hello...'", truncate("hello world", 8))
	}
}

func TestCompleterIncludesCustomCommands(t *testing.T) {
	custom := map[string]command.Def{
		"deploy": {
			Name:        "deploy",
			Description: "Deploy to prod",
			Type:        command.TypeScript,
			Script:      "deploy.sh",
			Source:      "project",
		},
	}
	cc := newCmdCompleter(custom, nil)
	found := false
	for _, e := range cc.commands {
		if e.name == "deploy" {
			found = true
			if e.desc != "Deploy to prod" {
				t.Errorf("deploy desc = %q, want 'Deploy to prod'", e.desc)
			}
			break
		}
	}
	if !found {
		t.Error("completer should include custom command 'deploy'")
	}
}
