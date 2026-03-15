package mutations

import "fmt"

// Suggestion is a mutation suggestion based on historical quality signals.
type Suggestion struct {
	TaskType    TaskType `json:"task_type"`
	Approach    string   `json:"approach"`    // what to try
	Rationale   string   `json:"rationale"`   // why this might work
	BasedOn     string   `json:"based_on"`    // session ID of reference signal
}

// Suggest generates mutation suggestions for a task type based on historical data.
// Returns suggestions that orient the agent toward better approaches.
func (s *Store) Suggest(tt TaskType) ([]Suggestion, error) {
	front, err := s.BestApproach(tt)
	if err != nil {
		return nil, err
	}

	if len(front) == 0 {
		return []Suggestion{{
			TaskType:  tt,
			Approach:  "No prior data — use default approach",
			Rationale: fmt.Sprintf("This is the first %s task. Establish a baseline.", tt),
		}}, nil
	}

	var suggestions []Suggestion

	// Analyze the Pareto front for patterns
	var totalTurns int
	var totalEfficiency float64
	var errorCount int
	for _, sig := range front {
		totalTurns += sig.Hard.TurnCount
		totalEfficiency += sig.Hard.TokenEfficiency
		if sig.Soft.ToolErrorRate > 0.1 {
			errorCount++
		}
	}
	avgTurns := totalTurns / len(front)
	avgEfficiency := totalEfficiency / float64(len(front))

	// Suggest based on observed patterns
	if avgTurns > 15 {
		suggestions = append(suggestions, Suggestion{
			TaskType:  tt,
			Approach:  "Break into smaller steps — previous sessions averaged " + fmt.Sprintf("%d", avgTurns) + " turns",
			Rationale: "High turn count suggests scope may be too large for single sessions",
			BasedOn:   front[0].SessionID,
		})
	}

	if avgEfficiency < 0.4 {
		suggestions = append(suggestions, Suggestion{
			TaskType:  tt,
			Approach:  "Reduce context — token efficiency is low at " + fmt.Sprintf("%.0f%%", avgEfficiency*100),
			Rationale: "Low output-to-input ratio suggests excessive context loading",
			BasedOn:   front[0].SessionID,
		})
	}

	if errorCount > len(front)/2 {
		suggestions = append(suggestions, Suggestion{
			TaskType:  tt,
			Approach:  "Verify tool availability before heavy tool use",
			Rationale: fmt.Sprintf("%d/%d best sessions had >10%% tool error rate", errorCount, len(front)),
			BasedOn:   front[0].SessionID,
		})
	}

	// Always include a reference to the best session
	best := front[0]
	for _, sig := range front {
		if sig.Hard.TurnCount < best.Hard.TurnCount && sig.Human.Outcome == "success" {
			best = sig
		}
	}
	if best.Human.Outcome == "success" {
		suggestions = append(suggestions, Suggestion{
			TaskType:  tt,
			Approach:  fmt.Sprintf("Reference: session %s completed in %d turns with %.0f%% efficiency", best.SessionID, best.Hard.TurnCount, best.Hard.TokenEfficiency*100),
			Rationale: "Best known successful approach for this task type",
			BasedOn:   best.SessionID,
		})
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, Suggestion{
			TaskType:  tt,
			Approach:  "Continue current approach — no clear improvement signal",
			Rationale: fmt.Sprintf("Pareto front has %d entries, no dominant pattern detected", len(front)),
		})
	}

	return suggestions, nil
}

// FormatSuggestions returns a compact string suitable for system prompt injection.
func FormatSuggestions(suggestions []Suggestion) string {
	if len(suggestions) == 0 {
		return ""
	}
	result := fmt.Sprintf("## Mutation Suggestions (%s)\n", suggestions[0].TaskType)
	for _, s := range suggestions {
		result += fmt.Sprintf("- %s\n  _%s_\n", s.Approach, s.Rationale)
	}
	return result
}
