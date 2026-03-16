package session

import (
	"fmt"
	"strings"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// CompactionSummary provides structured context for multi-scale compaction.
// Instead of a flat summary string, each field captures a distinct aspect
// of the session history so the model can selectively recall what it needs.
//
// Constructed from accumulated Evidence events by the agent loop — not from
// an LLM summary. This makes compaction deterministic and instant.
type CompactionSummary struct {
	Goal          string   // what the agent was working on
	Phase         string   // current OODARC phase
	Decisions     []string // key choices made (e.g., "chose approach A over B")
	FilesRead     []string // files the agent read
	FilesMutated  []string // files the agent wrote or edited
	TestResults   string   // last test outcome summary (pass/fail + count)
	Errors        []string // errors encountered (tool failures, test failures)
	OpenQuestions []string // unresolved issues or pending work
}

// Format renders the CompactionSummary as a structured prompt section.
// Each non-empty field becomes a labeled block. Empty fields are omitted.
func (cs CompactionSummary) Format() string {
	var sections []string

	if cs.Goal != "" {
		sections = append(sections, "**Goal:** "+cs.Goal)
	}
	if cs.Phase != "" {
		sections = append(sections, "**Phase:** "+cs.Phase)
	}
	if len(cs.Decisions) > 0 {
		sections = append(sections, "**Decisions:**\n- "+strings.Join(cs.Decisions, "\n- "))
	}
	if len(cs.FilesRead) > 0 {
		sections = append(sections, "**Files read:** "+strings.Join(dedup(cs.FilesRead), ", "))
	}
	if len(cs.FilesMutated) > 0 {
		sections = append(sections, "**Files modified:** "+strings.Join(dedup(cs.FilesMutated), ", "))
	}
	if cs.TestResults != "" {
		sections = append(sections, "**Tests:** "+cs.TestResults)
	}
	if len(cs.Errors) > 0 {
		sections = append(sections, "**Errors encountered:**\n- "+strings.Join(cs.Errors, "\n- "))
	}
	if len(cs.OpenQuestions) > 0 {
		sections = append(sections, "**Open questions:**\n- "+strings.Join(cs.OpenQuestions, "\n- "))
	}

	if len(sections) == 0 {
		return ""
	}
	return strings.Join(sections, "\n\n")
}

// CompactStructured replaces the conversation history with a structured
// summary message, preserving the most recent keepRecent messages.
// Returns the message count before and after compaction.
func (s *JSONLSession) CompactStructured(summary CompactionSummary, keepRecent int) (before, after int) {
	formatted := summary.Format()
	if formatted == "" {
		// Fall back to no-op if summary is completely empty.
		s.mu.Lock()
		before = len(s.messages)
		after = before
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	before = len(s.messages)
	if before <= keepRecent+1 {
		after = before
		return
	}

	summaryMsg := provider.Message{
		Role: provider.RoleUser,
		Content: []provider.ContentBlock{
			{Type: "text", Text: fmt.Sprintf("[Structured context from earlier conversation]\n\n%s", formatted)},
		},
	}

	var compacted []provider.Message
	compacted = append(compacted, summaryMsg)
	if keepRecent > 0 && keepRecent < before {
		compacted = append(compacted, s.messages[before-keepRecent:]...)
	}
	s.messages = compacted
	after = len(s.messages)
	return
}

// dedup returns a slice with duplicate strings removed, preserving order.
func dedup(items []string) []string {
	seen := make(map[string]bool, len(items))
	var result []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
