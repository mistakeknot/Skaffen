package extraction

import "testing"

func TestIsLikelyFeedbackPositive(t *testing.T) {
	tests := []string{
		"that was helpful, thanks",
		"that helped a lot",
		"that felt off to me",
		"don't do that again",
		"stop doing that",
		"be more direct please",
		"be less preachy",
		"too many questions",
		"too long",
		"more direct please",
		"that worked well",
		"that didn't work for me",
		"that's not what i meant",
		"wrong approach here",
		"good advice",
		"bad advice",
		"keep doing that",
		"that reframe was perfect",
		"that framework didn't fit",
	}
	for _, text := range tests {
		if !IsLikelyFeedback(text) {
			t.Errorf("IsLikelyFeedback(%q) = false, want true", text)
		}
	}
}

func TestIsLikelyFeedbackNegative(t *testing.T) {
	tests := []string{
		"I had a great day at work",
		"what should I do about this?",
		"I'm feeling stressed",
		"thanks for listening",
		"ok",
	}
	for _, text := range tests {
		if IsLikelyFeedback(text) {
			t.Errorf("IsLikelyFeedback(%q) = true, want false", text)
		}
	}
}

func TestIsLikelyFeedbackCaseInsensitive(t *testing.T) {
	if !IsLikelyFeedback("THAT WAS HELPFUL") {
		t.Error("expected case-insensitive match")
	}
}
