package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Masaq/theme"
)

// MessageRole identifies who sent a message.
type MessageRole int

const (
	RoleUser MessageRole = iota
	RoleAssistant
	RoleSystem
	RoleTool
)

// ChatMessage represents a message in the conversation.
type ChatMessage struct {
	Role      MessageRole
	Content   string
	Timestamp time.Time
	ToolName  string
	IsError   bool
}

// RenderMessage formats a chat message for display.
func RenderMessage(msg ChatMessage, width int) string {
	c := theme.Current().Semantic()
	switch msg.Role {
	case RoleUser:
		headerStyle := lipgloss.NewStyle().
			Foreground(c.Primary.Color()).
			Bold(true)
		return fmt.Sprintf("\n%s\n%s\n", headerStyle.Render("You"), msg.Content)

	case RoleAssistant:
		headerStyle := lipgloss.NewStyle().
			Foreground(c.Secondary.Color()).
			Bold(true)
		return fmt.Sprintf("\n%s\n%s", headerStyle.Render("Skaffen"), msg.Content)

	case RoleSystem:
		style := lipgloss.NewStyle().
			Foreground(c.Muted.Color()).
			Italic(true)
		return style.Render(fmt.Sprintf("--- %s ---", msg.Content))

	case RoleTool:
		if msg.IsError {
			style := lipgloss.NewStyle().Foreground(c.Error.Color())
			return style.Render(fmt.Sprintf("[x] %s: %s", msg.ToolName, msg.Content))
		}
		style := lipgloss.NewStyle().Foreground(c.Info.Color())
		return style.Render(fmt.Sprintf("> %s", msg.Content))

	default:
		return msg.Content
	}
}
