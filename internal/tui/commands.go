package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mistakeknot/Skaffen/internal/session"
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
	Quit    bool // true for /quit — signals tea.Quit
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
		"compact":  "Switch to compact tool call display",
		"diff":     "Show git diff",
		"help":     "Show available commands",
		"model":    "Show or switch model (opus, sonnet, haiku)",
		"phase":    "Show current OODARC phase",
		"quit":     "Exit Skaffen",
		"sessions": "List saved sessions",
		"settings": "Show or change settings",
		"ship":     "Push changes to remote",
		"status":   "Show session status summary",
		"theme":    "Switch theme (e.g. /theme catppuccin)",
		"undo":     "Undo the last git commit",
		"verbose":  "Switch to verbose tool call display",
		"version":  "Show version info",
	}
}

// FormatHelp renders the help text for all commands in sorted order.
func FormatHelp() string {
	cmds := KnownCommands()
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

// executeCommand runs a slash command and returns the result.
func (m *appModel) executeCommand(cmd *Command) CommandResult {
	switch cmd.Name {
	case "help":
		return CommandResult{Message: FormatHelp()}

	case "quit":
		return CommandResult{Message: "Goodbye!", Quit: true}

	case "clear":
		m.viewport.SetContent("")
		return CommandResult{Message: "Viewport cleared."}

	case "compact":
		m.compact.SetVerbose(false)
		m.settings.Verbose = false
		return CommandResult{Message: "Switched to compact display."}

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

	case "sessions":
		return m.execListSessions()

	default:
		return CommandResult{
			Message: fmt.Sprintf("Unknown command /%s. Type /help for available commands.", cmd.Name),
			IsError: true,
		}
	}
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

// commandResultMsg wraps a CommandResult for the Bubble Tea message loop.
type commandResultMsg CommandResult

// runCommand returns a tea.Cmd that executes a slash command and sends the result.
func (m *appModel) runCommand(cmd *Command) tea.Cmd {
	result := m.executeCommand(cmd)
	return func() tea.Msg {
		return commandResultMsg(result)
	}
}
