package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mistakeknot/Skaffen/internal/session"
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
		"compact":  "Switch to compact tool call display",
		"verbose":  "Switch to verbose tool call display",
		"phase":    "Show current OODARC phase",
		"advance":  "Advance to next OODARC phase",
		"undo":     "Undo the last git commit",
		"commit":   "Auto-commit current changes",
		"ship":     "Push changes to remote",
		"sessions": "List saved sessions",
		"help":     "Show available commands",
		"quit":     "Exit Skaffen",
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
// This method lives on appModel so handlers can access TUI state (phase, compact, git, etc.).
func (m *appModel) executeCommand(cmd *Command) CommandResult {
	switch cmd.Name {
	case "help":
		return CommandResult{Message: FormatHelp()}

	case "quit":
		return CommandResult{Message: "Goodbye!", Quit: true}

	case "compact":
		m.compact.SetVerbose(false)
		return CommandResult{Message: "Switched to compact display."}

	case "verbose":
		m.compact.SetVerbose(true)
		return CommandResult{Message: "Switched to verbose display."}

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
// This allows slash command execution to follow the same Cmd pattern as agent runs.
type commandResultMsg CommandResult

// runCommand returns a tea.Cmd that executes a slash command and sends the result.
func (m *appModel) runCommand(cmd *Command) tea.Cmd {
	result := m.executeCommand(cmd)
	return func() tea.Msg {
		return commandResultMsg(result)
	}
}
