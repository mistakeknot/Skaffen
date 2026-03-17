package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Masaq/theme"
)

var sidebarTabs = []string{"Files", "Git", "Tools", "Debug"}

// trackedFile records a file touched during the session.
type trackedFile struct {
	Path     string
	Mutated  bool
	LastSeen time.Time
}

// toolCallEntry records a single tool invocation.
type toolCallEntry struct {
	Name     string
	Target   string
	Duration time.Duration
	At       time.Time
}

// sidebarModel is the right-side panel showing session context.
type sidebarModel struct {
	width, height int
	activeTab     int
	files         []trackedFile
	fileIndex     map[string]int // path → index in files
	toolCalls     []toolCallEntry
	mcpServers    []string
	phase         string
	turns         int
	tokens        int
	subagentCount int
	gitStatus     string
}

func newSidebarModel(width, height int) sidebarModel {
	return sidebarModel{
		width:     width,
		height:    height,
		fileIndex: make(map[string]int),
	}
}

// TrackFile records a file being read or mutated.
func (s *sidebarModel) TrackFile(path string, mutated bool) {
	if idx, ok := s.fileIndex[path]; ok {
		s.files[idx].LastSeen = time.Now()
		if mutated {
			s.files[idx].Mutated = true
		}
		return
	}
	s.fileIndex[path] = len(s.files)
	s.files = append(s.files, trackedFile{
		Path:     path,
		Mutated:  mutated,
		LastSeen: time.Now(),
	})
}

// AddToolCall records a tool invocation.
func (s *sidebarModel) AddToolCall(name, target string, durationMs int) {
	entry := toolCallEntry{
		Name:     name,
		Target:   target,
		Duration: time.Duration(durationMs) * time.Millisecond,
		At:       time.Now(),
	}
	s.toolCalls = append(s.toolCalls, entry)
	// Keep last 20
	if len(s.toolCalls) > 20 {
		s.toolCalls = s.toolCalls[len(s.toolCalls)-20:]
	}
}

// SetMCPServers updates the list of active MCP server names.
func (s *sidebarModel) SetMCPServers(servers []string) {
	s.mcpServers = servers
}

// SetDebugInfo updates debug state.
func (s *sidebarModel) SetDebugInfo(phase string, turns, tokens, subagents int) {
	s.phase = phase
	s.turns = turns
	s.tokens = tokens
	s.subagentCount = subagents
}

// SetGitStatus updates the git status text.
func (s *sidebarModel) SetGitStatus(status string) {
	s.gitStatus = status
}

// SetSize updates the sidebar dimensions.
func (s *sidebarModel) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s sidebarModel) Update(msg tea.Msg) (sidebarModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyTab {
			s.activeTab = (s.activeTab + 1) % len(sidebarTabs)
		}
		if msg.Type == tea.KeyShiftTab {
			s.activeTab = (s.activeTab - 1 + len(sidebarTabs)) % len(sidebarTabs)
		}
	}
	return s, nil
}

func (s sidebarModel) View() string {
	sem := theme.Current().Semantic()
	borderColor := sem.Border.Color()
	dimStyle := lipgloss.NewStyle().Foreground(sem.FgDim.Color())
	headerStyle := lipgloss.NewStyle().Foreground(sem.Primary.Color()).Bold(true)

	// Tab header
	var tabs strings.Builder
	for i, name := range sidebarTabs {
		if i > 0 {
			tabs.WriteString(" ")
		}
		if i == s.activeTab {
			tabs.WriteString(headerStyle.Render("[" + name + "]"))
		} else {
			tabs.WriteString(dimStyle.Render(" " + name + " "))
		}
	}

	// Content area
	contentHeight := s.height - 3 // tabs + top border + bottom border
	if contentHeight < 1 {
		contentHeight = 1
	}
	var content string
	switch s.activeTab {
	case 0:
		content = s.viewFiles(contentHeight)
	case 1:
		content = s.viewGit(contentHeight)
	case 2:
		content = s.viewTools(contentHeight)
	case 3:
		content = s.viewDebug(contentHeight)
	}

	// Compose with border
	innerWidth := s.width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(innerWidth).
		Height(contentHeight + 1). // +1 for tab header
		Render(tabs.String() + "\n" + content)

	return box
}

func (s sidebarModel) viewFiles(maxLines int) string {
	if len(s.files) == 0 {
		return lipgloss.NewStyle().
			Foreground(theme.Current().Semantic().FgDim.Color()).
			Render("No files touched yet")
	}
	sem := theme.Current().Semantic()
	mutStyle := lipgloss.NewStyle().Foreground(sem.Warning.Color())
	readStyle := lipgloss.NewStyle().Foreground(sem.FgDim.Color())

	var lines []string
	for _, f := range s.files {
		if len(lines) >= maxLines {
			break
		}
		short := filepath.Base(f.Path)
		dir := filepath.Dir(f.Path)
		if dir != "." && dir != "/" {
			short = filepath.Join(filepath.Base(dir), short)
		}
		if f.Mutated {
			lines = append(lines, mutStyle.Render("M ")+short)
		} else {
			lines = append(lines, readStyle.Render("R ")+short)
		}
	}
	return strings.Join(lines, "\n")
}

func (s sidebarModel) viewGit(maxLines int) string {
	if s.gitStatus == "" {
		return lipgloss.NewStyle().
			Foreground(theme.Current().Semantic().FgDim.Color()).
			Render("No git changes")
	}
	lines := strings.Split(s.gitStatus, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n")
}

func (s sidebarModel) viewTools(maxLines int) string {
	sem := theme.Current().Semantic()
	dimStyle := lipgloss.NewStyle().Foreground(sem.FgDim.Color())
	var lines []string

	// MCP servers
	if len(s.mcpServers) > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(sem.Primary.Color()).Render("MCP Servers:"))
		for _, srv := range s.mcpServers {
			lines = append(lines, "  "+srv)
		}
		lines = append(lines, "")
	}

	// Recent tool calls (newest first)
	if len(s.toolCalls) > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(sem.Primary.Color()).Render("Recent:"))
		for i := len(s.toolCalls) - 1; i >= 0 && len(lines) < maxLines; i-- {
			tc := s.toolCalls[i]
			elapsed := ""
			if tc.Duration >= time.Second {
				elapsed = fmt.Sprintf(" %.1fs", tc.Duration.Seconds())
			}
			target := tc.Target
			if len(target) > 20 {
				target = "..." + target[len(target)-17:]
			}
			line := tc.Name
			if target != "" {
				line += " " + dimStyle.Render(target)
			}
			if elapsed != "" {
				line += dimStyle.Render(elapsed)
			}
			lines = append(lines, "  "+line)
		}
	}

	if len(lines) == 0 {
		return dimStyle.Render("No tool activity")
	}
	return strings.Join(lines, "\n")
}

func (s sidebarModel) viewDebug(maxLines int) string {
	sem := theme.Current().Semantic()
	labelStyle := lipgloss.NewStyle().Foreground(sem.Primary.Color())
	valStyle := lipgloss.NewStyle().Foreground(sem.Fg.Color())
	dimStyle := lipgloss.NewStyle().Foreground(sem.FgDim.Color())

	lines := []string{
		labelStyle.Render("Phase:    ") + valStyle.Render(s.phase),
		labelStyle.Render("Turns:    ") + valStyle.Render(fmt.Sprintf("%d", s.turns)),
		labelStyle.Render("Tokens:   ") + valStyle.Render(fmt.Sprintf("%dk", s.tokens/1000)),
	}
	if s.subagentCount > 0 {
		lines = append(lines, labelStyle.Render("Agents:   ")+valStyle.Render(fmt.Sprintf("%d active", s.subagentCount)))
	} else {
		lines = append(lines, labelStyle.Render("Agents:   ")+dimStyle.Render("none"))
	}
	return strings.Join(lines, "\n")
}
