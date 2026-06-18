package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--version"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("--version exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "auraken-lens") {
		t.Fatalf("--version stdout did not contain product name: %q", stdout.String())
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("--help exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("--help stderr did not contain usage block: %q", stderr.String())
	}
}

func TestRunDryRunEmitsEmpty(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(
		[]string{"--dry-run"},
		strings.NewReader(`{"text": "Anything"}`),
		&stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("--dry-run exit code = %d, want 0", code)
	}
	var sp Soundpost
	if err := json.Unmarshal(stdout.Bytes(), &sp); err != nil {
		t.Fatalf("dry-run stdout did not parse as JSON: %v\n%s", err, stdout.String())
	}
	if !sp.Empty {
		t.Fatalf("dry-run should always emit empty=true, got %+v", sp)
	}
	if err := sp.Validate(); err != nil {
		t.Fatalf("dry-run output failed schema validation: %v", err)
	}
}

func TestRunEmptyInputEmitsEmptyWithError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var sp Soundpost
	if err := json.Unmarshal(stdout.Bytes(), &sp); err != nil {
		t.Fatalf("empty input stdout did not parse as JSON: %v", err)
	}
	if !sp.Empty {
		t.Fatalf("empty input should yield empty=true, got %+v", sp)
	}
	if sp.Error == "" {
		t.Fatalf("empty input should include an error message, got %+v", sp)
	}
}

func TestReadInputJSONStructured(t *testing.T) {
	in, err := readInput(strings.NewReader(
		`{"text": "hello", "context_summary": "prior", "session_id": "s1"}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	if in.Text != "hello" || in.ContextSummary != "prior" || in.SessionID != "s1" {
		t.Fatalf("structured JSON parsed wrong: %+v", in)
	}
}

func TestReadInputPlainText(t *testing.T) {
	in, err := readInput(strings.NewReader("I have a question about latency."))
	if err != nil {
		t.Fatal(err)
	}
	if in.Text != "I have a question about latency." {
		t.Fatalf("plain text read wrong: %q", in.Text)
	}
}

func TestReadInputBrokenJSONFallsThroughToPlainText(t *testing.T) {
	// Starts with { but isn't valid JSON — must be treated as plain text.
	in, err := readInput(strings.NewReader("{this is not really json"))
	if err != nil {
		t.Fatal(err)
	}
	if in.Text != "{this is not really json" {
		t.Fatalf("broken JSON should fall through to plain text, got %q", in.Text)
	}
}

// TestDefaultAPIMode pins the model→api_mode mapping that backs the
// sylveste-22oi.1 misdiagnosis fix: Claude targets default to
// anthropic_native (Anthropic restricts /chat/completions for the Claude
// Max OAuth account); non-Claude targets default to chat_completions
// (CLIProxyAPI's cross-provider translator path).
func TestDefaultAPIMode(t *testing.T) {
	cases := []struct {
		model string
		want  string
	}{
		{"claude-opus-4-7", apiModeAnthropicNative},
		{"claude-haiku-4-5-20251001", apiModeAnthropicNative},
		{"Claude-Opus-4-7", apiModeAnthropicNative}, // case-insensitive
		{"gpt-5.5", apiModeChatCompletions},
		{"gpt-5.4-mini", apiModeChatCompletions},
		{"gpt-5.3-codex", apiModeChatCompletions},
		{"", apiModeChatCompletions},        // empty model → chat_completions
		{"mistral-large", apiModeChatCompletions},
	}
	for _, c := range cases {
		got := defaultAPIMode(c.model)
		if got != c.want {
			t.Errorf("defaultAPIMode(%q) = %q, want %q", c.model, got, c.want)
		}
	}
}
