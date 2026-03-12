package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/trust"
	"github.com/mistakeknot/masaq/compact"
	"github.com/mistakeknot/masaq/diff"
	"github.com/mistakeknot/masaq/keys"
	"github.com/mistakeknot/masaq/markdown"
	"github.com/mistakeknot/masaq/question"
	"github.com/mistakeknot/masaq/theme"
	"github.com/mistakeknot/masaq/viewport"
)

// Config holds TUI configuration.
type Config struct {
	Agent     *agent.Agent
	Trust     *trust.Evaluator
	SessionID string
	Verbose   bool
	WorkDir   string
}

// Run starts the TUI REPL.
func Run(cfg Config) error {
	m := newAppModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.program = p
	if m.agent != nil {
		// Wire StreamCallback → tea.Program.Send so agent events reach the
		// Bubble Tea message loop from the background goroutine.
		m.agent.SetStreamCallback(func(ev agent.StreamEvent) {
			p.Send(streamEventMsg(ev))
		})
		// Wire ToolApprover: for trust.Prompt decisions, block the agent
		// goroutine and show the question overlay in the TUI.
		if m.trust != nil {
			m.agent.SetToolApprover(func(toolName string, input json.RawMessage) bool {
				decision := m.trust.Evaluate(toolName, string(input))
				switch decision {
				case trust.Allow:
					return true
				case trust.Block:
					return false
				default: // trust.Prompt
					reply := make(chan bool, 1)
					p.Send(toolApprovalRequestMsg{
						ToolName: toolName,
						Input:    input,
						Reply:    reply,
					})
					return <-reply
				}
			})
		}
	}
	_, err := p.Run()
	return err
}

// streamEventMsg wraps a StreamEvent for the Bubble Tea message loop.
type streamEventMsg agent.StreamEvent

// agentDoneMsg signals the agent loop completed.
type agentDoneMsg struct {
	Response string
	Err      error
}

// submitMsg is sent when the user submits a prompt.
type submitMsg struct {
	Text string
}

// toolApprovalRequestMsg is sent from the agent goroutine when a tool call
// needs interactive approval. The goroutine blocks on Reply until the TUI
// sends back a decision.
type toolApprovalRequestMsg struct {
	ToolName string
	Input    json.RawMessage
	Reply    chan bool
}

type appModel struct {
	width, height int
	viewport      viewport.Model
	md            *markdown.Renderer
	compact       *compact.Formatter
	keys          keys.Map
	status        statusModel
	prompt        promptModel
	agent         *agent.Agent
	trust         *trust.Evaluator
	program       *tea.Program
	workDir       string
	phase         string
	turns         int
	totalCost     float64
	contextPct    float64
	modelName     string
	running       bool

	// Tool approval overlay
	approving     bool
	approvalQ     question.Model
	approvalReply chan bool
	approvalTool  string
}

func newAppModel(cfg Config) *appModel {
	vp := viewport.New(80, 20)
	cf := compact.New(80)
	if cfg.Verbose {
		cf.SetVerbose(true)
	}
	return &appModel{
		viewport:  vp,
		md:        markdown.New(80),
		compact:   cf,
		keys:      keys.NewDefault(keys.WithVim()),
		status:    newStatusModel(),
		prompt:    newPromptModel(),
		agent:     cfg.Agent,
		trust:     cfg.Trust,
		workDir:   cfg.WorkDir,
		phase:     "build",
		modelName: "claude",
	}
}

func (m *appModel) Init() tea.Cmd {
	return m.prompt.Init()
}

func (m *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Layout: status=1, prompt=3, rest=viewport
		vpHeight := m.height - 4 // 1 status + 3 prompt
		if vpHeight < 1 {
			vpHeight = 1
		}
		m.viewport.SetSize(m.width, vpHeight)
		m.md = markdown.New(m.width)
		m.compact = compact.New(m.width)
		m.status.width = m.width

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		// When showing approval overlay, delegate keys to question widget
		if m.approving {
			var cmd tea.Cmd
			m.approvalQ, cmd = m.approvalQ.Update(msg)
			cmds = append(cmds, cmd)
			break
		}
		// Delegate to prompt when not running
		if !m.running {
			var cmd tea.Cmd
			m.prompt, cmd = m.prompt.Update(msg)
			cmds = append(cmds, cmd)
		}
		// Always allow viewport scrolling
		vp, cmd := m.viewport.Update(msg)
		m.viewport = vp
		cmds = append(cmds, cmd)

	case submitMsg:
		if m.running {
			break
		}
		m.running = true
		// Render user message
		userStyle := lipgloss.NewStyle().Foreground(theme.Current().Semantic().Primary.Color()).Bold(true)
		m.viewport.AppendContent("\n" + userStyle.Render("You") + "\n" + msg.Text + "\n")
		// Start agent
		cmds = append(cmds, m.runAgent(msg.Text))

	case streamEventMsg:
		m.handleStreamEvent(agent.StreamEvent(msg))

	case toolApprovalRequestMsg:
		m.approving = true
		m.approvalReply = msg.Reply
		m.approvalTool = msg.ToolName
		summary := m.compact.FormatToolCall(msg.ToolName, string(msg.Input), "", false)
		m.viewport.AppendContent("\n" + summary)
		// Show diff preview for file-modifying tools
		if preview := renderDiffPreview(msg.ToolName, msg.Input, m.width); preview != "" {
			m.viewport.AppendContent("\n" + preview)
		}
		m.approvalQ = question.New(
			fmt.Sprintf("Allow %s?", msg.ToolName),
			[]question.Option{
				{Label: "Yes", Description: "allow this call"},
				{Label: "No", Description: "deny this call"},
				{Label: "Always", Description: "allow and remember for session"},
			},
		)

	case question.SelectedMsg:
		if !m.approving {
			break
		}
		m.approving = false
		allowed := msg.Index == 0 || msg.Index == 2 // Yes or Always
		if msg.Index == 2 && m.trust != nil {
			m.trust.Learn(m.approvalTool, trust.Allow, trust.ScopeSession)
		}
		m.approvalReply <- allowed
		m.approvalReply = nil

	case agentDoneMsg:
		m.running = false
		if msg.Err != nil {
			errStyle := lipgloss.NewStyle().Foreground(theme.Current().Semantic().Error.Color())
			m.viewport.AppendContent("\n" + errStyle.Render(fmt.Sprintf("Error: %v", msg.Err)) + "\n")
		}
		m.prompt.Reset()
	}

	return m, tea.Batch(cmds...)
}

func (m *appModel) View() string {
	vpView := m.viewport.View()
	statusView := m.status.View(m.phase, m.modelName, m.totalCost, m.contextPct, m.turns)

	if m.approving {
		return lipgloss.JoinVertical(lipgloss.Left, vpView, statusView, m.approvalQ.View())
	}

	promptView := m.prompt.View(m.width, m.running)
	return lipgloss.JoinVertical(lipgloss.Left, vpView, statusView, promptView)
}

func (m *appModel) handleStreamEvent(ev agent.StreamEvent) {
	switch ev.Type {
	case agent.StreamText:
		m.md.Append(ev.Text)
		m.viewport.AppendContent(m.md.View())
		m.md.Reset()
	case agent.StreamToolStart:
		summary := m.compact.FormatToolCall(ev.ToolName, ev.ToolParams, "", false)
		m.viewport.AppendContent("\n" + summary)
	case agent.StreamToolComplete:
		if ev.IsError {
			summary := m.compact.FormatToolCall(ev.ToolName, ev.ToolParams, ev.ToolResult, true)
			m.viewport.AppendContent("\n" + summary)
		}
	case agent.StreamTurnComplete:
		m.turns = ev.TurnNumber
		if ev.Usage.InputTokens > 0 {
			m.contextPct = float64(ev.Usage.InputTokens) / 200000.0 * 100
		}
	case agent.StreamPhaseChange:
		m.phase = ev.Phase
	}
}

func (m *appModel) runAgent(prompt string) tea.Cmd {
	a := m.agent
	return func() tea.Msg {
		if a == nil {
			return agentDoneMsg{Err: fmt.Errorf("no agent configured")}
		}
		// Runs in a goroutine. StreamCallback was wired in Run() to call
		// p.Send(streamEventMsg), so events reach Update() automatically.
		result, err := a.Run(context.Background(), prompt)
		if err != nil {
			return agentDoneMsg{Err: err}
		}
		return agentDoneMsg{Response: result.Response}
	}
}

// renderDiffPreview generates a diff preview for edit and write tool calls.
// Returns empty string for non-file-modifying tools or on any error.
func renderDiffPreview(toolName string, input json.RawMessage, width int) string {
	r := diff.New(width)

	switch toolName {
	case "edit":
		var p struct {
			FilePath  string `json:"file_path"`
			OldString string `json:"old_string"`
			NewString string `json:"new_string"`
		}
		if json.Unmarshal(input, &p) != nil || p.FilePath == "" {
			return ""
		}
		before, err := os.ReadFile(p.FilePath)
		if err != nil {
			return ""
		}
		after := strings.Replace(string(before), p.OldString, p.NewString, 1)
		return r.Render(string(before), after, p.FilePath)

	case "write":
		var p struct {
			FilePath string `json:"file_path"`
			Content  string `json:"content"`
		}
		if json.Unmarshal(input, &p) != nil || p.FilePath == "" {
			return ""
		}
		before, _ := os.ReadFile(p.FilePath) // may not exist (new file)
		return r.Render(string(before), p.Content, p.FilePath)

	default:
		return ""
	}
}
