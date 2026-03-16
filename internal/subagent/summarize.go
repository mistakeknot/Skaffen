package subagent

import (
	"context"
	"fmt"
	"strings"
)

// OversizeThreshold is the character count above which tool results
// are candidates for sub-LLM summarization. ~30K chars ≈ ~7.5K tokens.
const OversizeThreshold = 30000

// SummarizePromptTemplate is the system prompt for the summarization sub-agent.
// It instructs a focused extraction of relevant information from oversized output.
const SummarizePromptTemplate = `You are a focused extraction agent. Your job is to extract the most important information from a large tool output.

Rules:
- Extract error messages, test failures, and stack traces FIRST (they are usually at the bottom)
- Keep file paths, line numbers, and assertion messages verbatim
- Preserve the structure of error output (indentation, grouping)
- Omit passing test details, verbose logging, and repeated output
- Keep your summary under 3000 characters
- Do NOT add commentary — just extract the relevant parts`

// IsOversized returns true if the tool result exceeds the summarization threshold.
func IsOversized(result string) bool {
	return len(result) > OversizeThreshold
}

// SummarizeRequest holds the parameters for a tool result summarization.
type SummarizeRequest struct {
	ToolName   string // which tool produced this output
	ToolResult string // the full oversized output
	Intent     string // optional: "test_output", "build_output", "file_content"
}

// FormatSummarizePrompt creates the user prompt for the summarization sub-agent.
// It includes the tool name, an intent hint if provided, and the full output
// with markers showing the head and tail (since errors are often at the bottom).
func FormatSummarizePrompt(req SummarizeRequest) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Tool `%s` produced %d characters of output.\n\n", req.ToolName, len(req.ToolResult)))

	if req.Intent != "" {
		sb.WriteString(fmt.Sprintf("This is %s output. ", req.Intent))
	}
	sb.WriteString("Extract the most relevant information.\n\n")

	// For very long output, show head + tail to ensure errors at the bottom
	// are visible (many test frameworks print failures last)
	if len(req.ToolResult) > OversizeThreshold*2 {
		head := req.ToolResult[:OversizeThreshold/2]
		tail := req.ToolResult[len(req.ToolResult)-OversizeThreshold/2:]
		sb.WriteString("--- HEAD ---\n")
		sb.WriteString(head)
		sb.WriteString("\n\n--- MIDDLE OMITTED ---\n\n")
		sb.WriteString("--- TAIL ---\n")
		sb.WriteString(tail)
	} else {
		sb.WriteString("--- FULL OUTPUT ---\n")
		sb.WriteString(req.ToolResult)
	}

	return sb.String()
}

// SimpleLLM is a minimal interface for one-shot LLM calls used by
// SummarizeToolResult. This avoids coupling to the full Runner/agentloop.
// Implementations can wrap a provider.Provider or a Runner.
type SimpleLLM interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// SummarizeToolResult extracts relevant information from an oversized tool
// result using a focused sub-LLM call. Returns the summarized output, or
// a head+tail truncation if the LLM is unavailable or fails.
//
// This is opt-in — callers should check IsOversized() first. The function
// adds latency (one LLM round-trip) but prevents losing critical error
// messages that would be truncated by naive character limits.
func SummarizeToolResult(ctx context.Context, llm SimpleLLM, req SummarizeRequest) string {
	if llm == nil {
		return truncateHeadTail(req.ToolResult, OversizeThreshold)
	}

	prompt := FormatSummarizePrompt(req)
	result, err := llm.Complete(ctx, SummarizePromptTemplate, prompt)
	if err != nil {
		return truncateHeadTail(req.ToolResult, OversizeThreshold)
	}

	return result
}

// truncateHeadTail keeps the first and last portions of a string,
// inserting an omission marker in the middle.
func truncateHeadTail(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	half := maxLen / 2
	return s[:half] + "\n\n... (" + fmt.Sprintf("%d", len(s)-maxLen) + " chars omitted) ...\n\n" + s[len(s)-half:]
}
