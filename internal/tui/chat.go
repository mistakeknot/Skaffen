package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
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
	switch msg.Role {
	case RoleUser:
		headerStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7aa2f7")).
			Bold(true)
		return fmt.Sprintf("\n%s\n%s\n", headerStyle.Render("You"), msg.Content)

	case RoleAssistant:
		headerStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#bb9af7")).
			Bold(true)
		return fmt.Sprintf("\n%s\n%s", headerStyle.Render("Skaffen"), msg.Content)

	case RoleSystem:
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89")).
			Italic(true)
		return style.Render(fmt.Sprintf("--- %s ---", msg.Content))

	case RoleTool:
		if msg.IsError {
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
			return style.Render(fmt.Sprintf("[x] %s: %s", msg.ToolName, msg.Content))
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#7dcfff"))
		return style.Render(fmt.Sprintf("> %s", msg.Content))

	default:
		return msg.Content
	}
}
