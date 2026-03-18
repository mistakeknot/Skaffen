package session

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// makeMixedMessages creates n messages with varied content types:
// ~30% tool_use (edit/read/grep), ~30% tool_result (mutation/test/read),
// ~20% assistant text, ~20% user text. Some reference activeFile.
func makeMixedMessages(n int, activeFile string) []provider.Message {
	msgs := make([]provider.Message, n)
	for i := range msgs {
		switch i % 10 {
		case 0: // tool_use: edit
			msgs[i] = provider.Message{
				Role:    provider.RoleAssistant,
				Content: []provider.ContentBlock{{Type: "tool_use", Name: "edit", ID: fmt.Sprintf("tu_%d", i)}},
			}
		case 1: // tool_result: mutation
			msgs[i] = provider.Message{
				Role:    provider.RoleUser,
				Content: []provider.ContentBlock{{Type: "tool_result", ToolUseID: fmt.Sprintf("tu_%d", i-1), ResultContent: "File has been updated: " + activeFile}},
			}
		case 2: // tool_use: read
			msgs[i] = provider.Message{
				Role:    provider.RoleAssistant,
				Content: []provider.ContentBlock{{Type: "tool_use", Name: "read", ID: fmt.Sprintf("tu_%d", i)}},
			}
		case 3: // tool_result: file content (long)
			msgs[i] = provider.Message{
				Role:    provider.RoleUser,
				Content: []provider.ContentBlock{{Type: "tool_result", ToolUseID: fmt.Sprintf("tu_%d", i-1), ResultContent: strings.Repeat("line of code\n", 30)}},
			}
		case 4: // tool_use: grep
			msgs[i] = provider.Message{
				Role:    provider.RoleAssistant,
				Content: []provider.ContentBlock{{Type: "tool_use", Name: "grep", ID: fmt.Sprintf("tu_%d", i)}},
			}
		case 5: // tool_result: test output
			msgs[i] = provider.Message{
				Role:    provider.RoleUser,
				Content: []provider.ContentBlock{{Type: "tool_result", ToolUseID: fmt.Sprintf("tu_%d", i-1), ResultContent: "PASS: TestAuth (0.02s)\nPASS"}},
			}
		case 6: // assistant text (reasoning)
			msgs[i] = provider.Message{
				Role:    provider.RoleAssistant,
				Content: []provider.ContentBlock{{Type: "text", Text: "I'll modify the authentication handler to support JWT tokens."}},
			}
		case 7: // user text
			msgs[i] = provider.Message{
				Role:    provider.RoleUser,
				Content: []provider.ContentBlock{{Type: "text", Text: "Can you also update " + activeFile + " with error handling?"}},
			}
		case 8: // tool_use: bash
			msgs[i] = provider.Message{
				Role:    provider.RoleAssistant,
				Content: []provider.ContentBlock{{Type: "tool_use", Name: "bash", ID: fmt.Sprintf("tu_%d", i)}},
			}
		case 9: // tool_result: other
			msgs[i] = provider.Message{
				Role:    provider.RoleUser,
				Content: []provider.ContentBlock{{Type: "tool_result", ToolUseID: fmt.Sprintf("tu_%d", i-1), ResultContent: "exit status 0"}},
			}
		}
	}
	return msgs
}

// makeScoredMessages creates n ScoredMessages with varied scores.
func makeScoredMessages(n int) []ScoredMessage {
	msgs := makeMixedMessages(n, "auth.go")
	return ScoreMessages(msgs, []string{"auth.go"})
}

func BenchmarkScoreMessages50(b *testing.B) {
	msgs := makeMixedMessages(50, "auth.go")
	activeFiles := []string{"auth.go", "handler.go"}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ScoreMessages(msgs, activeFiles)
	}
}

func BenchmarkScoreMessages200(b *testing.B) {
	msgs := makeMixedMessages(200, "auth.go")
	activeFiles := []string{"auth.go", "handler.go"}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ScoreMessages(msgs, activeFiles)
	}
}

func BenchmarkTopK50From200(b *testing.B) {
	scored := makeScoredMessages(200)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		TopK(scored, 50)
	}
}

func BenchmarkTopK50From1000(b *testing.B) {
	scored := makeScoredMessages(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		TopK(scored, 50)
	}
}
