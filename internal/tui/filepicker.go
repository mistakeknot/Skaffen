package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Masaq/theme"
)

// filePickerSelectedMsg is sent when the user selects a file.
type filePickerSelectedMsg struct {
	Path string
}

// filePickerCancelMsg is sent when the user cancels the picker.
type filePickerCancelMsg struct{}

// skipDirs are directories excluded from file listing.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"__pycache__":  true,
	".beads":       true,
	".tldrs":       true,
	".clavain":     true,
}

const maxPickerItems = 10

type filePickerModel struct {
	root     string
	allFiles []string
	filtered []string
	pattern  string
	cursor   int
	visible  bool
}

func newFilePicker(root string) filePickerModel {
	files := walkFiles(root, 5)
	return filePickerModel{
		root:     root,
		allFiles: files,
		filtered: files,
		visible:  true,
	}
}

func (fp filePickerModel) Update(msg tea.Msg) (filePickerModel, tea.Cmd) {
	if !fp.visible {
		return fp, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if fp.cursor > 0 {
				fp.cursor--
			}
		case "down":
			max := len(fp.filtered) - 1
			if max >= maxPickerItems {
				max = maxPickerItems - 1
			}
			if fp.cursor < max {
				fp.cursor++
			}
		case "enter":
			if len(fp.filtered) > 0 && fp.cursor < len(fp.filtered) {
				fp.visible = false
				return fp, func() tea.Msg {
					return filePickerSelectedMsg{Path: fp.filtered[fp.cursor]}
				}
			}
		case "esc", "ctrl+c":
			fp.visible = false
			return fp, func() tea.Msg { return filePickerCancelMsg{} }
		case "backspace":
			if len(fp.pattern) > 0 {
				fp.pattern = fp.pattern[:len(fp.pattern)-1]
				fp.filtered = filterFiles(fp.allFiles, fp.pattern)
				fp.cursor = 0
			} else {
				// Backspace with empty pattern cancels
				fp.visible = false
				return fp, func() tea.Msg { return filePickerCancelMsg{} }
			}
		default:
			if len(msg.Runes) > 0 {
				for _, r := range msg.Runes {
					fp.pattern += string(r)
				}
				fp.filtered = filterFiles(fp.allFiles, fp.pattern)
				fp.cursor = 0
			}
		}
	}
	return fp, nil
}

func (fp filePickerModel) View(width int) string {
	if !fp.visible || width < 10 {
		return ""
	}
	c := theme.Current().Semantic()

	headerStyle := lipgloss.NewStyle().Foreground(c.Muted.Color())
	selectedStyle := lipgloss.NewStyle().Background(c.Primary.Color()).Foreground(c.Bg.Color())
	normalStyle := lipgloss.NewStyle().Foreground(c.Fg.Color())

	var lines []string
	prompt := "@" + fp.pattern
	lines = append(lines, headerStyle.Render("Files matching: "+prompt))

	show := fp.filtered
	if len(show) > maxPickerItems {
		show = show[:maxPickerItems]
	}
	for i, f := range show {
		if i == fp.cursor {
			lines = append(lines, selectedStyle.Render("> "+f))
		} else {
			lines = append(lines, normalStyle.Render("  "+f))
		}
	}
	if len(fp.filtered) > maxPickerItems {
		lines = append(lines, headerStyle.Render(
			"  ... and "+strings.Repeat(" ", 0)+itoa(len(fp.filtered)-maxPickerItems)+" more"))
	}
	if len(fp.filtered) == 0 {
		lines = append(lines, headerStyle.Render("  (no matches)"))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c.Secondary.Color()).
		Width(width - 4).
		Padding(0, 1)

	return box.Render(strings.Join(lines, "\n"))
}

// walkFiles returns relative paths under root, excluding skip dirs, up to maxDepth.
func walkFiles(root string, maxDepth int) []string {
	var files []string
	rootClean := filepath.Clean(root)
	filepath.WalkDir(rootClean, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		rel, _ := filepath.Rel(rootClean, path)
		if rel == "." {
			return nil
		}
		// Check depth
		depth := strings.Count(rel, string(filepath.Separator)) + 1
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, rel)
		return nil
	})
	sort.Strings(files)
	return files
}

// filterFiles returns files matching pattern via case-insensitive substring,
// sorted by match position (earlier = better).
func filterFiles(files []string, pattern string) []string {
	if pattern == "" {
		return files
	}
	lower := strings.ToLower(pattern)
	type scored struct {
		path string
		pos  int
	}
	var matches []scored
	for _, f := range files {
		pos := strings.Index(strings.ToLower(f), lower)
		if pos >= 0 {
			matches = append(matches, scored{f, pos})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].pos != matches[j].pos {
			return matches[i].pos < matches[j].pos
		}
		return matches[i].path < matches[j].path
	})
	result := make([]string, len(matches))
	for i, m := range matches {
		result[i] = m.path
	}
	return result
}

// itoa is a simple int-to-string to avoid importing strconv for one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
