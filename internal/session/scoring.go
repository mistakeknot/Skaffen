package session

import (
	"container/heap"
	"strings"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// scoreHeap is a min-heap of ScoredMessage used for TopK selection.
// The minimum-scored element sits at index 0, so we can efficiently
// evict the weakest candidate when the heap exceeds size K.
type scoreHeap []ScoredMessage

func (h scoreHeap) Len() int           { return len(h) }
func (h scoreHeap) Less(i, j int) bool { return h[i].Score < h[j].Score } // min-heap
func (h scoreHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *scoreHeap) Push(x any)        { *h = append(*h, x.(ScoredMessage)) }
func (h *scoreHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// ScoredMessage pairs a message with a relevance score for compaction.
type ScoredMessage struct {
	Index   int // original position in the message slice
	Message provider.Message
	Score   float64 // higher = more relevant, should be retained
}

// ScoreMessages assigns relevance scores to each message based on content type
// and relationship to the current task. Messages referencing actively-edited
// files score higher than exploratory reads or conversation turns.
//
// Scoring heuristic (base scores, higher = more important):
//   - tool_result with file mutation (write/edit): 10.0
//   - tool_use for write/edit: 8.0
//   - tool_result with test output: 7.0
//   - tool_result with file read: 5.0
//   - tool_use for read/grep/glob: 4.0
//   - tool_result (other): 3.0
//   - assistant text (reasoning): 2.0
//   - user text (conversation): 1.0
//
// Boost: +3.0 if the message references any of the activeFiles.
func ScoreMessages(messages []provider.Message, activeFiles []string) []ScoredMessage {
	scored := make([]ScoredMessage, len(messages))
	for i, msg := range messages {
		scored[i] = ScoredMessage{
			Index:   i,
			Message: msg,
			Score:   scoreMessage(msg, activeFiles),
		}
	}
	return scored
}

// TopK returns the indices of the top-K highest-scored messages, preserving
// original order. If K >= len(scored), returns all indices in order.
//
// Uses a min-heap of size K for O(n log k) selection instead of O(n*k)
// partial selection sort. When K << N this is substantially faster.
func TopK(scored []ScoredMessage, k int) []int {
	if k >= len(scored) {
		indices := make([]int, len(scored))
		for i := range indices {
			indices[i] = i
		}
		return indices
	}

	// Fill the first k elements directly, then heapify — avoids k allocations
	// from heap.Push interface boxing.
	h := make(scoreHeap, k)
	copy(h, scored[:k])
	heap.Init(&h)

	// Scan remaining elements; replace heap root when we find a higher score.
	for i := k; i < len(scored); i++ {
		if scored[i].Score > h[0].Score {
			h[0] = scored[i]
			heap.Fix(&h, 0)
		}
	}

	// Mark selected indices in a bitset for O(1) lookup
	selected := make([]bool, len(scored))
	for _, s := range h {
		selected[s.Index] = true
	}

	// Collect in original order
	indices := make([]int, 0, k)
	for i := range scored {
		if selected[i] {
			indices = append(indices, i)
		}
	}

	return indices
}

func scoreMessage(msg provider.Message, activeFiles []string) float64 {
	var maxScore float64

	for _, block := range msg.Content {
		var blockScore float64

		switch block.Type {
		case "tool_use":
			blockScore = scoreToolUse(block.Name)
		case "tool_result":
			blockScore = scoreToolResult(block)
		case "text":
			if msg.Role == provider.RoleAssistant {
				blockScore = 2.0
			} else {
				blockScore = 1.0
			}
		default:
			blockScore = 1.0
		}

		if blockScore > maxScore {
			maxScore = blockScore
		}
	}

	// Boost if message references actively-edited files
	if len(activeFiles) > 0 && referencesFiles(msg, activeFiles) {
		maxScore += 3.0
	}

	return maxScore
}

func scoreToolUse(toolName string) float64 {
	switch toolName {
	case "write", "edit":
		return 8.0
	case "read", "grep", "glob", "ls":
		return 4.0
	case "bash":
		return 5.0 // could be test run or build
	default:
		return 3.0
	}
}

func scoreToolResult(block provider.ContentBlock) float64 {
	content := block.ResultContent
	if content == "" {
		content = block.Text
	}

	// Check for mutation indicators
	if containsAny(content, []string{"File created", "has been updated", "written to"}) {
		return 10.0
	}

	// Check for test output indicators
	if containsAny(content, []string{"PASS", "FAIL", "passed", "failed", "Error", "panic"}) {
		return 7.0
	}

	// Check for file content (likely a read result)
	if len(content) > 200 {
		return 5.0
	}

	return 3.0
}

func referencesFiles(msg provider.Message, files []string) bool {
	for _, block := range msg.Content {
		text := block.Text
		if text == "" {
			text = block.ResultContent
		}
		if text == "" {
			continue
		}
		for _, f := range files {
			if strings.Contains(text, f) {
				return true
			}
		}
	}
	return false
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
