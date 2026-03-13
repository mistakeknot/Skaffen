package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy("/home/user/project")
	if len(p.WriteDirs) == 0 {
		t.Fatal("expected write dirs in default policy")
	}
	if !p.DenyNet {
		t.Fatal("expected network denied by default")
	}
	if len(p.DenyDirs) == 0 {
		t.Fatal("expected deny dirs in default policy")
	}
}

func TestDefaultPolicyContainsWorkdir(t *testing.T) {
	p := DefaultPolicy("/work")
	found := false
	for _, d := range p.WriteDirs {
		if d == "/work" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected /work in WriteDirs, got %v", p.WriteDirs)
	}
}

func TestStrictPolicy(t *testing.T) {
	p := StrictPolicy("/work")
	if len(p.ReadDirs) != 1 || p.ReadDirs[0] != "/work" {
		t.Fatalf("strict policy should only allow workdir, got ReadDirs=%v", p.ReadDirs)
	}
	if !p.DenyNet {
		t.Fatal("strict should deny network")
	}
}

func TestDisabledPolicy(t *testing.T) {
	p := DisabledPolicy()
	if len(p.WriteDirs) == 0 || p.WriteDirs[0] != "/" {
		t.Fatal("disabled policy should allow root write")
	}
	if p.DenyNet {
		t.Fatal("disabled policy should not deny network")
	}
}

func TestExpandVars(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := expandVars("~/.ssh", "/work")
	want := filepath.Join(home, ".ssh")
	if got != want {
		t.Fatalf("expandVars(~/.ssh) = %q, want %q", got, want)
	}
	got = expandVars("$WORKDIR/src", "/work")
	if got != "/work/src" {
		t.Fatalf("expandVars($WORKDIR/src) = %q, want /work/src", got)
	}
	got = expandVars("~", "/work")
	if got != home {
		t.Fatalf("expandVars(~) = %q, want %q", got, home)
	}
}

func TestMerge(t *testing.T) {
	base := DefaultPolicy("/work")
	overlay := Policy{
		WriteDirs: []string{"/extra"},
		DenyDirs:  []string{"/secret"},
	}
	merged := Merge(base, overlay)
	foundExtra := false
	for _, d := range merged.WriteDirs {
		if d == "/extra" {
			foundExtra = true
		}
	}
	if !foundExtra {
		t.Fatal("expected /extra in merged WriteDirs")
	}
	foundSecret := false
	for _, d := range merged.DenyDirs {
		if d == "/secret" {
			foundSecret = true
		}
	}
	if !foundSecret {
		t.Fatal("expected /secret in merged DenyDirs")
	}
}

func TestLoadFromJSON(t *testing.T) {
	dir := t.TempDir()
	skDir := filepath.Join(dir, ".skaffen")
	os.Mkdir(skDir, 0755)
	os.WriteFile(filepath.Join(skDir, "sandbox.json"), []byte(`{
		"write": ["$WORKDIR/extra"],
		"deny": ["~/.secret"]
	}`), 0644)

	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range p.WriteDirs {
		if d == filepath.Join(dir, "extra") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected $WORKDIR/extra in WriteDirs, got %v", p.WriteDirs)
	}
}

func TestLoadNoConfig(t *testing.T) {
	dir := t.TempDir()
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.WriteDirs) == 0 {
		t.Fatal("expected default policy when no config exists")
	}
}
