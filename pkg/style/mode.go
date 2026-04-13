package style

import "regexp"

// Mode signal weights — match Python style.py:39-92.
const (
	weightEmotional = 3
	weightAnalytical = 2
	weightDefault   = 1
)

type modeSignal struct {
	weight   int
	patterns []*regexp.Regexp
}

// modeSignals maps each mode to its weighted pattern set.
// Populated in init(); read-only after init — safe for concurrent access.
var modeSignals map[Mode]modeSignal

func init() {
	modeSignals = map[Mode]modeSignal{
		ModeEmotional: {weightEmotional, compileAll(
			`(?i)\bi feel\b`,
			`(?i)\bi'm feeling\b`,
			`(?i)\bi'm (?:sad|anxious|scared|overwhelmed|angry|hurt|lonely|frustrated|stressed|worried|upset)\b`,
			`(?i)\bit hurts\b`,
			`(?i)\bi cried\b`,
			`(?i)\bi'm crying\b`,
			`(?i)\bi'm struggling\b`,
			`(?i)\bi'm going through\b`,
			`(?i)\bi miss\b`,
			`(?i)\bi'm afraid\b`,
			`(?i)\bi don't know what to do\b`,
			`(?i)\bi can't handle\b`,
			`(?i)\bare you okay\b`,
			`(?i)\bhow are you (?:doing|holding|feeling)\b`,
			`(?i)\btherapy\b`,
			`(?i)\btherapist\b`,
			`(?i)\banxiety\b`,
		)},
		ModeAnalytical: {weightAnalytical, compileAll(
			`(?i)\bthe (?:problem|issue|challenge|question) is\b`,
			`(?i)\bpattern\b`,
			`(?i)\bframework\b`,
			`(?i)\btrade-?off\b`,
			`(?i)\bincentive\b`,
			`(?i)\bconstraint\b`,
			`(?i)\bhypothesis\b`,
			`(?i)\bevidence\b`,
			`(?i)\bthe reason\b`,
			`(?i)\broot cause\b`,
			`(?i)\bstrateg(?:y|ic)\b`,
			`(?i)\bperspective\b`,
			`(?i)\bassumption\b`,
			`(?i)\balthough\b`,
			`(?i)\bhowever\b`,
			`(?i)\bnevertheless\b`,
			`(?i)\bon the other hand\b`,
			`(?i)\bthat said\b`,
		)},
		ModePlayful: {weightDefault, compileAll(
			`(?i)\bhaha(?:ha)*\b`,
			`(?i)\bahaha\b`,
			`(?i)\blol\b`,
			`(?i)\blmao\b`,
			`(?i)\b(?:omg|omfg)\b`,
			`[\x{1F602}\x{1F923}\x{1F62D}\x{1F480}]`,
			`(?i)\bwild\b`,
			`(?i)\bchaos\b`,
			`(?i)\bcursed\b`,
		)},
		ModeIntimate: {weightDefault, compileAll(
			`(?i)\bi love you\b`,
			`(?i)\bmiss you\b`,
			`(?i)\bbaby\b`,
			`(?i)\bbabe\b`,
			`(?i)\bkiss\b`,
			`(?i)\bcuddle\b`,
			`(?i)\bhold(?:ing)? (?:you|me)\b`,
			`(?i)\bsleep well\b`,
			`(?i)\bsweet dreams\b`,
			`(?i)\baw+\b`,
			`[\x{2764}\x{1F495}\x{1F497}\x{1F498}\x{1F618}\x{1F970}\x{1F48B}]`,
			`(?i)\bsmitten\b`,
			`(?i)\badore\b`,
			`(?i)\bcan't stop thinking\b`,
		)},
		ModeLogistics: {weightDefault, compileAll(
			`(?i)\bwhat time\b`,
			`(?i)\bwhen (?:are|is|do|should|can|will)\b`,
			`(?i)\b\d{1,2}(?::\d{2})?\s*(?:am|pm)\b`,
			`(?i)\btomorrow\b`,
			`(?i)\btonight\b`,
			`(?i)\bthis (?:weekend|week)\b`,
			`(?i)\bnext (?:week|month|weekend)\b`,
			`(?i)\breservation\b`,
			`(?i)\bbooked\b`,
			`(?i)\baddress\b`,
			`(?i)\bsounds good\b`,
			`(?i)\bworks for me\b`,
			`(?i)\bi'm free\b`,
			`(?i)\bconfirm\b`,
			`(?i)\breschedule\b`,
		)},
		ModeUpdate: {weightDefault, compileAll(
			`(?i)^(?:hey|hi|hello|morning|good morning)\b`,
			`(?i)\bhow(?:'s| is| was) (?:your|the)\b`,
			`(?i)\bjust (?:got|finished|left|arrived|woke|started)\b`,
			`(?i)\bon my way\b`,
			`(?i)\bheading (?:out|home|to)\b`,
			`(?i)\bfyi\b`,
			`(?i)\bjust wanted to\b`,
			`(?i)\bwork was\b`,
			`(?i)\bday was\b`,
		)},
	}
}

func compileAll(patterns ...string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		compiled[i] = regexp.MustCompile(p)
	}
	return compiled
}

// ClassifyMode classifies a message into a conversation mode using weighted
// regex scoring. Ties are broken by canonical priority order (emotional first).
//
// Pure function — no locks required, no package-level mutable state after init.
func ClassifyMode(text string) Mode {
	bestMode := ModeGeneral
	bestScore := 0

	for _, mode := range modePriority {
		sig, ok := modeSignals[mode]
		if !ok {
			continue
		}
		hits := 0
		for _, p := range sig.patterns {
			if p.MatchString(text) {
				hits++
			}
		}
		score := hits * sig.weight
		if score > bestScore {
			bestScore = score
			bestMode = mode
		}
	}
	return bestMode
}
