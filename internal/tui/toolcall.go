package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Skaffen/internal/trust"
	"github.com/mistakeknot/masaq/theme"
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
		blockStyle := lipgloss.NewStyle().Foreground(theme.Current().Semantic().Error.Color()).Bold(true)
		return ToolCallDecision{
			Allowed: false,
			Message: blockStyle.Render(fmt.Sprintf("Blocked: %s", toolName)),
		}

	case trust.Prompt:
		// Interactive approval is handled by the ToolApprover in the agent loop.
		// This display path only formats the message — actual gating happens
		// before execution via the channel-based approval in app.go.
		promptStyle := lipgloss.NewStyle().Foreground(theme.Current().Semantic().Warning.Color())
		notice := promptStyle.Render(fmt.Sprintf("Approval required: %s", toolName))
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
	c := theme.Current().Semantic()
	toolStyle := lipgloss.NewStyle().Foreground(c.Info.Color()).Bold(true)
	qStyle := lipgloss.NewStyle().Foreground(c.Fg.Color())

	return fmt.Sprintf(
		"%s %s\n%s",
		qStyle.Render("Allow"),
		toolStyle.Render(toolName+" "+summary),
		lipgloss.NewStyle().Foreground(c.Muted.Color()).Render("[y]es  [n]o  [a]lways  [s]ession"),
	)
}

// TrustLearnMsg is sent when the user approves a tool call with a scope.
type TrustLearnMsg struct {
	Pattern  string
	Decision trust.Decision
	Scope    trust.Scope
}
