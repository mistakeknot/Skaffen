package subagent

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestIsOversized(t *testing.T) {
	short := strings.Repeat("x", OversizeThreshold-1)
	if IsOversized(short) {
		t.Error("should not be oversized")
	}
	long := strings.Repeat("x", OversizeThreshold+1)
	if !IsOversized(long) {
		t.Error("should be oversized")
	}
}

func TestFormatSummarizePrompt(t *testing.T) {
	req := SummarizeRequest{
		ToolName:   "bash",
		ToolResult: "test output here",
		Intent:     "test_output",
	}
	prompt := FormatSummarizePrompt(req)
	if !strings.Contains(prompt, "bash") {
		t.Error("should mention tool name")
	}
	if !strings.Contains(prompt, "test_output") {
		t.Error("should mention intent")
	}
	if !strings.Contains(prompt, "test output here") {
		t.Error("should contain the result")
	}
}

func TestFormatSummarizePromptVeryLong(t *testing.T) {
	// Very long output should get head+tail treatment in the prompt
	long := strings.Repeat("A", OversizeThreshold) + strings.Repeat("B", OversizeThreshold+1)
	req := SummarizeRequest{
		ToolName:   "bash",
		ToolResult: long,
	}
	prompt := FormatSummarizePrompt(req)
	if !strings.Contains(prompt, "HEAD") {
		t.Error("very long output should show HEAD marker")
	}
	if !strings.Contains(prompt, "TAIL") {
		t.Error("very long output should show TAIL marker")
	}
	if !strings.Contains(prompt, "MIDDLE OMITTED") {
		t.Error("very long output should show MIDDLE OMITTED")
	}
}

func TestTruncateHeadTail(t *testing.T) {
	s := strings.Repeat("A", 100) + strings.Repeat("B", 100)
	result := truncateHeadTail(s, 50)
	if !strings.HasPrefix(result, "AAAA") {
		t.Error("should start with head")
	}
	if !strings.HasSuffix(result, "BBBB") {
		t.Error("should end with tail")
	}
	if !strings.Contains(result, "omitted") {
		t.Error("should contain omission marker")
	}
}

func TestTruncateHeadTailShort(t *testing.T) {
	s := "short"
	result := truncateHeadTail(s, 100)
	if result != s {
		t.Error("short string should not be truncated")
	}
}

type mockLLM struct {
	response string
	err      error
}

func (m *mockLLM) Complete(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

func TestSummarizeToolResultWithLLM(t *testing.T) {
	llm := &mockLLM{response: "FAIL: TestLogin expected 200, got 401"}
	req := SummarizeRequest{
		ToolName:   "bash",
		ToolResult: strings.Repeat("x", OversizeThreshold+1),
	}
	result := SummarizeToolResult(context.Background(), llm, req)
	if result != "FAIL: TestLogin expected 200, got 401" {
		t.Errorf("expected LLM response, got %q", result)
	}
}

func TestSummarizeToolResultNilLLM(t *testing.T) {
	req := SummarizeRequest{
		ToolName:   "bash",
		ToolResult: strings.Repeat("x", OversizeThreshold+1),
	}
	result := SummarizeToolResult(context.Background(), nil, req)
	if !strings.Contains(result, "omitted") {
		t.Error("nil LLM should fall back to truncation")
	}
}

func TestSummarizeToolResultLLMError(t *testing.T) {
	llm := &mockLLM{err: fmt.Errorf("provider unavailable")}
	req := SummarizeRequest{
		ToolName:   "bash",
		ToolResult: strings.Repeat("x", OversizeThreshold+1),
	}
	result := SummarizeToolResult(context.Background(), llm, req)
	if !strings.Contains(result, "omitted") {
		t.Error("LLM error should fall back to truncation")
	}
}
