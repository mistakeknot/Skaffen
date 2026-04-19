package main

import (
	"encoding/json"
	"testing"
)

func TestHeadlessApproverAllowed(t *testing.T) {
	allowed := map[string]bool{"read": true, "edit": true, "bash": true}
	autoApprove := map[string]bool{"read": true}

	tests := []struct {
		tool         string
		edits, bash  bool
		want         bool
	}{
		{"read", false, false, true},       // auto-approved
		{"edit", false, false, false},       // denied without flag
		{"edit", true, false, true},         // allowed with flag
		{"bash", false, false, false},       // denied without flag
		{"bash", false, true, true},         // allowed with flag
		{"unknown", false, false, false},    // not in allowed list
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			fn := headlessApprover(allowed, autoApprove, tt.edits, tt.bash)
			got := fn(tt.tool, nil)
			if got != tt.want {
				t.Errorf("headlessApprover(%q, edits=%v, bash=%v) = %v, want %v",
					tt.tool, tt.edits, tt.bash, got, tt.want)
			}
		})
	}
}

func TestIsTestFile(t *testing.T) {
	patterns := defaultTestPatterns()

	tests := []struct {
		filePath string
		want     bool
	}{
		{"/home/user/project/auth_test.go", true},
		{"/home/user/project/test_auth.py", true},
		{"/home/user/project/auth.test.ts", true},
		{"/home/user/project/auth.spec.js", true},
		{"/home/user/project/tests/auth.py", true},
		{"/home/user/project/__tests__/auth.js", true},
		{"/home/user/project/src/auth.go", false},
		{"/home/user/project/src/auth.py", false},
		{"/home/user/project/main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			input, _ := json.Marshal(map[string]string{"file_path": tt.filePath})
			got := isTestFile(input, patterns)
			if got != tt.want {
				t.Errorf("isTestFile(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestFormatApprovalMessage(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    map[string]any
		contains string
	}{
		{
			"edit",
			"edit",
			map[string]any{"file_path": "/src/auth.go", "old_string": "foo", "new_string": "bar"},
			"[edit] /src/auth.go",
		},
		{
			"write",
			"write",
			map[string]any{"file_path": "/src/new.go", "content": "package main"},
			"[write] /src/new.go",
		},
		{
			"bash",
			"bash",
			map[string]any{"command": "go test ./..."},
			"[bash] go test ./...",
		},
		{
			"unknown",
			"custom",
			map[string]any{},
			"[custom]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, _ := json.Marshal(tt.input)
			got := formatApprovalMessage(tt.toolName, input)
			if got == "" {
				t.Fatal("empty message")
			}
			if !containsStr(got, tt.contains) {
				t.Errorf("message %q should contain %q", got, tt.contains)
			}
		})
	}
}

func TestFormatApprovalDetail(t *testing.T) {
	input, _ := json.Marshal(map[string]any{
		"file_path":  "/src/auth.go",
		"old_string": "func old() {}",
		"new_string": "func new() {}",
	})

	detail := formatApprovalDetail("edit", input)
	if detail == "" {
		t.Fatal("expected non-empty detail for edit")
	}
	if !containsStr(detail, "--- old ---") {
		t.Error("detail should contain old marker")
	}
	if !containsStr(detail, "+++ new +++") {
		t.Error("detail should contain new marker")
	}
}

func TestTruncateLines(t *testing.T) {
	short := "line1\nline2\nline3"
	if got := truncateLines(short, 5); got != short {
		t.Errorf("short text should pass through, got %q", got)
	}

	long := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10"
	got := truncateLines(long, 3)
	if !containsStr(got, "7 more lines") {
		t.Errorf("should indicate truncation, got %q", got)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
