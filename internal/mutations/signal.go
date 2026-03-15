package mutations

// QualitySignal captures aggregated quality metrics from a session.
// Written by the Compound phase, read by Orient on subsequent sessions.
type QualitySignal struct {
	SessionID string `json:"session_id"`
	Timestamp string `json:"timestamp"`
	Phase     string `json:"phase"` // OODARC phase name (e.g. "compound")
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
