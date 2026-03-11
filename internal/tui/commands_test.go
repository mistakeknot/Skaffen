package tui

import (
	"testing"
)

func TestParseSlashCommand(t *testing.T) {
	tests := []struct {
		input string
		cmd   string
		args  []string
	}{
		{"/compact", "compact", nil},
		{"/verbose", "verbose", nil},
		{"/phase", "phase", nil},
		{"/undo", "undo", nil},
		{"/help", "help", nil},
		{"/sessions", "sessions", nil},
		{"/commit fix typo", "commit", []string{"fix", "typo"}},
		{"not a command", "", nil},
		{"", "", nil},
		{"/", "", nil},
	}
	for _, tt := range tests {
		cmd := ParseCommand(tt.input)
		if tt.cmd == "" {
			if cmd != nil {
				t.Errorf("ParseCommand(%q) = %+v, want nil", tt.input, cmd)
			}
			continue
		}
		if cmd == nil {
			t.Errorf("ParseCommand(%q) = nil, want %q", tt.input, tt.cmd)
			continue
		}
		if cmd.Name != tt.cmd {
			t.Errorf("ParseCommand(%q).Name = %q, want %q", tt.input, cmd.Name, tt.cmd)
		}
		if tt.args != nil && len(cmd.Args) != len(tt.args) {
			t.Errorf("ParseCommand(%q).Args = %v, want %v", tt.input, cmd.Args, tt.args)
		}
	}
}

func TestKnownCommands(t *testing.T) {
	cmds := KnownCommands()
	required := []string{"compact", "verbose", "phase", "undo", "help", "sessions"}
	for _, name := range required {
		if _, ok := cmds[name]; !ok {
			t.Errorf("missing required command %q", name)
		}
	}
}

func TestFormatHelp(t *testing.T) {
	help := FormatHelp()
	if help == "" {
		t.Fatal("help should not be empty")
	}
	if !contains(help, "/compact") {
		t.Fatal("help should mention /compact")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
