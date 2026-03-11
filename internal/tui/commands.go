package tui

import (
	"fmt"
	"strings"
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
		"sessions": "List and resume sessions",
		"help":     "Show available commands",
		"quit":     "Exit Skaffen",
	}
}

// FormatHelp renders the help text for all commands.
func FormatHelp() string {
	var b strings.Builder
	b.WriteString("Available commands:\n")
	for name, desc := range KnownCommands() {
		b.WriteString(fmt.Sprintf("  /%s — %s\n", name, desc))
	}
	return b.String()
}
