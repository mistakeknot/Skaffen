package claudecode

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

func TestClaudeCodeProvider_Name(t *testing.T) {
	p := New(WithBinaryPath("/nonexistent"))
	if p.Name() != "claude-code" {
		t.Errorf("Name() = %q, want %q", p.Name(), "claude-code")
	}
}

func TestClaudeCodeProvider_BinaryNotFound(t *testing.T) {
	p := New(WithBinaryPath("/nonexistent/claude-does-not-exist"))
	_, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hello"}}},
	}, nil, provider.Config{})

	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "claude binary not found") {
		t.Errorf("error = %v, want mention of binary not found", err)
	}
}

func TestClaudeCodeProvider_StreamText(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses bash script")
	}

	// Create a mock script that outputs the golden JSONL
	goldenData, err := os.ReadFile(filepath.Join("testdata", "stream_response.jsonl"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "mock-claude")
	script := "#!/bin/sh\ncat <<'GOLDEN'\n" + string(goldenData) + "GOLDEN\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	p := New(WithBinaryPath(scriptPath))
	resp, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hello"}}},
	}, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	result, err := resp.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if result.Text != "Hello from Claude Code!" {
		t.Errorf("text = %q, want %q", result.Text, "Hello from Claude Code!")
	}
	if result.Usage.InputTokens != 15 {
		t.Errorf("input_tokens = %d, want 15", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 6 {
		t.Errorf("output_tokens = %d, want 6", result.Usage.OutputTokens)
	}
}

func TestClaudeCodeProvider_NonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses bash script")
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "mock-claude")
	script := "#!/bin/sh\necho 'something went wrong' >&2\nexit 1\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	p := New(WithBinaryPath(scriptPath))
	resp, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hello"}}},
	}, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	_, err = resp.Collect()
	if err == nil {
		t.Fatal("expected error from non-zero exit")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("error = %v, want stderr content", err)
	}
}

func TestClaudeCodeProvider_NoUserMessage(t *testing.T) {
	p := New(WithBinaryPath("/bin/echo"))
	_, err := p.Stream(context.Background(), []provider.Message{}, nil, provider.Config{})
	if err == nil {
		t.Fatal("expected error for empty messages")
	}
}
