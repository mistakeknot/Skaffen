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

// CompactIntent describes the current work intent, which biases how the
// compaction summary emphasizes different fields. Debugging intent promotes
// errors and test output; building intent promotes files and decisions.
type CompactIntent string

const (
	IntentDefault  CompactIntent = ""          // balanced rendering (same as Format)
	IntentDebugging CompactIntent = "debugging" // emphasize errors, test results, stack traces
	IntentBuilding CompactIntent = "building"  // emphasize files, decisions, acceptance criteria
)

// FormatWithIntent renders the CompactionSummary with field ordering and
// emphasis biased by the given intent. Debugging intent puts errors and
// test results first; building intent puts files and decisions first.
// IntentDefault falls back to the standard Format() ordering.
func (cs CompactionSummary) FormatWithIntent(intent CompactIntent) string {
	type section struct {
		label   string
		content string
	}

	var all []section

	// Always include goal and phase first
	if cs.Goal != "" {
		all = append(all, section{"goal", "**Goal:** " + cs.Goal})
	}
	if cs.Phase != "" {
		all = append(all, section{"phase", "**Phase:** " + cs.Phase})
	}

	// Intent-biased sections: promoted fields come first
	switch intent {
	case IntentDebugging:
		// Errors and test results are most important when debugging
		if len(cs.Errors) > 0 {
			all = append(all, section{"errors", "**Errors encountered:**\n- " + strings.Join(cs.Errors, "\n- ")})
		}
		if cs.TestResults != "" {
			all = append(all, section{"tests", "**Tests:** " + cs.TestResults})
		}
		if len(cs.FilesMutated) > 0 {
			all = append(all, section{"mutated", "**Files modified:** " + strings.Join(dedup(cs.FilesMutated), ", ")})
		}
		if len(cs.Decisions) > 0 {
			all = append(all, section{"decisions", "**Decisions:**\n- " + strings.Join(cs.Decisions, "\n- ")})
		}
		if len(cs.FilesRead) > 0 {
			all = append(all, section{"read", "**Files read:** " + strings.Join(dedup(cs.FilesRead), ", ")})
		}
		if len(cs.OpenQuestions) > 0 {
			all = append(all, section{"questions", "**Open questions:**\n- " + strings.Join(cs.OpenQuestions, "\n- ")})
		}

	case IntentBuilding:
		// Files and decisions are most important when building
		if len(cs.FilesMutated) > 0 {
			all = append(all, section{"mutated", "**Files modified:** " + strings.Join(dedup(cs.FilesMutated), ", ")})
		}
		if len(cs.Decisions) > 0 {
			all = append(all, section{"decisions", "**Decisions:**\n- " + strings.Join(cs.Decisions, "\n- ")})
		}
		if len(cs.FilesRead) > 0 {
			all = append(all, section{"read", "**Files read:** " + strings.Join(dedup(cs.FilesRead), ", ")})
		}
		if cs.TestResults != "" {
			all = append(all, section{"tests", "**Tests:** " + cs.TestResults})
		}
		if len(cs.Errors) > 0 {
			all = append(all, section{"errors", "**Errors encountered:**\n- " + strings.Join(cs.Errors, "\n- ")})
		}
		if len(cs.OpenQuestions) > 0 {
			all = append(all, section{"questions", "**Open questions:**\n- " + strings.Join(cs.OpenQuestions, "\n- ")})
		}

	default:
		return cs.Format()
	}

	if len(all) == 0 {
		return ""
	}
	contents := make([]string, len(all))
	for i, s := range all {
		contents[i] = s.content
	}
	return strings.Join(contents, "\n\n")
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
