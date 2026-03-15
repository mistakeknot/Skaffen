package mutations

import "fmt"

// BestApproach returns the Pareto-optimal quality signals for a task type.
// These represent the best historical approaches — no single signal dominates
// another in the result set.
func (s *Store) BestApproach(tt TaskType) ([]QualitySignal, error) {
	signals, err := s.ReadRecentForType(tt, 50)
	if err != nil {
		return nil, fmt.Errorf("read signals for %s: %w", tt, err)
	}
	if len(signals) == 0 {
		return nil, nil
	}
	return ParetoFront(signals), nil
}

// BestSummary returns a human-readable summary of the best approaches for a task type.
func (s *Store) BestSummary(tt TaskType) (string, error) {
	front, err := s.BestApproach(tt)
	if err != nil {
		return "", err
	}
	if len(front) == 0 {
		return fmt.Sprintf("No history for %s tasks yet.", tt), nil
	}

	summary := fmt.Sprintf("Best approaches for %s tasks (%d Pareto-optimal):\n", tt, len(front))
	for i, sig := range front {
		outcome := sig.Human.Outcome
		if outcome == "" {
			outcome = "unknown"
		}
		summary += fmt.Sprintf("  %d. %s — %d turns, efficiency %.2f, error rate %.0f%%, outcome: %s\n",
			i+1, sig.SessionID, sig.Hard.TurnCount, sig.Hard.TokenEfficiency,
			sig.Soft.ToolErrorRate*100, outcome)
	}
	return summary, nil
}
