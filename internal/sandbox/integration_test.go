//go:build linux

package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func skipIfBwrapUnavailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap not installed")
	}
	// Check if user namespaces are available (Ubuntu 24.04 restricts them)
	cmd := exec.Command("bwrap", "--ro-bind", "/usr", "/usr", "--", "true")
	if err := cmd.Run(); err != nil {
		t.Skip("bwrap cannot create user namespaces (kernel restriction), skipping")
	}
}

func TestIntegrationBwrapBlocksDeniedPath(t *testing.T) {
	skipIfBwrapUnavailable(t)

	secretDir := t.TempDir()
	secretFile := filepath.Join(secretDir, "secret.txt")
	os.WriteFile(secretFile, []byte("top-secret"), 0644)

	workDir := t.TempDir()
	p := Policy{
		WriteDirs: []string{workDir},
		ReadDirs:  []string{"/usr", "/bin", "/lib", "/lib64", "/tmp"},
		DenyDirs:  []string{secretDir},
		DenyNet:   true,
	}
	s := New(p, ModeDefault)

	name, args := s.WrapArgs("cat", secretFile)
	cmd := exec.CommandContext(context.Background(), name, args...)
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("expected bwrap to block access, got output: %s", string(out))
	}
	if strings.Contains(string(out), "top-secret") {
		t.Fatal("secret content should not be readable")
	}
}

func TestIntegrationBwrapAllowsWorkdir(t *testing.T) {
	skipIfBwrapUnavailable(t)

	workDir := t.TempDir()
	testFile := filepath.Join(workDir, "test.txt")
	os.WriteFile(testFile, []byte("allowed"), 0644)

	p := Policy{
		WriteDirs: []string{workDir},
		ReadDirs:  []string{"/usr", "/bin", "/lib", "/lib64", "/tmp"},
		DenyNet:   false,
	}
	s := New(p, ModeDefault)

	name, args := s.WrapArgs("cat", testFile)
	cmd := exec.CommandContext(context.Background(), name, args...)
	out, err := cmd.CombinedOutput()

	if err != nil {
		// Also capture stderr for debugging
		t.Logf("bwrap command: %s %v", name, args)
		t.Fatalf("expected workdir access to succeed, got: %v\noutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "allowed") {
		t.Fatalf("expected 'allowed' in output, got: %s", string(out))
	}
}
