package main

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

func TestVersionOutput(t *testing.T) {
	// Build the binary
	binary := t.TempDir() + "/skaffen"
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	// Run version
	out, err := exec.Command(binary, "version").Output()
	if err != nil {
		t.Fatalf("version failed: %v", err)
	}

	output := string(out)
	if !bytes.Contains(out, []byte("skaffen")) {
		t.Errorf("version output missing 'skaffen': %s", output)
	}
	if !bytes.Contains(out, []byte("go1.")) {
		t.Errorf("version output missing Go version: %s", output)
	}
}

func TestUnknownCommand(t *testing.T) {
	binary := t.TempDir() + "/skaffen"
	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	cmd = exec.Command(binary, "bogus")
	err := cmd.Run()
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestNoPromptError(t *testing.T) {
	binary := t.TempDir() + "/skaffen"
	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	// Run with empty stdin and no -p flag
	cmd = exec.Command(binary)
	cmd.Stdin = bytes.NewReader(nil)
	cmd.Env = append(os.Environ(), "ANTHROPIC_API_KEY=test-key")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected error for missing prompt")
	}
	if !bytes.Contains(out, []byte("no prompt provided")) {
		t.Errorf("error message: %s", string(out))
	}
}

func TestInvalidPhase(t *testing.T) {
	binary := t.TempDir() + "/skaffen"
	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	cmd = exec.Command(binary, "-phase", "invalid", "-p", "hello")
	cmd.Env = append(os.Environ(), "ANTHROPIC_API_KEY=test-key")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected error for invalid phase")
	}
	if !bytes.Contains(out, []byte("invalid phase")) {
		t.Errorf("error message: %s", string(out))
	}
}

func TestMissingAPIKey(t *testing.T) {
	binary := t.TempDir() + "/skaffen"
	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	// Clear ANTHROPIC_API_KEY
	env := []string{}
	for _, e := range os.Environ() {
		if !bytes.HasPrefix([]byte(e), []byte("ANTHROPIC_API_KEY=")) {
			env = append(env, e)
		}
	}

	// Must explicitly request anthropic provider to trigger API key check
	// (default is claude-code which uses OAuth)
	cmd = exec.Command(binary, "-provider", "anthropic", "-p", "hello")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected error for missing API key")
	}
	if !bytes.Contains(out, []byte("ANTHROPIC_API_KEY not set")) {
		t.Errorf("error message: %s", string(out))
	}
}

func TestAutoDetectProvider(t *testing.T) {
	binary := t.TempDir() + "/skaffen"
	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	// With ANTHROPIC_API_KEY set, should auto-select anthropic
	// (will fail at the actual API call, but we check it picks the right provider)
	cmd = exec.Command(binary, "-p", "hello")
	cmd.Env = append(os.Environ(), "ANTHROPIC_API_KEY=sk-ant-test-invalid")
	out, _ := cmd.CombinedOutput()
	output := string(out)
	// Should NOT say "claude" binary not found — it picked anthropic
	if bytes.Contains(out, []byte("claude binary not found")) {
		t.Errorf("should auto-detect anthropic when API key set, got: %s", output)
	}
}
