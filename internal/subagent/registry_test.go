package subagent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTypeRegistry_Builtins(t *testing.T) {
	reg := NewTypeRegistry("")
	// Explore must exist
	st, err := reg.Get("explore")
	if err != nil {
		t.Fatalf("Get(explore): %v", err)
	}
	if !st.ReadOnly {
		t.Error("explore should be read-only")
	}
	if len(st.Tools) == 0 {
		t.Error("explore should have tools")
	}

	// General must exist
	st, err = reg.Get("general")
	if err != nil {
		t.Fatalf("Get(general): %v", err)
	}
	if st.ReadOnly {
		t.Error("general should not be read-only")
	}

	// List includes both builtins
	all := reg.List()
	if len(all) < 2 {
		t.Errorf("List() = %d types, want >= 2", len(all))
	}

	// Unknown type returns error
	_, err = reg.Get("nonexistent")
	if err == nil {
		t.Error("Get(nonexistent) should return error")
	}
}

func TestTypeRegistry_CustomTOML(t *testing.T) {
	dir := t.TempDir()
	tomlContent := `name = "researcher"
description = "Research-only agent"
tools = ["read", "grep", "glob"]
read_only = true
max_turns = 15
system_prompt = "You are a researcher. {{.TaskPrompt}}"
`
	if err := os.WriteFile(filepath.Join(dir, "researcher.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewTypeRegistry(dir)
	st, err := reg.Get("researcher")
	if err != nil {
		t.Fatalf("Get(researcher): %v", err)
	}
	if st.MaxTurns != 15 {
		t.Errorf("MaxTurns = %d, want 15", st.MaxTurns)
	}
	if !st.ReadOnly {
		t.Error("researcher should be read-only")
	}
}

func TestTypeRegistry_Names(t *testing.T) {
	reg := NewTypeRegistry("")
	names := reg.Names()
	if len(names) < 2 {
		t.Fatalf("Names() = %d, want >= 2", len(names))
	}
	// Should be sorted
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("Names() not sorted: %v", names)
			break
		}
	}
}
