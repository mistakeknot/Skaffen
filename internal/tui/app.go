package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/trust"
	"github.com/mistakeknot/masaq/compact"
	"github.com/mistakeknot/masaq/keys"
	"github.com/mistakeknot/masaq/markdown"
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
	workDir       string
	phase         string
	turns         int
	totalCost     float64
	contextPct    float64
	modelName     string
	running       bool
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
		userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Bold(true)
		m.viewport.AppendContent("\n" + userStyle.Render("You") + "\n" + msg.Text + "\n")
		// Start agent
		cmds = append(cmds, m.runAgent(msg.Text))

	case streamEventMsg:
		m.handleStreamEvent(agent.StreamEvent(msg))

	case agentDoneMsg:
		m.running = false
		if msg.Err != nil {
			errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
			m.viewport.AppendContent("\n" + errStyle.Render(fmt.Sprintf("Error: %v", msg.Err)) + "\n")
		}
		m.prompt.Reset()
	}

	return m, tea.Batch(cmds...)
}

func (m *appModel) View() string {
	vpView := m.viewport.View()
	statusView := m.status.View(m.phase, m.modelName, m.totalCost, m.contextPct, m.turns)
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
	return func() tea.Msg {
		// This runs in a goroutine. The agent streams events via callback
		// which must be funneled through the tea.Program via Send().
		// For now, since the program reference isn't wired, return a stub.
		_ = prompt
		return agentDoneMsg{Response: "Agent loop not wired yet"}
	}
}
