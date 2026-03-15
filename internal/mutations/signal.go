package mutations

// TaskType categorizes the kind of work a session performed.
type TaskType string

const (
	TaskBugFix       TaskType = "bug-fix"
	TaskFeature      TaskType = "feature"
	TaskRefactor     TaskType = "refactor"
	TaskOptimization TaskType = "optimization"
	TaskDocs         TaskType = "docs"
	TaskGeneral      TaskType = "general" // fallback when type can't be determined
)

// QualitySignal captures aggregated quality metrics from a session.
// Written by the Compound phase, read by Orient on subsequent sessions.
type QualitySignal struct {
	SessionID string   `json:"session_id"`
	Timestamp string   `json:"timestamp"`
	Phase     string   `json:"phase"`     // OODARC phase name (e.g. "compound")
	TaskType  TaskType `json:"task_type"` // categorization for per-type tracking
	Hard      HardSignals  `json:"hard"`
	Soft      SoftSignals  `json:"soft"`
	Human     HumanSignals `json:"human"`
}

// HardSignals are objective, automated measurements.
type HardSignals struct {
	TestsPassed     *bool   `json:"tests_passed,omitempty"`
	BuildSuccess    *bool   `json:"build_success,omitempty"`
	TokenEfficiency float64 `json:"token_efficiency"`
	TurnCount       int     `json:"turn_count"`
}

// SoftSignals are derived from tool and agent behavior.
type SoftSignals struct {
	ComplexityTier  int     `json:"complexity_tier"`
	ToolErrorRate   float64 `json:"tool_error_rate"`
	ToolDenialRate  float64 `json:"tool_denial_rate"`
}

// HumanSignals are qualitative signals from user interaction.
type HumanSignals struct {
	ApprovalRate float64 `json:"approval_rate"`
	Outcome      string  `json:"outcome"`
}

// Scores returns a flat slice of numeric quality scores for Pareto comparison.
// Higher is better for all dimensions. Returns [hard..., soft..., human...].
//
// Hard: token_efficiency (higher = more output per input), -turn_count (fewer = better)
// Soft: -tool_error_rate (lower = better), -tool_denial_rate (lower = better)
// Human: approval_rate (higher = better), outcome_score (1 if success, 0 otherwise)
func (s *QualitySignal) Scores() []float64 {
	outcomeScore := 0.0
	if s.Human.Outcome == "success" {
		outcomeScore = 1.0
	}
	return []float64{
		s.Hard.TokenEfficiency,
		-float64(s.Hard.TurnCount), // fewer turns = better
		-s.Soft.ToolErrorRate,      // lower error = better
		-s.Soft.ToolDenialRate,     // lower denial = better
		s.Human.ApprovalRate,
		outcomeScore,
	}
}

// Dominates returns true if signal a Pareto-dominates signal b:
// a is at least as good as b on every dimension AND strictly better on at least one.
func (a *QualitySignal) Dominates(b *QualitySignal) bool {
	as, bs := a.Scores(), b.Scores()
	if len(as) != len(bs) {
		return false
	}
	strictlyBetter := false
	for i := range as {
		if as[i] < bs[i] {
			return false // worse on at least one dimension
		}
		if as[i] > bs[i] {
			strictlyBetter = true
		}
	}
	return strictlyBetter
}

// ParetoFront returns the non-dominated signals from a set.
// Result is the Pareto frontier: no signal in the result is dominated by another.
func ParetoFront(signals []QualitySignal) []QualitySignal {
	if len(signals) <= 1 {
		return signals
	}
	var front []QualitySignal
	for i := range signals {
		dominated := false
		for j := range signals {
			if i != j && signals[j].Dominates(&signals[i]) {
				dominated = true
				break
			}
		}
		if !dominated {
			front = append(front, signals[i])
		}
	}
	return front
}
