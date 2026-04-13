package style

import "testing"

func TestComputeObservablesEmpty(t *testing.T) {
	obs := ComputeObservables("")
	if obs.WordCount != 0 {
		t.Errorf("WordCount = %d, want 0", obs.WordCount)
	}
	if obs.Mode != "" {
		t.Errorf("Mode = %q, want empty", obs.Mode)
	}
}

func TestComputeObservablesBasic(t *testing.T) {
	obs := ComputeObservables("hey how are you")
	if obs.WordCount != 4 {
		t.Errorf("WordCount = %d, want 4", obs.WordCount)
	}
	if obs.Opener != "hey" {
		t.Errorf("Opener = %q, want %q", obs.Opener, "hey")
	}
	if obs.HasQuestion {
		t.Error("HasQuestion should be false (no ? mark)")
	}
}

func TestEmojiDensityUsesRuneCount(t *testing.T) {
	obs := ComputeObservables("hello \U0001F602")
	// "hello 😂" = 7 runes, 1 emoji
	if obs.MessageLength != 7 {
		t.Errorf("MessageLength = %d, want 7 (runes)", obs.MessageLength)
	}
	if obs.EmojiCount != 1 {
		t.Errorf("EmojiCount = %d, want 1", obs.EmojiCount)
	}
	expected := 1.0 / 7.0
	if obs.EmojiDensity != expected {
		t.Errorf("EmojiDensity = %f, want %f (1/7 runes)", obs.EmojiDensity, expected)
	}
}

func TestMessageLengthUsesRuneCount(t *testing.T) {
	obs := ComputeObservables("café")
	if obs.MessageLength != 4 {
		t.Errorf("MessageLength = %d, want 4 (runes)", obs.MessageLength)
	}
}

func TestDuplicateLaughterLabels(t *testing.T) {
	// "ahaha" matches only the ahaha pattern (\b prevents haha matching mid-word).
	obs := ComputeObservables("ahaha")
	if len(obs.Laughter) != 1 {
		t.Errorf("len(Laughter) = %d, want 1", len(obs.Laughter))
	}

	// "haha ahaha" matches both haha and ahaha patterns — both return "haha" label.
	obs = ComputeObservables("haha ahaha")
	if len(obs.Laughter) != 2 {
		t.Errorf("len(Laughter) = %d, want 2 (both haha and ahaha patterns match)", len(obs.Laughter))
	}
	for i, l := range obs.Laughter {
		if l != "haha" {
			t.Errorf("Laughter[%d] = %q, want %q", i, l, "haha")
		}
	}
}

func TestOpenerLeadingWhitespace(t *testing.T) {
	obs := ComputeObservables("  Hey how")
	if obs.Opener != "hey" {
		t.Errorf("Opener = %q, want %q", obs.Opener, "hey")
	}
}

func TestOpenerStripsTrailingPunctuation(t *testing.T) {
	obs := ComputeObservables("hi, how are you")
	if obs.Opener != "hi" {
		t.Errorf("Opener = %q, want %q", obs.Opener, "hi")
	}
}

func TestMultiSentenceBoundary(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"Hello!", false},
		{"Hello! Bye.", true},
		{"Just one sentence", false},
		{"One. Two. Three.", true},
	}
	for _, tt := range tests {
		obs := ComputeObservables(tt.text)
		if obs.IsMultiSentence != tt.want {
			t.Errorf("IsMultiSentence(%q) = %v, want %v", tt.text, obs.IsMultiSentence, tt.want)
		}
	}
}

func TestContractionDetection(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"I'm fine", true},
		{"I am fine", false},
		{"don't worry", true},
		{"do not worry", false},
	}
	for _, tt := range tests {
		obs := ComputeObservables(tt.text)
		if obs.HasContraction != tt.want {
			t.Errorf("HasContraction(%q) = %v, want %v", tt.text, obs.HasContraction, tt.want)
		}
	}
}

func TestIsAllLowercase(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"hey how are you", true},
		{"Hey how are you", false},
		{"ALL CAPS", false},
	}
	for _, tt := range tests {
		obs := ComputeObservables(tt.text)
		if obs.IsAllLowercase != tt.want {
			t.Errorf("IsAllLowercase(%q) = %v, want %v", tt.text, obs.IsAllLowercase, tt.want)
		}
	}
}

func TestIntensifiers(t *testing.T) {
	obs := ComputeObservables("i'm really so happy")
	if len(obs.Intensifiers) != 2 {
		t.Errorf("len(Intensifiers) = %d, want 2", len(obs.Intensifiers))
	}
}

func TestHedges(t *testing.T) {
	obs := ComputeObservables("i think maybe we should")
	if len(obs.Hedges) != 2 {
		t.Errorf("len(Hedges) = %d, want 2", len(obs.Hedges))
	}
}
