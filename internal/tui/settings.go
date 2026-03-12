package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mistakeknot/Masaq/theme"
)

// settings holds runtime-configurable TUI preferences.
// All fields are live — changes take effect immediately.
type settings struct {
	Verbose         bool   // verbose tool call display (vs compact)
	ShowToolResults bool   // show successful tool results, not just errors
	DiffPreview     bool   // show inline diff preview on edit/write approval
	AutoScroll      bool   // viewport follows new content
	Timestamps      bool   // show timestamps on messages
	Theme           string // active theme name
	ColorMode       string // "dark" or "light"
}

func defaultSettings() settings {
	return settings{
		Verbose:         false,
		ShowToolResults: false,
		DiffPreview:     true,
		AutoScroll:      true,
		Timestamps:      false,
		Theme:           theme.Current().Name,
		ColorMode:       theme.CurrentMode().String(),
	}
}

// settingEntry describes one setting for display and parsing.
type settingEntry struct {
	Key         string
	Description string
	Get         func(s *settings) string
	Set         func(s *settings, val string) error
}

// settingsRegistry defines all available settings.
var settingsRegistry = []settingEntry{
	{
		Key:         "verbose",
		Description: "Verbose tool call display",
		Get:         func(s *settings) string { return fmtBool(s.Verbose) },
		Set: func(s *settings, val string) error {
			b, err := parseBool(val)
			if err != nil {
				return err
			}
			s.Verbose = b
			return nil
		},
	},
	{
		Key:         "show-tool-results",
		Description: "Show successful tool results (not just errors)",
		Get:         func(s *settings) string { return fmtBool(s.ShowToolResults) },
		Set: func(s *settings, val string) error {
			b, err := parseBool(val)
			if err != nil {
				return err
			}
			s.ShowToolResults = b
			return nil
		},
	},
	{
		Key:         "diff-preview",
		Description: "Inline diff preview on edit/write approval",
		Get:         func(s *settings) string { return fmtBool(s.DiffPreview) },
		Set: func(s *settings, val string) error {
			b, err := parseBool(val)
			if err != nil {
				return err
			}
			s.DiffPreview = b
			return nil
		},
	},
	{
		Key:         "auto-scroll",
		Description: "Viewport follows new content",
		Get:         func(s *settings) string { return fmtBool(s.AutoScroll) },
		Set: func(s *settings, val string) error {
			b, err := parseBool(val)
			if err != nil {
				return err
			}
			s.AutoScroll = b
			return nil
		},
	},
	{
		Key:         "timestamps",
		Description: "Show timestamps on messages",
		Get:         func(s *settings) string { return fmtBool(s.Timestamps) },
		Set: func(s *settings, val string) error {
			b, err := parseBool(val)
			if err != nil {
				return err
			}
			s.Timestamps = b
			return nil
		},
	},
	{
		Key:         "theme",
		Description: "Color theme",
		Get:         func(s *settings) string { return s.Theme },
		Set: func(s *settings, val string) error {
			t, ok := theme.ThemeByName(val)
			if !ok {
				names := availableThemeNames()
				return fmt.Errorf("unknown theme %q (available: %s)", val, strings.Join(names, ", "))
			}
			theme.SetCurrent(t)
			s.Theme = t.Name
			return nil
		},
	},
	{
		Key:         "color-mode",
		Description: "Color mode (dark/light)",
		Get:         func(s *settings) string { return s.ColorMode },
		Set: func(s *settings, val string) error {
			switch strings.ToLower(val) {
			case "dark":
				theme.SetMode(theme.Dark)
				s.ColorMode = "dark"
			case "light":
				theme.SetMode(theme.Light)
				s.ColorMode = "light"
			default:
				return fmt.Errorf("unknown color mode %q (use dark or light)", val)
			}
			return nil
		},
	},
}

// FormatSettings renders all settings as a table.
func FormatSettings(s *settings) string {
	var b strings.Builder
	b.WriteString("Settings:\n")

	// Find max key length for alignment
	maxKey := 0
	for _, e := range settingsRegistry {
		if len(e.Key) > maxKey {
			maxKey = len(e.Key)
		}
	}

	for _, e := range settingsRegistry {
		b.WriteString(fmt.Sprintf("  %-*s = %-8s  %s\n", maxKey, e.Key, e.Get(s), e.Description))
	}
	b.WriteString("\nChange with: /settings <key> <value>")
	return b.String()
}

// ApplySetting updates a setting by key. Returns a user-friendly message.
func ApplySetting(s *settings, key, value string) (string, error) {
	for _, e := range settingsRegistry {
		if e.Key == key {
			if err := e.Set(s, value); err != nil {
				return "", err
			}
			return fmt.Sprintf("%s = %s", key, e.Get(s)), nil
		}
	}
	// Fuzzy suggestion
	keys := make([]string, len(settingsRegistry))
	for i, e := range settingsRegistry {
		keys[i] = e.Key
	}
	return "", fmt.Errorf("unknown setting %q. Available: %s", key, strings.Join(keys, ", "))
}

func availableThemeNames() []string {
	themes := theme.Themes()
	names := make([]string, len(themes))
	for i, t := range themes {
		names[i] = strings.ToLower(t.Name)
	}
	sort.Strings(names)
	return names
}

func fmtBool(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "on", "true", "1", "yes":
		return true, nil
	case "off", "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q (use on/off)", s)
	}
}
