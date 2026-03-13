package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/command"
	"github.com/mistakeknot/Skaffen/internal/git"
	"github.com/mistakeknot/Skaffen/internal/skill"
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/session"
	"github.com/mistakeknot/Skaffen/internal/subagent"
	"github.com/mistakeknot/Skaffen/internal/trust"
	"github.com/mistakeknot/Masaq/breadcrumb"
	"github.com/mistakeknot/Masaq/compact"
	"github.com/mistakeknot/Masaq/diff"
	"github.com/mistakeknot/Masaq/keys"
	"github.com/mistakeknot/Masaq/markdown"
	"github.com/mistakeknot/Masaq/question"
	msettings "github.com/mistakeknot/Masaq/settings"
	"github.com/mistakeknot/Masaq/statusbar"
	"github.com/mistakeknot/Masaq/theme"
	"github.com/mistakeknot/Masaq/viewport"
)

// Config holds TUI configuration.
type Config struct {
	Agent      *agent.Agent
	Trust      *trust.Evaluator
	Session    *session.JSONLSession // for context compaction
	SessionID  string
	Verbose    bool
	WorkDir        string
	SkaffenVer     string
	MasaqVer       string
	CustomCommands map[string]command.Def
	Skills         map[string]skill.Def
	SubagentInit   *SubagentInit // nil = subagents disabled
}

// SubagentInit carries the components needed to wire the subagent runner
// once the TUI program is running and can receive Bubble Tea messages.
type SubagentInit struct {
	AgentTool   *subagent.AgentTool
	Registry    *subagent.TypeRegistry
	Provider    provider.Provider
	Reservation *subagent.ReservationBridge
}

// subagentStatusMsg wraps a subagent.StatusUpdate for the Bubble Tea message loop.
type subagentStatusMsg subagent.StatusUpdate

// Run starts the TUI REPL.
func Run(cfg Config) error {
	m := newAppModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.program = p
	// Wire subagent runner now that we have the tea.Program for sending messages
	if cfg.SubagentInit != nil {
		si := cfg.SubagentInit
		runner := subagent.NewRunner(si.Registry, si.Provider, si.Reservation, subagent.RunnerConfig{
			MaxConcurrent: 5,
			StatusCB: func(u subagent.StatusUpdate) {
				p.Send(subagentStatusMsg(u))
			},
		})
		si.AgentTool.SetRunner(runner)
		m.subagents = newSubagentTracker()
	}
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
	status        statusbar.Model
	crumbs        breadcrumb.Model
	prompt        promptModel
	agent         *agent.Agent
	trust         *trust.Evaluator
	session       *session.JSONLSession
	git           *git.Git
	program       *tea.Program
	workDir       string
	settings      settings
	skaffenVer    string
	masaqVer      string
	phase         string
	turns         int
	totalCost     float64
	contextPct    float64
	modelName     string
	running       bool

	// Logo animation (renders at top of viewport, animates until first keypress)
	logo logoModel

	// Tool approval overlay
	approving     bool
	approvalQ     question.Model
	approvalReply chan bool
	approvalTool  string

	// Settings overlay
	settingsOpen    bool
	settingsOverlay msettings.Model

	// Custom slash commands loaded from disk
	customCmds map[string]command.Def

	// Skills loaded from SKILL.md files
	skills map[string]skill.Def
	pinner *skill.Pinner

	// Subagent tracker for inline status rendering
	subagents *subagentTracker
}

func newAppModel(cfg Config) *appModel {
	vp := viewport.New(80, 20)
	cf := compact.New(80)
	pm := newPromptModel()
	pm.workDir = cfg.WorkDir
	pm.customCmds = cfg.CustomCommands
	// Initialize git helper if workDir is a git repo
	var g *git.Git
	if cfg.WorkDir != "" {
		g = git.New(cfg.WorkDir)
		if !g.IsRepo() {
			g = nil
		}
	}
	skVer := cfg.SkaffenVer
	if skVer == "" {
		skVer = "dev"
	}
	mqVer := cfg.MasaqVer
	if mqVer == "" {
		mqVer = "dev"
	}
	s := defaultSettings()
	if cfg.Verbose {
		s.Verbose = true
	}
	bc := breadcrumb.New(80)
	bc.SetSteps([]breadcrumb.Step{
		{Label: "brainstorm", Status: breadcrumb.Pending},
		{Label: "plan", Status: breadcrumb.Pending},
		{Label: "build", Status: breadcrumb.Active},
		{Label: "review", Status: breadcrumb.Pending},
		{Label: "ship", Status: breadcrumb.Pending},
	})
	return &appModel{
		viewport:   vp,
		md:         markdown.New(80),
		compact:    cf,
		keys:       keys.NewDefault(keys.WithVim()),
		status:     newStatusBar(80),
		crumbs:     bc,
		prompt:     pm,
		agent:      cfg.Agent,
		trust:      cfg.Trust,
		session:    cfg.Session,
		git:        g,
		workDir:    cfg.WorkDir,
		settings:   s,
		skaffenVer: skVer,
		masaqVer:   mqVer,
		phase:      "build",
		modelName:  "opus",
		logo:       newLogoModel(skVer, mqVer),
		customCmds: cfg.CustomCommands,
		skills:     cfg.Skills,
		pinner:     skill.NewPinner(cfg.Skills),
	}
}

func (m *appModel) Init() tea.Cmd {
	return tea.Batch(m.prompt.Init(), m.logo.tick())
}

func (m *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Layout: breadcrumb=1, status=1, prompt=3, rest=viewport
		vpHeight := m.height - 5 // 1 breadcrumb + 1 status + 3 prompt
		if vpHeight < 1 {
			vpHeight = 1
		}
		m.viewport.SetSize(m.width, vpHeight)
		m.md = markdown.New(m.width)
		m.compact = compact.New(m.width)
		m.status = newStatusBar(m.width)
		m.crumbs = breadcrumb.New(m.width)
		m.syncBreadcrumb()
		m.logo.width = m.width

	case logoTickMsg:
		if m.logo.active {
			m.logo.frame++
			m.logo.step()
			cmds = append(cmds, m.logo.tick())
		}

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.drainApproval()
			return m, tea.Quit
		}
		// Plan mode toggle: Shift+Tab toggles read-only mode (only when idle)
		if msg.String() == "shift+tab" && !m.running && !m.approving && !m.settingsOpen {
			on := !m.agent.PlanMode()
			m.agent.SetPlanMode(on)
			if on {
				m.viewport.AppendContent("\n" + lipgloss.NewStyle().Foreground(theme.Current().Semantic().Info.Color()).Render("Plan mode enabled — read-only tools only (Shift+Tab to toggle)") + "\n")
			} else {
				m.viewport.AppendContent("\n" + lipgloss.NewStyle().Foreground(theme.Current().Semantic().Success.Color()).Render("Plan mode disabled — full tools available") + "\n")
			}
			break
		}
		// Stop logo animation on first real typed character.
		// Ignore scroll keys, control sequences, and escape sequence
		// fragments (like termenv OSC query responses).
		if m.logo.active && isTypedChar(msg) {
			m.logo.stop()
		}
		// When showing approval overlay, delegate keys to question widget
		if m.approving {
			var cmd tea.Cmd
			m.approvalQ, cmd = m.approvalQ.Update(msg)
			cmds = append(cmds, cmd)
			break
		}
		// When showing settings overlay, delegate keys to settings widget
		if m.settingsOpen {
			var cmd tea.Cmd
			m.settingsOverlay, cmd = m.settingsOverlay.Update(msg)
			cmds = append(cmds, cmd)
			break
		}
		// Delegate to prompt (including file picker) when not running.
		// Block escape sequence fragments (mouse reports, OSC responses)
		// that arrive as KeyRunes from reaching the textinput.
		if !m.running && !isEscapeFragment(msg) {
			var cmd tea.Cmd
			m.prompt, cmd = m.prompt.Update(msg)
			cmds = append(cmds, cmd)
		}
		// Viewport scrolling: when prompt is focused, only pass through
		// dedicated scroll keys (PgUp/PgDn/Home/End/Ctrl+U/Ctrl+D) —
		// arrow keys belong to the textinput for cursor movement.
		if m.running || isScrollKey(msg) {
			vp, cmd := m.viewport.Update(msg)
			m.viewport = vp
			cmds = append(cmds, cmd)
		}

	case tea.MouseMsg:
		// Mouse wheel always goes to viewport
		vp, cmd := m.viewport.Update(msg)
		m.viewport = vp
		cmds = append(cmds, cmd)

	case submitMsg:
		if m.running {
			break
		}
		// Check for shell escape (! prefix) before slash commands
		if shellCmd, isShell := ParseShellEscape(msg.Text); isShell {
			c := theme.Current().Semantic()
			if shellCmd == "" {
				// Bare "!" — show usage help
				helpStyle := lipgloss.NewStyle().Foreground(c.FgDim.Color())
				m.viewport.AppendContent("\n" + helpStyle.Render("Usage: !<command> — run a shell command") + "\n")
				m.prompt.Reset()
				break
			}
			shellStyle := lipgloss.NewStyle().Foreground(c.FgDim.Color())
			m.viewport.AppendContent("\n" + shellStyle.Render("! "+shellCmd) + "\n")
			m.running = true
			cmds = append(cmds, m.runShellCommand(shellCmd))
			m.prompt.Reset()
			break
		}
		// Check for slash commands before sending to agent
		if cmd := ParseCommand(msg.Text); cmd != nil {
			cmdStyle := lipgloss.NewStyle().Foreground(theme.Current().Semantic().FgDim.Color())
			m.viewport.AppendContent("\n" + cmdStyle.Render("/"+cmd.Name) + "\n")
			cmds = append(cmds, m.runCommand(cmd))
			m.prompt.Reset()
			break
		}
		m.running = true
		// Render user message (original text with @mentions)
		userStyle := lipgloss.NewStyle().Foreground(theme.Current().Semantic().Primary.Color()).Bold(true)
		m.viewport.AppendContent("\n" + userStyle.Render("You") + "\n" + msg.Text + "\n")
		// Expand @file mentions before sending to agent
		expanded := expandAtMentions(msg.Text, m.workDir)
		cmds = append(cmds, m.runAgent(expanded))

	case streamEventMsg:
		m.handleStreamEvent(agent.StreamEvent(msg))

	case toolApprovalRequestMsg:
		m.approving = true
		m.approvalReply = msg.Reply
		m.approvalTool = msg.ToolName
		summary := m.compact.FormatToolCall(msg.ToolName, string(msg.Input), "", false)
		m.viewport.AppendContent("\n" + summary)
		// Show diff preview for file-modifying tools
		if m.settings.DiffPreview {
			if preview := renderDiffPreview(msg.ToolName, msg.Input, m.width); preview != "" {
				m.viewport.AppendContent("\n" + preview)
			}
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

	case msettings.ChangedMsg:
		if !m.settingsOpen {
			break
		}
		if _, err := ApplySetting(&m.settings, msg.Key, msg.NewValue); err != nil {
			// Revert entry in overlay to old value
			entries := m.settingsOverlay.Entries()
			for i, e := range entries {
				if e.Key == msg.Key {
					e.Value = msg.OldValue
					m.settingsOverlay = m.settingsOverlay.UpdateEntry(i, e)
					break
				}
			}
			break
		}
		// Sync side-effects for settings that affect other model state
		if msg.Key == "verbose" {
			m.compact.SetVerbose(m.settings.Verbose)
		}

	case msettings.DismissedMsg:
		m.settingsOpen = false

	case editorResultMsg:
		if msg.Err != nil {
			errStyle := lipgloss.NewStyle().Foreground(theme.Current().Semantic().Error.Color())
			m.viewport.AppendContent(errStyle.Render(fmt.Sprintf("Editor error: %v", msg.Err)) + "\n")
			break
		}
		if strings.TrimSpace(msg.Text) != "" {
			m.prompt.input.SetValue(msg.Text)
			m.prompt.input.CursorEnd()
		}

	case filePickerSelectedMsg, filePickerCancelMsg,
		cmdCompleterSelectedMsg, cmdCompleterCancelMsg:
		var cmd tea.Cmd
		m.prompt, cmd = m.prompt.Update(msg)
		cmds = append(cmds, cmd)

	case commandResultMsg:
		if msg.IsError {
			errStyle := lipgloss.NewStyle().Foreground(theme.Current().Semantic().Error.Color())
			m.viewport.AppendContent(errStyle.Render(msg.Message) + "\n")
		} else {
			m.viewport.AppendContent(msg.Message + "\n")
		}
		if msg.Quit {
			m.drainApproval()
			return m, tea.Quit
		}

	case shellResultMsg:
		m.running = false
		c := theme.Current().Semantic()
		if msg.Err != nil {
			errStyle := lipgloss.NewStyle().Foreground(c.Error.Color())
			m.viewport.AppendContent(errStyle.Render(fmt.Sprintf("Shell error: %v", msg.Err)) + "\n")
		} else {
			if msg.Output != "" {
				outputStyle := lipgloss.NewStyle().Foreground(c.FgDim.Color())
				m.viewport.AppendContent(outputStyle.Render(msg.Output))
			}
			if msg.TimedOut {
				warnStyle := lipgloss.NewStyle().Foreground(c.Warning.Color())
				m.viewport.AppendContent(warnStyle.Render("\n(timed out after 30s)") + "\n")
			} else if msg.ExitCode != 0 {
				errStyle := lipgloss.NewStyle().Foreground(c.Error.Color())
				m.viewport.AppendContent(errStyle.Render(fmt.Sprintf("\nexit code: %d", msg.ExitCode)) + "\n")
			}
		}
		m.prompt.Reset()

	case subagentStatusMsg:
		if m.subagents != nil {
			m.subagents.update(subagent.StatusUpdate(msg))
			// Re-render subagent tracker inline
			m.viewport.AppendContent("\r" + m.subagents.View(m.width) + "\n")
		}

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
	logoView := m.logo.View()
	vpView := m.viewport.View()

	// Update status slots with current state.
	planMode := m.agent != nil && m.agent.PlanMode()
	updateStatusSlots(&m.status, m.phase, m.modelName, m.totalCost, m.contextPct, m.turns, planMode)
	statusView := m.status.View()
	crumbView := m.crumbs.View()

	// Logo sits above viewport, taking space from viewport height
	logoHeight := strings.Count(logoView, "\n")
	vpHeight := m.height - 5 - logoHeight // 1 breadcrumb + 1 status + 3 prompt + logo
	if vpHeight < 1 {
		vpHeight = 1
	}
	if m.viewport.Height() != vpHeight {
		m.viewport.SetSize(m.width, vpHeight)
		vpView = m.viewport.View()
	}

	if m.approving {
		return lipgloss.JoinVertical(lipgloss.Left, logoView, vpView, m.approvalQ.View(), crumbView, statusView)
	}

	if m.settingsOpen {
		return lipgloss.JoinVertical(lipgloss.Left, logoView, vpView, m.settingsOverlay.View(), crumbView, statusView)
	}

	promptView := m.prompt.View(m.width, m.running)
	return lipgloss.JoinVertical(lipgloss.Left, logoView, vpView, promptView, crumbView, statusView)
}

// drainApproval sends false to a pending approval channel so the agent
// goroutine doesn't block forever on <-reply after p.Run() exits.
func (m *appModel) drainApproval() {
	if m.approving && m.approvalReply != nil {
		m.approvalReply <- false
		m.approvalReply = nil
		m.approving = false
	}
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
		if ev.IsError || m.settings.ShowToolResults {
			summary := m.compact.FormatToolCall(ev.ToolName, ev.ToolParams, ev.ToolResult, ev.IsError)
			m.viewport.AppendContent("\n" + summary)
		}
	case agent.StreamTurnComplete:
		m.turns = ev.TurnNumber
		if ev.Usage.InputTokens > 0 {
			m.contextPct = float64(ev.Usage.InputTokens) / 200000.0 * 100
		}
		// Auto-compact when context exceeds 80%
		if m.settings.AutoCompact && m.contextPct > 80 && m.session != nil {
			result := m.execCompact()
			if !result.IsError && result.Message != "" {
				c := theme.Current().Semantic()
				infoStyle := lipgloss.NewStyle().Foreground(c.FgDim.Color())
				m.viewport.AppendContent("\n" + infoStyle.Render("Auto-compact: "+result.Message) + "\n")
			}
		}
	case agent.StreamPhaseChange:
		m.phase = ev.Phase
		m.syncBreadcrumb()
	}
}

// syncBreadcrumb updates the breadcrumb trail to reflect the current OODARC phase.
// Phases before the current one are Done, the current is Active, the rest are Pending.
func (m *appModel) syncBreadcrumb() {
	current := -1
	for i, p := range phaseOrder {
		if p == m.phase {
			current = i
			break
		}
	}
	steps := make([]breadcrumb.Step, len(phaseOrder))
	for i, p := range phaseOrder {
		var s breadcrumb.Status
		switch {
		case current < 0:
			s = breadcrumb.Pending // unknown phase — all pending
		case i < current:
			s = breadcrumb.Done
		case i == current:
			s = breadcrumb.Active
		default:
			s = breadcrumb.Pending
		}
		steps[i] = breadcrumb.Step{Label: p, Status: s}
	}
	m.crumbs.SetSteps(steps)
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

// isScrollKey returns true for keys that should scroll the viewport even
// when the prompt has focus (i.e. keys that don't conflict with textinput).
func isScrollKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd, tea.KeyCtrlU, tea.KeyCtrlD:
		return true
	}
	return false
}

// isTypedChar returns true for actual user-typed printable characters.
// Returns false for control keys, escape sequences, function keys, and
// terminal query responses (like OSC 11 background color replies).
func isTypedChar(msg tea.KeyMsg) bool {
	if msg.Type != tea.KeyRunes {
		return false
	}
	s := msg.String()
	if isEscapeLike(s) {
		return false
	}
	return len(s) > 0
}

// isEscapeFragment returns true for KeyMsg events that look like terminal
// escape sequence fragments rather than real user input. This catches:
//   - SGR mouse reports: [<65;58;19M  (mouse wheel/click)
//   - OSC responses:     ]11;rgb:0000/0000/0000\
func isEscapeFragment(msg tea.KeyMsg) bool {
	if msg.Type != tea.KeyRunes {
		return false
	}
	return isEscapeLike(msg.String())
}

func isEscapeLike(s string) bool {
	// OSC responses contain ] ; and backslash
	if strings.ContainsAny(s, "\x1b];\\") {
		return true
	}
	// SGR mouse reports: [<NN;NN;NNM or [<NN;NN;NNm
	if len(s) > 3 && s[0] == '[' && s[1] == '<' {
		return true
	}
	return false
}

// atMentionRe matches @path tokens in user input.
var atMentionRe = regexp.MustCompile(`@([\w./_-][\w./_-]*)`)

const maxAtFileSize = 50 * 1024 // 50KB

// expandAtMentions replaces @path tokens with file content blocks.
// Paths are resolved relative to workDir. Files that don't exist or are
// too large are left as-is.
func expandAtMentions(text, workDir string) string {
	if !strings.Contains(text, "@") {
		return text
	}
	return atMentionRe.ReplaceAllStringFunc(text, func(match string) string {
		path := match[1:] // strip leading @
		fullPath := path
		if !filepath.IsAbs(path) && workDir != "" {
			fullPath = filepath.Join(workDir, path)
		}
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			return match // leave as-is
		}
		if info.Size() > maxAtFileSize {
			return match // too large, leave as-is
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return match
		}
		return fmt.Sprintf("[File: %s]\n%s\n[/File]", path, string(content))
	})
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
