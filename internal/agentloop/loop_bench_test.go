package agentloop

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// generateBashOutput creates realistic bash-like output with file paths,
// line numbers, and command output for benchmarking.
func generateBashOutput(size int) string {
	var b strings.Builder
	lines := []string{
		"$ go build ./cmd/skaffen",
		"$ ls -la internal/agentloop/",
		"total 48",
		"-rw-r--r--  1 mk  staff  12345 Mar 18 14:22 loop.go",
		"-rw-r--r--  1 mk  staff   3456 Mar 18 14:22 types.go",
		"-rw-r--r--  1 mk  staff   2345 Mar 18 14:22 registry.go",
		"$ cat internal/agentloop/loop.go | head -20",
		"package agentloop",
		"",
		"import (",
		`	"context"`,
		`	"fmt"`,
		`	"strings"`,
		")",
		"",
		"// RunLoop executes the main agent loop.",
		"func RunLoop(ctx context.Context, cfg Config) error {",
		"$ go vet ./...",
		"$ echo 'all checks passed'",
		"all checks passed",
		"$ find . -name '*.go' -exec wc -l {} +",
		"     145 ./internal/agentloop/loop.go",
		"      89 ./internal/agentloop/types.go",
		"      67 ./internal/agentloop/registry.go",
		"     301 total",
		"$ git status",
		"On branch main",
		"nothing to commit, working tree clean",
		"$ go test -count=1 ./internal/provider/",
		"ok  	github.com/mistakeknot/Skaffen/internal/provider	0.012s",
		"$ go test -count=1 ./internal/session/",
		"ok  	github.com/mistakeknot/Skaffen/internal/session	0.034s",
	}
	for b.Len() < size {
		for _, line := range lines {
			b.WriteString(line)
			b.WriteByte('\n')
			if b.Len() >= size {
				break
			}
		}
	}
	return b.String()[:size]
}

// makeToolResults builds a single tool_result block with the given content and isError flag.
func makeToolResults(content string, isError bool) []provider.ContentBlock {
	return []provider.ContentBlock{
		{
			Type:          "tool_result",
			ToolUseID:     "toolu_01",
			ResultContent: content,
			IsError:       isError,
		},
	}
}

func makeBashToolCalls() []provider.ToolCall {
	return []provider.ToolCall{
		{ID: "toolu_01", Name: "bash"},
	}
}

// BenchmarkClassifyFailure1K benchmarks 1K input with no error patterns.
func BenchmarkClassifyFailure1K(b *testing.B) {
	content := generateBashOutput(1000)
	results := makeToolResults(content, true)
	calls := makeBashToolCalls()
	b.ResetTimer()
	for b.Loop() {
		classifyFailure(calls, results)
	}
}

// BenchmarkClassifyFailure10K benchmarks 10K input with a syntax error in the middle.
func BenchmarkClassifyFailure10K(b *testing.B) {
	base := generateBashOutput(10000)
	// Embed a real error pattern in the middle
	mid := len(base) / 2
	content := base[:mid] + "\nSyntaxError: unexpected token '}' at line 42\n" + base[mid:]
	results := makeToolResults(content, true)
	calls := makeBashToolCalls()
	b.ResetTimer()
	for b.Loop() {
		classifyFailure(calls, results)
	}
}

// BenchmarkClassifyFailure100K benchmarks 100K input with a panic at the end.
func BenchmarkClassifyFailure100K(b *testing.B) {
	base := generateBashOutput(100000)
	// Error at end — worst case for substring search
	content := base + "\ngoroutine 1 [running]:\npanic: runtime error: index out of range [5] with length 3\n"
	results := makeToolResults(content, true)
	calls := makeBashToolCalls()
	b.ResetTimer()
	for b.Loop() {
		classifyFailure(calls, results)
	}
}

// BenchmarkClassifyFailure10K_TestFailure benchmarks test failure detection (second pass).
func BenchmarkClassifyFailure10K_TestFailure(b *testing.B) {
	base := generateBashOutput(10000)
	mid := len(base) / 2
	content := base[:mid] + "\n--- FAIL: TestRouter (0.01s)\n    router_test.go:42: expected 3, got 5\nFAIL\n" + base[mid:]
	// Not marked as isError — test failures detected in non-error results too
	results := makeToolResults(content, false)
	calls := makeBashToolCalls()
	b.ResetTimer()
	for b.Loop() {
		classifyFailure(calls, results)
	}
}

// BenchmarkClassifyFailure_NoMatch100K benchmarks worst case: 100K, no patterns, full scan.
func BenchmarkClassifyFailure_NoMatch100K(b *testing.B) {
	content := generateBashOutput(100000)
	results := makeToolResults(content, true)
	calls := makeBashToolCalls()
	b.ResetTimer()
	for b.Loop() {
		classifyFailure(calls, results)
	}
}

// Verify benchmarks produce expected results.
func TestClassifyFailureBenchmarkInputs(t *testing.T) {
	calls := makeBashToolCalls()

	t.Run("1K_no_error", func(t *testing.T) {
		content := generateBashOutput(1000)
		results := makeToolResults(content, true)
		got := classifyFailure(calls, results)
		if got == FailSyntaxError || got == FailHallucination {
			t.Errorf("1K clean input should not match syntax/hallucination, got %s", got)
		}
	})

	t.Run("10K_syntax_error", func(t *testing.T) {
		base := generateBashOutput(10000)
		mid := len(base) / 2
		content := base[:mid] + "\nSyntaxError: unexpected token '}' at line 42\n" + base[mid:]
		results := makeToolResults(content, true)
		got := classifyFailure(calls, results)
		if got != FailSyntaxError {
			t.Errorf("expected FailSyntaxError, got %s", got)
		}
	})

	t.Run("100K_panic", func(t *testing.T) {
		base := generateBashOutput(100000)
		content := base + "\ngoroutine 1 [running]:\npanic: runtime error: index out of range\n"
		results := makeToolResults(content, true)
		got := classifyFailure(calls, results)
		// "panic:" matches test failure patterns in the single-pass check,
		// and isError=true also sets hasToolError. Test failure takes priority.
		if got != FailTestFailure {
			t.Errorf("expected FailTestFailure, got %s", got)
		}
	})

	t.Run("10K_test_failure", func(t *testing.T) {
		base := generateBashOutput(10000)
		mid := len(base) / 2
		content := base[:mid] + "\n--- FAIL: TestRouter (0.01s)\nFAIL\n" + base[mid:]
		results := makeToolResults(content, false)
		got := classifyFailure(calls, results)
		if got != FailTestFailure {
			t.Errorf("expected FailTestFailure, got %s", got)
		}
	})
}

// makeFileActivityCalls creates a realistic mix of tool calls for benchmarking
// extractFileActivity. fileOpPct controls what fraction are file operations
// (read/write/edit); the rest are non-file tools (bash, grep, glob, etc.).
func makeFileActivityCalls(n int, fileOpPct float64) []provider.ToolCall {
	nonFileTools := []string{"bash", "grep", "glob", "web_search", "mcp_tool", "list_files"}
	fileOps := []struct {
		name string
		key  string
	}{
		{"read", "file_path"},
		{"write", "file_path"},
		{"edit", "file_path"},
	}

	calls := make([]provider.ToolCall, n)
	fileCount := int(float64(n) * fileOpPct)

	for i := range calls {
		if i < fileCount {
			op := fileOps[i%len(fileOps)]
			input, _ := json.Marshal(map[string]interface{}{
				op.key:  fmt.Sprintf("/home/user/project/src/file_%d.go", i),
				"limit": 100,
			})
			calls[i] = provider.ToolCall{
				ID:    fmt.Sprintf("toolu_%04d", i),
				Name:  op.name,
				Input: input,
			}
		} else {
			tool := nonFileTools[i%len(nonFileTools)]
			input, _ := json.Marshal(map[string]interface{}{
				"command":     fmt.Sprintf("go test ./pkg%d/...", i),
				"description": "run tests",
				"timeout":     30000,
			})
			calls[i] = provider.ToolCall{
				ID:    fmt.Sprintf("toolu_%04d", i),
				Name:  tool,
				Input: input,
			}
		}
	}
	return calls
}

// BenchmarkExtractFileActivity50Calls benchmarks 50 tool calls with ~5% file ops.
func BenchmarkExtractFileActivity50Calls(b *testing.B) {
	calls := makeFileActivityCalls(50, 0.05)
	b.ResetTimer()
	for b.Loop() {
		extractFileActivity(calls)
	}
}

// BenchmarkExtractFileActivity100Calls benchmarks 100 tool calls with ~5% file ops.
func BenchmarkExtractFileActivity100Calls(b *testing.B) {
	calls := makeFileActivityCalls(100, 0.05)
	b.ResetTimer()
	for b.Loop() {
		extractFileActivity(calls)
	}
}

// --- estimateMessageTokens benchmarks ---

// makeConversationMessages builds a realistic conversation of n messages
// alternating user/assistant with varying content sizes.
func makeConversationMessages(n int) []provider.Message {
	msgs := make([]provider.Message, n)
	for i := range msgs {
		if i%2 == 0 {
			// User message: text block (100-500 chars)
			text := strings.Repeat("Please analyze the code in internal/agentloop/loop.go and suggest improvements. ", 3+(i%5))
			msgs[i] = provider.Message{
				Role: provider.RoleUser,
				Content: []provider.ContentBlock{
					{Type: "text", Text: text},
				},
			}
		} else {
			// Assistant message: text + tool_use blocks
			text := strings.Repeat("I'll examine the file and provide suggestions based on the patterns I see. ", 4+(i%3))
			blocks := []provider.ContentBlock{
				{Type: "text", Text: text},
			}
			// Add 1-3 tool_use blocks
			for j := 0; j < 1+(i%3); j++ {
				blocks = append(blocks, provider.ContentBlock{
					Type:  "tool_use",
					ID:    "toolu_bench",
					Name:  "read",
					Input: []byte(`{"file_path":"/home/mk/projects/Demarch/os/Skaffen/internal/agentloop/loop.go"}`),
				})
			}
			msgs[i] = provider.Message{
				Role:    provider.RoleAssistant,
				Content: blocks,
			}
		}
	}
	return msgs
}

// BenchmarkEstimateTokens50Messages benchmarks uncached token estimation for 50 messages.
func BenchmarkEstimateTokens50Messages(b *testing.B) {
	msgs := makeConversationMessages(50)
	b.ResetTimer()
	for b.Loop() {
		estimateMessageTokens(msgs)
	}
}

// BenchmarkEstimateTokens200Messages benchmarks uncached token estimation for 200 messages.
func BenchmarkEstimateTokens200Messages(b *testing.B) {
	msgs := makeConversationMessages(200)
	b.ResetTimer()
	for b.Loop() {
		estimateMessageTokens(msgs)
	}
}

// BenchmarkEstimateTokensCached50Messages benchmarks cached estimation with 50 messages,
// simulating steady-state where only 2 new messages are appended per call.
func BenchmarkEstimateTokensCached50Messages(b *testing.B) {
	msgs := makeConversationMessages(50)
	b.ResetTimer()
	for b.Loop() {
		l := &Loop{}
		// Prime cache with first 48 messages (simulates prior turns)
		l.estimateMessageTokensCached(msgs[:48])
		// Steady-state call: only 2 new messages
		l.estimateMessageTokensCached(msgs)
	}
}

// BenchmarkEstimateTokensCached200Messages benchmarks cached estimation with 200 messages,
// simulating steady-state where only 2 new messages are appended per call.
func BenchmarkEstimateTokensCached200Messages(b *testing.B) {
	msgs := makeConversationMessages(200)
	b.ResetTimer()
	for b.Loop() {
		l := &Loop{}
		// Prime cache with first 198 messages
		l.estimateMessageTokensCached(msgs[:198])
		// Steady-state call: only 2 new messages
		l.estimateMessageTokensCached(msgs)
	}
}
