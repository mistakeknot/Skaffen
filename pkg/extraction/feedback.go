package extraction

import "regexp"

// Feedback detection patterns — ported from Python style.py:134-165.
// Compiled at package init; read-only after init.
var feedbackRE *regexp.Regexp

func init() {
	patterns := []string{
		`(?i)\bthat was helpful\b`,
		`(?i)\bthat helped\b`,
		`(?i)\bthat felt off\b`,
		`(?i)\bthat felt wrong\b`,
		`(?i)\bdon'?t do that\b`,
		`(?i)\bstop doing\b`,
		`(?i)\bstop asking\b`,
		`(?i)\bbe more\b`,
		`(?i)\bbe less\b`,
		`(?i)\btoo many questions\b`,
		`(?i)\btoo long\b`,
		`(?i)\btoo short\b`,
		`(?i)\btoo preachy\b`,
		`(?i)\btoo vague\b`,
		`(?i)\btoo abstract\b`,
		`(?i)\bmore direct\b`,
		`(?i)\bless direct\b`,
		`(?i)\bnot helpful\b`,
		`(?i)\bthat worked\b`,
		`(?i)\bthat didn'?t work\b`,
		`(?i)\bgive.* feedback\b`,
		`(?i)\bhow can i give you feedback\b`,
		`(?i)\bthat'?s not what i\b`,
		`(?i)\bi didn'?t ask for\b`,
		`(?i)\bwrong approach\b`,
		`(?i)\bgood advice\b`,
		`(?i)\bbad advice\b`,
		`(?i)\bkeep doing\b`,
		`(?i)\bthat reframe\b`,
		`(?i)\bthat framework\b`,
	}

	combined := ""
	for i, p := range patterns {
		if i > 0 {
			combined += "|"
		}
		combined += p
	}
	feedbackRE = regexp.MustCompile(combined)
}

// IsLikelyFeedback detects if a message contains meta-feedback signals
// about the bot's behavior.
func IsLikelyFeedback(text string) bool {
	return feedbackRE.MatchString(text)
}
