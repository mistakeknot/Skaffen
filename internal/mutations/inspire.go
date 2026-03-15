package mutations

import (
	"fmt"
	"os/exec"
	"strings"
)

// Inspiration holds pre-session context gathered for the Orient phase.
type Inspiration struct {
	TaskType    TaskType     `json:"task_type"`
	BestHistory string       `json:"best_history,omitempty"` // from mutations store
	CassSessions string      `json:"cass_sessions,omitempty"` // from cass search
	Suggestions []Suggestion `json:"suggestions,omitempty"`   // from mutation analysis
}

// Inspire gathers inspiration data for a task description.
// Queries mutations store for best approach, cass for related sessions.
// Best-effort: missing sources are skipped, not errors.
func (s *Store) Inspire(taskDescription string) Inspiration {
	tt := ClassifyTask(taskDescription)
	insp := Inspiration{TaskType: tt}

	// 1. Best approach from mutations store
	if summary, err := s.BestSummary(tt); err == nil && summary != "" {
		insp.BestHistory = summary
	}

	// 2. Mutation suggestions
	if suggestions, err := s.Suggest(tt); err == nil {
		insp.Suggestions = suggestions
	}

	// 3. CASS search for related sessions (best-effort, shells out)
	if cassResult := cassSearch(taskDescription); cassResult != "" {
		insp.CassSessions = cassResult
	}

	return insp
}

// ClassifyTask infers a TaskType from a task description string.
func ClassifyTask(description string) TaskType {
	lower := strings.ToLower(description)
	switch {
	case strings.Contains(lower, "fix") || strings.Contains(lower, "bug") || strings.Contains(lower, "broken"):
		return TaskBugFix
	case strings.Contains(lower, "refactor") || strings.Contains(lower, "rename") || strings.Contains(lower, "extract"):
		return TaskRefactor
	case strings.Contains(lower, "optimize") || strings.Contains(lower, "perf") || strings.Contains(lower, "faster"):
		return TaskOptimization
	case strings.Contains(lower, "doc") || strings.Contains(lower, "readme") || strings.Contains(lower, "comment"):
		return TaskDocs
	case strings.Contains(lower, "add") || strings.Contains(lower, "implement") || strings.Contains(lower, "feature") || strings.Contains(lower, "new"):
		return TaskFeature
	default:
		return TaskGeneral
	}
}

// cassSearch shells out to cass for related sessions. Best-effort.
func cassSearch(query string) string {
	cassPath, err := exec.LookPath("cass")
	if err != nil {
		return "" // cass not installed
	}

	// Truncate query to first 100 chars for search
	q := query
	if len(q) > 100 {
		q = q[:100]
	}

	cmd := exec.Command(cassPath, "search", q, "--robot", "--limit", "3")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	result := strings.TrimSpace(string(out))
	if result == "" || result == "[]" {
		return ""
	}
	return result
}

// FormatInspiration returns a compact string for system prompt injection.
func FormatInspiration(insp Inspiration) string {
	if insp.BestHistory == "" && insp.CassSessions == "" && len(insp.Suggestions) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("## Orient Inspiration (task type: %s)", insp.TaskType))

	if insp.BestHistory != "" {
		parts = append(parts, insp.BestHistory)
	}

	if len(insp.Suggestions) > 0 {
		parts = append(parts, FormatSuggestions(insp.Suggestions))
	}

	if insp.CassSessions != "" {
		parts = append(parts, "### Related Sessions (CASS)\n"+insp.CassSessions)
	}

	return strings.Join(parts, "\n\n")
}
