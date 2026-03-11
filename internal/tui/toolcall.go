package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Skaffen/internal/trust"
)

// ToolCallDecision represents the result of evaluating a tool call through the trust system.
type ToolCallDecision struct {
	Allowed bool
	Message string // Styled message for the chat stream
}

// EvaluateToolCall checks a tool call against the trust evaluator and returns a display decision.
func EvaluateToolCall(eval *trust.Evaluator, toolName, paramsJSON string, compactFmt func(string, string, string, bool) string) ToolCallDecision {
	if eval == nil {
		// No trust evaluator — allow everything (headless mode)
		return ToolCallDecision{
			Allowed: true,
			Message: compactFmt(toolName, paramsJSON, "", false),
		}
	}

	decision := eval.Evaluate(toolName, paramsJSON)

	switch decision {
	case trust.Allow:
		return ToolCallDecision{
			Allowed: true,
			Message: compactFmt(toolName, paramsJSON, "", false),
		}

	case trust.Block:
		blockStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Bold(true)
		return ToolCallDecision{
			Allowed: false,
			Message: blockStyle.Render(fmt.Sprintf("Blocked: %s", toolName)),
		}

	case trust.Prompt:
		// For now, auto-allow with a notice. Full interactive approval
		// requires blocking the agent loop (future: channel-based approval).
		promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68"))
		notice := promptStyle.Render(fmt.Sprintf("Auto-allowed: %s (trust learning pending)", toolName))
		return ToolCallDecision{
			Allowed: true,
			Message: notice + "\n" + compactFmt(toolName, paramsJSON, "", false),
		}

	default:
		return ToolCallDecision{
			Allowed: true,
			Message: compactFmt(toolName, paramsJSON, "", false),
		}
	}
}

// FormatApprovalPrompt renders the approval question for interactive trust decisions.
// This will be used when the full interactive approval flow is implemented.
func FormatApprovalPrompt(toolName, summary string) string {
	toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7dcfff")).Bold(true)
	qStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#c0caf5"))

	return fmt.Sprintf(
		"%s %s\n%s",
		qStyle.Render("Allow"),
		toolStyle.Render(toolName+" "+summary),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Render("[y]es  [n]o  [a]lways  [s]ession"),
	)
}

// TrustLearnMsg is sent when the user approves a tool call with a scope.
type TrustLearnMsg struct {
	Pattern  string
	Decision trust.Decision
	Scope    trust.Scope
}
