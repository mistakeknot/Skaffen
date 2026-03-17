package tui

import (
	"encoding/json"
	"strings"

	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/session"
)

// compactionState accumulates tool activity across turns so that
// execCompact can build a CompactionSummary without an LLM call.
// Reset after each compaction.
type compactionState struct {
	filesRead    map[string]bool
	filesMutated map[string]bool
	errors       []string
	toolCalls    []string
}

func newCompactionState() *compactionState {
	return &compactionState{
		filesRead:    make(map[string]bool),
		filesMutated: make(map[string]bool),
	}
}

// observeToolComplete records file activity and errors from a tool event.
func (cs *compactionState) observeToolComplete(ev agent.StreamEvent) {
	cs.toolCalls = append(cs.toolCalls, ev.ToolName)

	if ev.IsError && ev.ToolResult != "" {
		// Keep first 200 chars of error for the summary
		errMsg := ev.ToolResult
		if len(errMsg) > 200 {
			errMsg = errMsg[:200] + "..."
		}
		cs.errors = append(cs.errors, ev.ToolName+": "+errMsg)
	}

	// Extract file paths from tool parameters
	path := extractFilePath(ev.ToolName, ev.ToolParams)
	if path == "" {
		return
	}

	switch ev.ToolName {
	case "write", "edit":
		cs.filesMutated[path] = true
	case "read", "glob", "grep":
		cs.filesRead[path] = true
	}
}

// buildSummary constructs a CompactionSummary from accumulated state.
func (cs *compactionState) buildSummary(phase string, turns int) session.CompactionSummary {
	summary := session.CompactionSummary{
		Phase: phase,
		Goal:  "", // filled by caller if available
	}

	for f := range cs.filesRead {
		summary.FilesRead = append(summary.FilesRead, f)
	}
	for f := range cs.filesMutated {
		summary.FilesMutated = append(summary.FilesMutated, f)
	}
	if len(cs.errors) > 0 {
		// Keep up to 10 errors
		n := len(cs.errors)
		if n > 10 {
			n = 10
		}
		summary.Errors = cs.errors[:n]
	}

	return summary
}

// detectIntent infers the compaction intent from OODARC phase and error state.
func (cs *compactionState) detectIntent(phase string) session.CompactIntent {
	// If we have errors, emphasize debugging regardless of phase
	if len(cs.errors) > 0 {
		return session.IntentDebugging
	}

	switch phase {
	case "reflect":
		return session.IntentDebugging
	case "act", "decide":
		return session.IntentBuilding
	default:
		return session.IntentDefault
	}
}

// activeFiles returns the set of files that were mutated in this session.
// Used by ScoreMessages to boost relevance of messages touching these files.
func (cs *compactionState) activeFiles() []string {
	files := make([]string, 0, len(cs.filesMutated))
	for f := range cs.filesMutated {
		files = append(files, f)
	}
	return files
}

// reset clears accumulated state after compaction.
func (cs *compactionState) reset() {
	cs.filesRead = make(map[string]bool)
	cs.filesMutated = make(map[string]bool)
	cs.errors = nil
	cs.toolCalls = nil
}

// extractFilePath pulls a file path from tool parameters JSON.
func extractFilePath(toolName, params string) string {
	if params == "" {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(params), &m); err != nil {
		return ""
	}

	// Try common keys
	for _, key := range []string{"file_path", "path", "pattern"} {
		if v, ok := m[key].(string); ok && v != "" {
			return v
		}
	}

	// For bash, try to extract file paths from the command
	if toolName == "bash" {
		if cmd, ok := m["command"].(string); ok {
			return extractPathFromBash(cmd)
		}
	}

	return ""
}

// extractPathFromBash attempts to extract a file path from a bash command.
// Returns empty string if no clear path is found.
func extractPathFromBash(cmd string) string {
	// Look for "go test" patterns
	if strings.Contains(cmd, "go test") {
		return ""
	}
	// Look for simple file references at the end
	parts := strings.Fields(cmd)
	if len(parts) >= 2 {
		last := parts[len(parts)-1]
		if strings.Contains(last, "/") && !strings.HasPrefix(last, "-") {
			return last
		}
	}
	return ""
}
