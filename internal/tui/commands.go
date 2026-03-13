package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mistakeknot/Skaffen/internal/command"
	"github.com/mistakeknot/Skaffen/internal/session"
	"github.com/mistakeknot/Skaffen/internal/skill"
	msettings "github.com/mistakeknot/Masaq/settings"
)

// Command represents a parsed slash command.
type Command struct {
	Name string
	Args []string
}

// CommandResult is the output of executing a command.
type CommandResult struct {
	Message string
	IsError bool
	Quit    bool   // true for /quit — signals tea.Quit
	Retry   string // non-empty = re-submit this text as a prompt
}

// ParseCommand parses a slash command from input text.
// Returns nil if the input is not a slash command.
func ParseCommand(input string) *Command {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return nil
	}
	parts := strings.Fields(input[1:])
	if len(parts) == 0 {
		return nil
	}
	return &Command{
		Name: parts[0],
		Args: parts[1:],
	}
}

// KnownCommands returns the list of supported slash commands with descriptions.
func KnownCommands() map[string]string {
	return map[string]string{
		"advance":  "Advance to next OODARC phase",
		"clear":    "Clear viewport",
		"commit":   "Auto-commit current changes",
		"compact":  "Compact context (free tokens by summarizing old turns)",
		"diff":     "Show git diff",
		"help":     "Show available commands (? for keyboard shortcuts)",
		"history":  "Show recent prompt history",
		"model":    "Show or switch model (opus, sonnet, haiku)",
		"phase":    "Show current OODARC phase",
		"plan":     "Toggle plan mode (read-only tools only)",
		"quit":     "Exit Skaffen",
		"retry":    "Re-submit the last prompt",
		"sessions": "List saved sessions",
		"settings": "Show or change settings",
		"ship":     "Push changes to remote",
		"skills":   "List, inspect, pin, and manage skills",
		"status":   "Show session status summary",
		"theme":    "Switch theme (e.g. /theme catppuccin)",
		"undo":     "Undo the last git commit",
		"verbose":  "Switch to verbose tool call display",
		"version":  "Show version info",
	}
}

// FormatHelp renders the help text for all commands in sorted order.
func FormatHelp() string {
	return formatHelpWithCustom(nil)
}

// formatHelpWithCustom renders help text for built-in + custom commands.
func formatHelpWithCustom(custom map[string]command.Def) string {
	cmds := KnownCommands()
	for name, def := range custom {
		if _, exists := cmds[name]; !exists {
			cmds[name] = def.Description
		}
	}
	names := make([]string, 0, len(cmds))
	for name := range cmds {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("Available commands:\n")
	for _, name := range names {
		b.WriteString(fmt.Sprintf("  /%s — %s\n", name, cmds[name]))
	}
	return b.String()
}

// formatHelp renders help text for built-in + custom commands + skills.
func (m *appModel) formatHelp() string {
	cmds := KnownCommands()
	for name, def := range m.customCmds {
		if _, exists := cmds[name]; !exists {
			cmds[name] = def.Description
		}
	}
	names := make([]string, 0, len(cmds))
	for name := range cmds {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("Available commands:\n")
	for _, name := range names {
		b.WriteString(fmt.Sprintf("  /%s — %s\n", name, cmds[name]))
	}

	// Skills section
	var skillNames []string
	for name, d := range m.skills {
		if d.UserInvocable {
			skillNames = append(skillNames, name)
		}
	}
	if len(skillNames) > 0 {
		sort.Strings(skillNames)
		b.WriteString("\nSkills:\n")
		for _, name := range skillNames {
			d := m.skills[name]
			b.WriteString(fmt.Sprintf("  /%s — %s [%s]\n", name, d.Description, d.Source))
		}
	}

	return b.String()
}

// executeCommand runs a slash command and returns the result.
func (m *appModel) executeCommand(cmd *Command) CommandResult {
	switch cmd.Name {
	case "help":
		return CommandResult{Message: m.formatHelp()}

	case "quit":
		return CommandResult{Message: "Goodbye!", Quit: true}

	case "clear":
		m.viewport.SetContent("")
		return CommandResult{Message: "Viewport cleared."}

	case "compact":
		return m.execCompact()

	case "verbose":
		m.compact.SetVerbose(true)
		m.settings.Verbose = true
		return CommandResult{Message: "Switched to verbose display."}

	case "model":
		return m.execModel(cmd.Args)

	case "version":
		return CommandResult{Message: fmt.Sprintf("Skaffen %s / Masaq %s", m.skaffenVer, m.masaqVer)}

	case "status":
		return m.execStatus()

	case "phase":
		if m.agent == nil {
			return CommandResult{Message: fmt.Sprintf("Phase: %s (no agent)", m.phase)}
		}
		return CommandResult{Message: fmt.Sprintf("Phase: %s", m.agent.CurrentPhase())}

	case "plan":
		if m.agent == nil {
			return CommandResult{Message: "No agent configured.", IsError: true}
		}
		on := !m.agent.PlanMode()
		m.agent.SetPlanMode(on)
		if on {
			return CommandResult{Message: "Plan mode enabled — read-only tools only. Use /plan to toggle off."}
		}
		return CommandResult{Message: "Plan mode disabled — full tools available."}

	case "advance":
		if m.agent == nil {
			return CommandResult{Message: "No agent configured.", IsError: true}
		}
		old := string(m.agent.CurrentPhase())
		if err := m.agent.AdvancePhase(); err != nil {
			return CommandResult{
				Message: fmt.Sprintf("Cannot advance: %v", err),
				IsError: true,
			}
		}
		newPhase := string(m.agent.CurrentPhase())
		m.phase = newPhase
		return CommandResult{
			Message: PhaseTransition(old, newPhase),
		}

	case "settings":
		return m.execSettings(cmd.Args)

	case "theme":
		if len(cmd.Args) == 0 {
			return m.execSettings([]string{"theme"})
		}
		return m.execSettings([]string{"theme", cmd.Args[0]})

	case "diff":
		return m.execGitDiff()

	case "undo":
		return m.execGitUndo()

	case "commit":
		return m.execGitCommit(cmd.Args)

	case "ship":
		return m.execGitShip()

	case "history":
		return m.execHistory(cmd.Args)

	case "retry":
		if m.lastPrompt == "" {
			return CommandResult{Message: "No previous prompt to retry.", IsError: true}
		}
		return CommandResult{
			Message: fmt.Sprintf("Retrying: %s", truncate(m.lastPrompt, 60)),
			Retry:   m.lastPrompt,
		}

	case "sessions":
		return m.execListSessions()

	case "skills":
		return m.execSkills(cmd.Args)

	default:
		// Check custom commands loaded from disk
		if def, ok := m.customCmds[cmd.Name]; ok {
			return m.execCustomCommand(def, cmd.Args)
		}
		// Check skills
		if sd, ok := m.skills[strings.ToLower(cmd.Name)]; ok {
			return m.execSkill(sd, cmd.Args)
		}
		return CommandResult{
			Message: fmt.Sprintf("Unknown command /%s. Type /help for available commands.", cmd.Name),
			IsError: true,
		}
	}
}

// execCustomCommand dispatches a disk-based custom command.
func (m *appModel) execCustomCommand(def command.Def, args []string) CommandResult {
	switch def.Type {
	case command.TypeTemplate:
		// Template commands inject text as a user prompt.
		// The caller (Update) handles submitting the returned message as input.
		return CommandResult{Message: def.Template}
	case command.TypeScript:
		// Script commands run via shell with a timeout.
		ctx, cancel := context.WithTimeout(context.Background(), shellTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "bash", "-c", def.Script)
		cmd.Dir = m.workDir
		out, err := cmd.CombinedOutput()

		output := string(out)
		if len(output) > shellMaxOutput {
			output = output[:shellMaxOutput] + "\n... (truncated)"
		}
		if ctx.Err() == context.DeadlineExceeded {
			return CommandResult{
				Message: fmt.Sprintf("/%s timed out after %s:\n%s", def.Name, shellTimeout, output),
				IsError: true,
			}
		}
		if err != nil {
			return CommandResult{
				Message: fmt.Sprintf("/%s failed: %v\n%s", def.Name, err, output),
				IsError: true,
			}
		}
		if output == "" {
			return CommandResult{Message: fmt.Sprintf("/%s completed (no output).", def.Name)}
		}
		return CommandResult{Message: output}
	default:
		return CommandResult{
			Message: fmt.Sprintf("Unknown command type %q for /%s", def.Type, def.Name),
			IsError: true,
		}
	}
}

// execSkill activates a skill via slash command invocation.
// The skill body is loaded, formatted, and queued as a pending injection
// for the next agent call.
func (m *appModel) execSkill(d skill.Def, args []string) CommandResult {
	// Handle --pin flag
	pin := false
	filteredArgs := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--pin" {
			pin = true
		} else {
			filteredArgs = append(filteredArgs, a)
		}
	}

	// Lazy-load body
	body, err := skill.LoadBody(&d)
	if err != nil {
		return CommandResult{
			Message: fmt.Sprintf("Failed to load skill %q: %v", d.Name, err),
			IsError: true,
		}
	}

	// Update the cached def with the loaded body
	d.Body = body
	m.skills[d.Name] = d

	// Check size limit
	argStr := strings.Join(filteredArgs, " ")
	msg, err := skill.FormatInjectionSafe(&d, argStr)
	if err != nil {
		return CommandResult{Message: err.Error(), IsError: true}
	}

	// Pin if requested
	if pin {
		if err := m.pinner.Pin(d.Name); err != nil {
			return CommandResult{
				Message: fmt.Sprintf("Skill activated but pin failed: %v", err),
				IsError: true,
			}
		}
	}

	// Store the pending skill injection for the next runAgent call
	m.pendingSkills = append(m.pendingSkills, msg)

	status := fmt.Sprintf("[skill: %s]", d.Name)
	if pin {
		status += " (pinned)"
	}
	return CommandResult{Message: status}
}

// execSkills handles the /skills management command.
func (m *appModel) execSkills(args []string) CommandResult {
	if len(args) == 0 || args[0] == "list" {
		return m.execSkillsList()
	}
	switch args[0] {
	case "info":
		if len(args) < 2 {
			return CommandResult{Message: "Usage: /skills info <name>", IsError: true}
		}
		return m.execSkillsInfo(args[1])
	case "pin":
		if len(args) < 2 {
			return CommandResult{Message: "Usage: /skills pin <name>", IsError: true}
		}
		return m.execSkillsPin(args[1])
	case "unpin":
		if len(args) < 2 {
			return CommandResult{Message: "Usage: /skills unpin <name>", IsError: true}
		}
		m.pinner.Unpin(args[1])
		return CommandResult{Message: fmt.Sprintf("Unpinned skill %q.", args[1])}
	case "pinned":
		pinned := m.pinner.Pinned()
		if len(pinned) == 0 {
			return CommandResult{Message: "No pinned skills."}
		}
		return CommandResult{Message: "Pinned skills:\n  " + strings.Join(pinned, "\n  ")}
	default:
		return CommandResult{
			Message: "Usage: /skills [list|info <name>|pin <name>|unpin <name>|pinned]",
			IsError: true,
		}
	}
}

func (m *appModel) execSkillsList() CommandResult {
	if len(m.skills) == 0 {
		return CommandResult{Message: "No skills discovered."}
	}

	// Group by source tier
	groups := make(map[string][]skill.Def)
	for _, d := range m.skills {
		groups[d.Source] = append(groups[d.Source], d)
	}

	tierOrder := []struct{ key, label string }{
		{"project", "Project (.skaffen/skills/)"},
		{"project-plugin", "Project Plugins (.skaffen/plugins/*/skills/)"},
		{"user", "User (~/.skaffen/skills/)"},
		{"user-plugin", "User Plugins (~/.skaffen/plugins/*/skills/)"},
	}

	var b strings.Builder
	b.WriteString("Skills:\n")
	for _, tier := range tierOrder {
		defs, ok := groups[tier.key]
		if !ok || len(defs) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("\n  %s:\n", tier.label))
		sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
		for _, d := range defs {
			pinned := ""
			if m.pinner.IsPinned(d.Name) {
				pinned = " (pinned)"
			}
			b.WriteString(fmt.Sprintf("    /%s — %s%s\n", d.Name, d.Description, pinned))
		}
	}
	return CommandResult{Message: b.String()}
}

func (m *appModel) execSkillsInfo(name string) CommandResult {
	d, ok := m.skills[name]
	if !ok {
		return CommandResult{
			Message: fmt.Sprintf("Skill %q not found.", name),
			IsError: true,
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Skill: %s\n", d.Name))
	b.WriteString(fmt.Sprintf("  Description: %s\n", d.Description))
	b.WriteString(fmt.Sprintf("  Source: %s\n", d.Source))
	b.WriteString(fmt.Sprintf("  Invocable: %v\n", d.UserInvocable))
	if len(d.Triggers) > 0 {
		b.WriteString(fmt.Sprintf("  Triggers: %s\n", strings.Join(d.Triggers, ", ")))
	}
	if d.Args != "" {
		b.WriteString(fmt.Sprintf("  Args: %s\n", d.Args))
	}
	if d.Model != "" {
		b.WriteString(fmt.Sprintf("  Model: %s\n", d.Model))
	}
	b.WriteString(fmt.Sprintf("  Path: %s\n", d.Path))

	// Body preview (first 3 lines)
	body, err := skill.LoadBody(&d)
	if err == nil && body != "" {
		m.skills[name] = d // cache loaded body
		lines := strings.SplitN(body, "\n", 4)
		if len(lines) > 3 {
			lines = lines[:3]
		}
		b.WriteString("  Preview:\n")
		for _, line := range lines {
			b.WriteString(fmt.Sprintf("    %s\n", line))
		}
	}

	return CommandResult{Message: b.String()}
}

func (m *appModel) execSkillsPin(name string) CommandResult {
	if err := m.pinner.Pin(name); err != nil {
		return CommandResult{Message: err.Error(), IsError: true}
	}
	return CommandResult{Message: fmt.Sprintf("Pinned skill %q for this session.", name)}
}

func (m *appModel) execStatus() CommandResult {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Phase: %s  Model: %s  Turns: %d\n", m.phase, m.modelName, m.turns))
	if m.contextPct > 0 {
		b.WriteString(fmt.Sprintf("Context: %.0f%%", m.contextPct))
	}
	if m.totalCost > 0 {
		if m.contextPct > 0 {
			b.WriteString(fmt.Sprintf("  Cost: $%.4f", m.totalCost))
		} else {
			b.WriteString(fmt.Sprintf("Cost: $%.4f", m.totalCost))
		}
	}
	if m.git != nil {
		branch, _ := m.git.CurrentBranch()
		if branch != "" {
			b.WriteString(fmt.Sprintf("\nBranch: %s", branch))
		}
		has, _ := m.git.HasChanges()
		if has {
			b.WriteString(" (dirty)")
		}
	}
	return CommandResult{Message: b.String()}
}

// validModels maps short aliases to display names for the /model command.
var validModels = map[string]string{
	"opus":   "claude-opus-4-6",
	"sonnet": "claude-sonnet-4-6",
	"haiku":  "claude-haiku-4-5-20251001",
}

func (m *appModel) execModel(args []string) CommandResult {
	// /model — show current
	if len(args) == 0 {
		return CommandResult{Message: fmt.Sprintf("Model: %s", m.modelName)}
	}

	alias := strings.ToLower(args[0])
	canonical, ok := validModels[alias]
	if !ok {
		names := make([]string, 0, len(validModels))
		for k := range validModels {
			names = append(names, k)
		}
		sort.Strings(names)
		return CommandResult{
			Message: fmt.Sprintf("Unknown model %q. Available: %s", args[0], strings.Join(names, ", ")),
			IsError: true,
		}
	}

	if m.agent == nil {
		return CommandResult{Message: "No agent configured.", IsError: true}
	}

	if !m.agent.SetModelOverride(alias) {
		return CommandResult{Message: "Router does not support model switching.", IsError: true}
	}

	m.modelName = alias
	return CommandResult{Message: fmt.Sprintf("Switched to %s (%s)", alias, canonical)}
}

func (m *appModel) execSettings(args []string) CommandResult {
	// /settings — open interactive overlay
	if len(args) == 0 {
		entries := buildSettingsEntries(&m.settings)
		m.settingsOverlay = msettings.New("Settings", entries).SetWidth(m.width)
		m.settingsOpen = true
		return CommandResult{} // no message — overlay renders in View
	}
	// /settings <key> — show one
	if len(args) == 1 {
		for _, e := range settingsRegistry {
			if e.Key == args[0] {
				return CommandResult{Message: fmt.Sprintf("%s = %s (%s)", e.Key, e.Get(&m.settings), e.Description)}
			}
		}
		return CommandResult{Message: fmt.Sprintf("Unknown setting %q. Type /settings for list.", args[0]), IsError: true}
	}
	// /settings <key> <value> — set
	msg, err := ApplySetting(&m.settings, args[0], args[1])
	if err != nil {
		return CommandResult{Message: err.Error(), IsError: true}
	}
	// Sync compact formatter when verbose setting changes
	if args[0] == "verbose" {
		m.compact.SetVerbose(m.settings.Verbose)
	}
	return CommandResult{Message: msg}
}

func (m *appModel) execGitDiff() CommandResult {
	if m.git == nil {
		return CommandResult{Message: "Git not available (no working directory).", IsError: true}
	}
	d, err := m.git.Diff()
	if err != nil {
		return CommandResult{Message: fmt.Sprintf("Git diff failed: %v", err), IsError: true}
	}
	if strings.TrimSpace(d) == "" {
		return CommandResult{Message: "No changes."}
	}
	return CommandResult{Message: d}
}

func (m *appModel) execGitUndo() CommandResult {
	if m.git == nil {
		return CommandResult{Message: "Git not available (no working directory).", IsError: true}
	}
	if err := m.git.Undo(); err != nil {
		return CommandResult{Message: fmt.Sprintf("Undo failed: %v", err), IsError: true}
	}
	msg, _ := m.git.LastCommitMessage()
	if msg != "" {
		return CommandResult{Message: fmt.Sprintf("Undid last commit. HEAD is now: %s", msg)}
	}
	return CommandResult{Message: "Undid last commit (changes kept staged)."}
}

func (m *appModel) execGitCommit(args []string) CommandResult {
	if m.git == nil {
		return CommandResult{Message: "Git not available (no working directory).", IsError: true}
	}
	has, err := m.git.HasChanges()
	if err != nil {
		return CommandResult{Message: fmt.Sprintf("Git error: %v", err), IsError: true}
	}
	if !has {
		return CommandResult{Message: "Nothing to commit (working directory clean)."}
	}
	message := strings.Join(args, " ")
	if message == "" {
		message = "skaffen: auto-commit"
	}
	if err := m.git.AutoCommit(message); err != nil {
		return CommandResult{Message: fmt.Sprintf("Commit failed: %v", err), IsError: true}
	}
	return CommandResult{Message: fmt.Sprintf("Committed: %s", message)}
}

func (m *appModel) execGitShip() CommandResult {
	if m.git == nil {
		return CommandResult{Message: "Git not available (no working directory).", IsError: true}
	}
	branch, _ := m.git.CurrentBranch()
	if err := m.git.Push(); err != nil {
		return CommandResult{Message: fmt.Sprintf("Push failed: %v", err), IsError: true}
	}
	if branch != "" {
		return CommandResult{Message: fmt.Sprintf("Pushed %s to origin.", branch)}
	}
	return CommandResult{Message: "Pushed to origin."}
}

func (m *appModel) execHistory(args []string) CommandResult {
	if m.historyStore == nil {
		return CommandResult{Message: "History not available.", IsError: true}
	}
	entries := m.historyStore.Search("")
	if len(entries) == 0 {
		return CommandResult{Message: "No history entries."}
	}
	limit := 20
	if len(entries) < limit {
		limit = len(entries)
	}
	var b strings.Builder
	b.WriteString("Recent prompts:\n")
	for i, e := range entries[:limit] {
		display := e
		if len(display) > 80 {
			display = display[:77] + "..."
		}
		b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, display))
	}
	if len(entries) > limit {
		b.WriteString(fmt.Sprintf("  ... %d more (Ctrl+R to search)\n", len(entries)-limit))
	}
	return CommandResult{Message: b.String()}
}

func (m *appModel) execListSessions() CommandResult {
	sessDir := filepath.Join(os.Getenv("HOME"), ".skaffen", "sessions")
	sessions, err := session.ListSessions(sessDir)
	if err != nil {
		return CommandResult{Message: fmt.Sprintf("Error listing sessions: %v", err), IsError: true}
	}
	if len(sessions) == 0 {
		return CommandResult{Message: "No saved sessions."}
	}
	var b strings.Builder
	b.WriteString("Saved sessions:\n")
	limit := 10
	if len(sessions) < limit {
		limit = len(sessions)
	}
	for _, si := range sessions[:limit] {
		b.WriteString(fmt.Sprintf("  %s — %s\n", si.ID, session.FormatSessionEntry(si)))
	}
	if len(sessions) > 10 {
		b.WriteString(fmt.Sprintf("  ... and %d more\n", len(sessions)-10))
	}
	b.WriteString("\nResume with: skaffen -r <session-id>")
	return CommandResult{Message: b.String()}
}

const compactKeepRecent = 4 // keep last 4 messages (2 turns) after compaction

func (m *appModel) execCompact() CommandResult {
	if m.session == nil {
		return CommandResult{Message: "No session available for compaction.", IsError: true}
	}
	beforeCount := m.session.MessageCount()
	if beforeCount <= compactKeepRecent+1 {
		return CommandResult{Message: fmt.Sprintf("Context is small (%d messages) — nothing to compact.", beforeCount)}
	}

	// Build a simple summary from the kept messages' context
	summary := fmt.Sprintf("Previous conversation had %d messages covering %d turns.", beforeCount, m.turns)
	before, after := m.session.Compact(summary, compactKeepRecent)

	beforePct := m.contextPct
	// Estimate new context %: rough proportional reduction
	if before > 0 {
		m.contextPct = m.contextPct * float64(after) / float64(before)
	}

	return CommandResult{
		Message: fmt.Sprintf(
			"Compacted: %d → %d messages (%.0f%% → ~%.0f%% context)",
			before, after, beforePct, m.contextPct,
		),
	}
}

// commandResultMsg wraps a CommandResult for the Bubble Tea message loop.
type commandResultMsg CommandResult

// runCommand returns a tea.Cmd that executes a slash command and sends the result.
func (m *appModel) runCommand(cmd *Command) tea.Cmd {
	result := m.executeCommand(cmd)
	return func() tea.Msg {
		return commandResultMsg(result)
	}
}

// ParseShellEscape checks if input starts with ! and returns the command.
// Returns ("", false) if not a shell escape.
// Returns ("", true) for bare "!" with no command.
func ParseShellEscape(input string) (string, bool) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "!") {
		return "", false
	}
	cmd := strings.TrimSpace(input[1:])
	return cmd, true
}

const (
	shellTimeout    = 30 * time.Second
	shellMaxOutput  = 10240 // 10KB
)

// shellResultMsg carries the result of a shell escape command.
type shellResultMsg struct {
	Command  string
	Output   string
	ExitCode int
	Err      error
	TimedOut bool
}

// runShellCommand executes a shell command and returns a tea.Cmd that sends the result.
func (m *appModel) runShellCommand(command string) tea.Cmd {
	workDir := m.workDir
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), shellTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "bash", "-c", command)
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()

		output := string(out)
		if len(output) > shellMaxOutput {
			output = output[:shellMaxOutput] + "\n... (truncated)"
		}

		exitCode := 0
		timedOut := ctx.Err() == context.DeadlineExceeded
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else if !timedOut {
				return shellResultMsg{Command: command, Err: err}
			}
		}

		return shellResultMsg{
			Command:  command,
			Output:   output,
			ExitCode: exitCode,
			TimedOut: timedOut,
		}
	}
}
