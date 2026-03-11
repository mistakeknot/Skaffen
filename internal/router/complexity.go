package router

// ComplexityOverride records what the complexity layer would/did change.
type ComplexityOverride struct {
	Tier         int    `json:"complexity_tier"`
	WouldPromote bool   `json:"would_promote,omitempty"`
	WouldDemote  bool   `json:"would_demote,omitempty"`
	Applied      bool   `json:"complexity_override"`
	OrigModel    string `json:"original_model,omitempty"`
}

// ComplexityClassifier classifies prompt complexity and optionally overrides model selection.
type ComplexityClassifier struct {
	mode string // "shadow" or "enforce"
}

func newComplexityClassifier(cfg *ComplexityConfig) *ComplexityClassifier {
	mode := cfg.Mode
	if mode == "" {
		mode = "shadow"
	}
	return &ComplexityClassifier{mode: mode}
}

// Classify returns a complexity tier (1-5) based on input token count.
// C1: <300, C2: <800, C3: <2000, C4: <4000, C5: 4000+
func (cc *ComplexityClassifier) Classify(inputTokens int) int {
	switch {
	case inputTokens < 300:
		return 1
	case inputTokens < 800:
		return 2
	case inputTokens < 2000:
		return 3
	case inputTokens < 4000:
		return 4
	default:
		return 5
	}
}

// MaybeOverride returns a (possibly overridden) model based on complexity.
// In shadow mode, returns the original model but still provides override info for logging.
// In enforce mode, C4-C5 promote to opus, C1-C2 demote to haiku.
func (cc *ComplexityClassifier) MaybeOverride(model, reason string, inputTokens int) (string, string, *ComplexityOverride) {
	tier := cc.Classify(inputTokens)
	override := &ComplexityOverride{
		Tier:      tier,
		OrigModel: model,
	}

	// Determine what would change
	if tier >= 4 {
		override.WouldPromote = true
	} else if tier <= 2 {
		override.WouldDemote = true
	}

	if cc.mode == "shadow" {
		// Log what would change, but don't apply
		override.Applied = false
		return model, reason, override
	}

	// Enforce mode: apply overrides
	if tier >= 4 && model != ModelOpus {
		override.Applied = true
		return ModelOpus, "complexity-promote", override
	}
	if tier <= 2 && model != ModelHaiku {
		override.Applied = true
		return ModelHaiku, "complexity-demote", override
	}

	override.Applied = false
	return model, reason, override
}
