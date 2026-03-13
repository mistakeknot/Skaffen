package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Skaffen/internal/command"
	"github.com/mistakeknot/Skaffen/internal/skill"
	"github.com/mistakeknot/Masaq/theme"
)

const maxCompleterItems = 8

type cmdCompleterModel struct {
	commands []cmdEntry // all commands, sorted
	filtered []cmdEntry // matching current pattern
	pattern  string     // text after "/"
	cursor   int
	visible  bool
}

type cmdEntry struct {
	name string
	desc string
}

// cmdCompleterSelectedMsg is sent when the user picks a command.
type cmdCompleterSelectedMsg struct {
	Name string
}

// cmdCompleterCancelMsg is sent when the completer is dismissed.
type cmdCompleterCancelMsg struct{}

func newCmdCompleter(custom map[string]command.Def, skills map[string]skill.Def) cmdCompleterModel {
	known := KnownCommands()
	for name, def := range custom {
		if _, exists := known[name]; !exists {
			known[name] = def.Description
		}
	}
	for name, def := range skills {
		if def.UserInvocable {
			if _, exists := known[name]; !exists {
				known[name] = def.Description + " [skill]"
			}
		}
	}
	entries := make([]cmdEntry, 0, len(known))
	for name, desc := range known {
		entries = append(entries, cmdEntry{name, desc})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})
	return cmdCompleterModel{
		commands: entries,
		filtered: entries,
		visible:  true,
	}
}

func (cc cmdCompleterModel) Update(msg tea.Msg) (cmdCompleterModel, tea.Cmd) {
	if !cc.visible {
		return cc, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if cc.cursor > 0 {
				cc.cursor--
			}
		case "down":
			max := len(cc.filtered) - 1
			if max >= maxCompleterItems-1 {
				max = maxCompleterItems - 1
			}
			if cc.cursor < max {
				cc.cursor++
			}
		case "tab", "enter":
			if len(cc.filtered) > 0 && cc.cursor < len(cc.filtered) {
				cc.visible = false
				return cc, func() tea.Msg {
					return cmdCompleterSelectedMsg{Name: cc.filtered[cc.cursor].name}
				}
			}
		case "esc", "ctrl+c":
			cc.visible = false
			return cc, func() tea.Msg { return cmdCompleterCancelMsg{} }
		case "backspace":
			if len(cc.pattern) > 0 {
				cc.pattern = cc.pattern[:len(cc.pattern)-1]
				cc.filtered = filterCommands(cc.commands, cc.pattern)
				cc.cursor = clampCursor(cc.cursor, cc.filtered)
			} else {
				// Backspace past "/" → cancel
				cc.visible = false
				return cc, func() tea.Msg { return cmdCompleterCancelMsg{} }
			}
		default:
			if len(msg.Runes) > 0 {
				for _, r := range msg.Runes {
					// Space means done typing the command name — select current
					if r == ' ' {
						if len(cc.filtered) > 0 && cc.cursor < len(cc.filtered) {
							cc.visible = false
							return cc, func() tea.Msg {
								return cmdCompleterSelectedMsg{Name: cc.filtered[cc.cursor].name}
							}
						}
						return cc, nil
					}
					cc.pattern += string(r)
				}
				cc.filtered = filterCommands(cc.commands, cc.pattern)
				cc.cursor = clampCursor(cc.cursor, cc.filtered)
			}
		}
	}
	return cc, nil
}

func (cc cmdCompleterModel) View(width int) string {
	if !cc.visible || width < 10 {
		return ""
	}
	c := theme.Current().Semantic()

	headerStyle := lipgloss.NewStyle().Foreground(c.Muted.Color())
	selectedStyle := lipgloss.NewStyle().Background(c.Primary.Color()).Foreground(c.Bg.Color())
	nameStyle := lipgloss.NewStyle().Foreground(c.Fg.Color())
	descStyle := lipgloss.NewStyle().Foreground(c.FgDim.Color())

	var lines []string

	show := cc.filtered
	if len(show) > maxCompleterItems {
		show = show[:maxCompleterItems]
	}
	for i, e := range show {
		entry := "/" + e.name + "  " + e.desc
		if i == cc.cursor {
			lines = append(lines, selectedStyle.Render("> "+entry))
		} else {
			lines = append(lines, nameStyle.Render("  /")+nameStyle.Render(e.name)+"  "+descStyle.Render(e.desc))
		}
	}
	if len(cc.filtered) > maxCompleterItems {
		lines = append(lines, headerStyle.Render(
			"  ... and "+itoa(len(cc.filtered)-maxCompleterItems)+" more"))
	}
	if len(cc.filtered) == 0 {
		lines = append(lines, headerStyle.Render("  (no matching commands)"))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c.Secondary.Color()).
		Width(width - 4).
		Padding(0, 1)

	return box.Render(strings.Join(lines, "\n"))
}

func filterCommands(commands []cmdEntry, pattern string) []cmdEntry {
	if pattern == "" {
		return commands
	}
	lower := strings.ToLower(pattern)
	var result []cmdEntry
	// Prefix matches first, then substring matches
	var prefix, substring []cmdEntry
	for _, e := range commands {
		nameLower := strings.ToLower(e.name)
		if strings.HasPrefix(nameLower, lower) {
			prefix = append(prefix, e)
		} else if strings.Contains(nameLower, lower) || strings.Contains(strings.ToLower(e.desc), lower) {
			substring = append(substring, e)
		}
	}
	result = append(result, prefix...)
	result = append(result, substring...)
	return result
}

func clampCursor(cursor int, filtered []cmdEntry) int {
	max := len(filtered) - 1
	if max >= maxCompleterItems-1 {
		max = maxCompleterItems - 1
	}
	if cursor > max {
		return max
	}
	if cursor < 0 {
		return 0
	}
	return cursor
}
