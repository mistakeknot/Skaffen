package tui

import (
	"strings"
	"testing"

	"github.com/mistakeknot/Masaq/compact"
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
	required := []string{"compact", "verbose", "phase", "advance", "undo", "commit", "ship", "sessions", "help", "quit"}
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
	if !strings.Contains(help, "/compact") {
		t.Fatal("help should mention /compact")
	}
	if !strings.Contains(help, "/quit") {
		t.Fatal("help should mention /quit")
	}
}

func TestFormatHelpSorted(t *testing.T) {
	help := FormatHelp()
	lines := strings.Split(strings.TrimSpace(help), "\n")
	// Skip header line
	var cmdLines []string
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) != "" {
			cmdLines = append(cmdLines, line)
		}
	}
	// Verify sorted order — each line should come before the next alphabetically
	for i := 1; i < len(cmdLines); i++ {
		if cmdLines[i] < cmdLines[i-1] {
			t.Errorf("commands not sorted: %q should come before %q", cmdLines[i], cmdLines[i-1])
		}
	}
}

func TestExecuteCommand_Help(t *testing.T) {
	m := &appModel{compact: compact.New(80)}
	result := m.executeCommand(&Command{Name: "help"})
	if result.IsError {
		t.Fatal("help should not be an error")
	}
	if !strings.Contains(result.Message, "/help") {
		t.Fatal("help output should list commands")
	}
}

func TestExecuteCommand_Quit(t *testing.T) {
	m := &appModel{compact: compact.New(80)}
	result := m.executeCommand(&Command{Name: "quit"})
	if !result.Quit {
		t.Fatal("quit should set Quit=true")
	}
}

func TestExecuteCommand_Compact(t *testing.T) {
	m := &appModel{compact: compact.New(80)}
	m.compact.SetVerbose(true)
	result := m.executeCommand(&Command{Name: "compact"})
	if result.IsError {
		t.Fatalf("compact should not error: %s", result.Message)
	}
	if m.compact.IsVerbose() {
		t.Fatal("compact should have set verbose=false")
	}
}

func TestExecuteCommand_Verbose(t *testing.T) {
	m := &appModel{compact: compact.New(80)}
	result := m.executeCommand(&Command{Name: "verbose"})
	if result.IsError {
		t.Fatalf("verbose should not error: %s", result.Message)
	}
	if !m.compact.IsVerbose() {
		t.Fatal("verbose should have set verbose=true")
	}
}

func TestExecuteCommand_PhaseNoAgent(t *testing.T) {
	m := &appModel{compact: compact.New(80), phase: "build"}
	result := m.executeCommand(&Command{Name: "phase"})
	if result.IsError {
		t.Fatal("phase should not error without agent")
	}
	if !strings.Contains(result.Message, "build") {
		t.Fatal("phase should show current phase")
	}
}

func TestExecuteCommand_AdvanceNoAgent(t *testing.T) {
	m := &appModel{compact: compact.New(80)}
	result := m.executeCommand(&Command{Name: "advance"})
	if !result.IsError {
		t.Fatal("advance without agent should be an error")
	}
}

func TestExecuteCommand_UndoNoGit(t *testing.T) {
	m := &appModel{compact: compact.New(80)}
	result := m.executeCommand(&Command{Name: "undo"})
	if !result.IsError {
		t.Fatal("undo without git should be an error")
	}
	if !strings.Contains(result.Message, "Git not available") {
		t.Fatalf("unexpected error: %s", result.Message)
	}
}

func TestExecuteCommand_CommitNoGit(t *testing.T) {
	m := &appModel{compact: compact.New(80)}
	result := m.executeCommand(&Command{Name: "commit"})
	if !result.IsError {
		t.Fatal("commit without git should be an error")
	}
}

func TestExecuteCommand_ShipNoGit(t *testing.T) {
	m := &appModel{compact: compact.New(80)}
	result := m.executeCommand(&Command{Name: "ship"})
	if !result.IsError {
		t.Fatal("ship without git should be an error")
	}
}

func TestExecuteCommand_Unknown(t *testing.T) {
	m := &appModel{compact: compact.New(80)}
	result := m.executeCommand(&Command{Name: "bogus"})
	if !result.IsError {
		t.Fatal("unknown command should be an error")
	}
	if !strings.Contains(result.Message, "Unknown command") {
		t.Fatal("should mention unknown command")
	}
}
