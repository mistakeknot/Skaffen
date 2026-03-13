//go:build linux

package sandbox

import (
	"os/exec"
	"strings"
	"testing"
)

func TestBwrapArgsContainBinds(t *testing.T) {
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap not installed")
	}
	workDir := t.TempDir()
	p := DefaultPolicy(workDir)
	s := New(p, ModeDefault)
	name, args := s.WrapArgs("echo", "hello")
	allArgs := name + " " + strings.Join(args, " ")
	if !strings.Contains(allArgs, "--ro-bind") {
		t.Fatalf("expected --ro-bind in bwrap args, got: %s", allArgs)
	}
	if !strings.Contains(allArgs, "--bind "+workDir+" "+workDir) {
		t.Fatalf("expected --bind %s in bwrap args, got: %s", workDir, allArgs)
	}
}

func TestBwrapArgsNetworkDeny(t *testing.T) {
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap not installed")
	}
	p := DefaultPolicy(t.TempDir())
	s := New(p, ModeDefault)
	_, args := s.WrapArgs("echo", "hello")
	allArgs := strings.Join(args, " ")
	if !strings.Contains(allArgs, "--unshare-net") {
		t.Fatal("expected --unshare-net when DenyNet is true")
	}
}

func TestBwrapDisabledMode(t *testing.T) {
	s := New(DisabledPolicy(), ModeDisabled)
	name, args := s.WrapArgs("echo", "hello")
	if name != "echo" {
		t.Fatalf("disabled mode should return original name, got: %s", name)
	}
	if len(args) != 1 || args[0] != "hello" {
		t.Fatalf("disabled mode should return original args, got: %v", args)
	}
}
