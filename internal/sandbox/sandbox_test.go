package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckPathAllowsWorkdir(t *testing.T) {
	workDir := t.TempDir()
	s := New(DefaultPolicy(workDir), ModeDefault)
	if err := s.CheckPath(filepath.Join(workDir, "main.go"), false); err != nil {
		t.Fatalf("read in workdir should be allowed: %v", err)
	}
	if err := s.CheckPath(filepath.Join(workDir, "main.go"), true); err != nil {
		t.Fatalf("write in workdir should be allowed: %v", err)
	}
}

func TestCheckPathDeniesSSH(t *testing.T) {
	home, _ := os.UserHomeDir()
	s := New(DefaultPolicy(t.TempDir()), ModeDefault)
	sshPath := filepath.Join(home, ".ssh", "id_rsa")
	if err := s.CheckPath(sshPath, false); err == nil {
		t.Fatal("read of ~/.ssh/id_rsa should be denied")
	}
}

func TestCheckPathDeniesWriteOutsideWorkdir(t *testing.T) {
	s := New(DefaultPolicy(t.TempDir()), ModeDefault)
	if err := s.CheckPath("/etc/passwd", true); err == nil {
		t.Fatal("write to /etc/passwd should be denied")
	}
}

func TestCheckPathAllowsReadUsr(t *testing.T) {
	s := New(DefaultPolicy(t.TempDir()), ModeDefault)
	if err := s.CheckPath("/usr/bin/git", false); err != nil {
		t.Fatalf("read /usr/bin/git should be allowed: %v", err)
	}
}

func TestCheckPathDisabledMode(t *testing.T) {
	s := New(DefaultPolicy(t.TempDir()), ModeDisabled)
	home, _ := os.UserHomeDir()
	sshPath := filepath.Join(home, ".ssh", "id_rsa")
	if err := s.CheckPath(sshPath, false); err != nil {
		t.Fatalf("disabled mode should allow everything: %v", err)
	}
}

func TestCheckPathStrictMode(t *testing.T) {
	workDir := t.TempDir()
	s := New(StrictPolicy(workDir), ModeStrict)
	if err := s.CheckPath("/usr/bin/git", false); err == nil {
		t.Fatal("strict mode should deny /usr/bin/git read")
	}
	if err := s.CheckPath(filepath.Join(workDir, "src", "main.go"), false); err != nil {
		t.Fatalf("strict mode should allow workdir read: %v", err)
	}
}

func TestCheckPathNilSandbox(t *testing.T) {
	var s *Sandbox
	if err := s.CheckPath("/etc/shadow", false); err != nil {
		t.Fatal("nil sandbox should allow everything")
	}
}

func TestCheckPathDenyOverridesRead(t *testing.T) {
	home, _ := os.UserHomeDir()
	// home is in ReadDirs, but ~/.ssh is in DenyDirs — deny wins
	s := New(DefaultPolicy(t.TempDir()), ModeDefault)
	if err := s.CheckPath(filepath.Join(home, "Documents", "file.txt"), false); err != nil {
		t.Fatalf("reading under home should be allowed: %v", err)
	}
	if err := s.CheckPath(filepath.Join(home, ".ssh", "config"), false); err == nil {
		t.Fatal("reading ~/.ssh should be denied even though home is readable")
	}
}
