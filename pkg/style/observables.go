package style

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Emoji detection — 9 Unicode ranges matching Python style.py:20-32.
var emojiRE = regexp.MustCompile(
	`[\x{1F600}-\x{1F64F}` +
		`\x{1F300}-\x{1F5FF}` +
		`\x{1F680}-\x{1F6FF}` +
		`\x{1F1E0}-\x{1F1FF}` +
		`\x{2702}-\x{27B0}` +
		`\x{1F900}-\x{1F9FF}` +
		`\x{1FA00}-\x{1FA6F}` +
		`\x{1FA70}-\x{1FAFF}` +
		`\x{2600}-\x{26FF}` +
		`]+`,
)

// Contraction detection — ASCII \w only (documented limitation).
var contractionRE = regexp.MustCompile(`\b\w+'\w+\b`)

// Multi-sentence splitting.
var multiSentenceRE = regexp.MustCompile(`[.!?]+`)

// tokenPattern pairs a compiled regex with a label for vocabulary detection.
type tokenPattern struct {
	pattern *regexp.Regexp
	label   string
}

// Laughter patterns — note: both "haha" and "ahaha" map to label "haha".
// Duplicate labels are intentional — "ahaha" increments "haha" counter by 2.
var laughterPatterns = []tokenPattern{
	{regexp.MustCompile(`(?i)\bhaha(?:ha)*\b`), "haha"},
	{regexp.MustCompile(`(?i)\bahaha(?:ha)*\b`), "haha"},
	{regexp.MustCompile(`(?i)\blol\b`), "lol"},
	{regexp.MustCompile(`(?i)\blmao\b`), "lmao"},
}

var affirmationPatterns = []tokenPattern{
	{regexp.MustCompile(`(?i)^yeah\b`), "yeah"},
	{regexp.MustCompile(`(?i)^yes\b`), "yes"},
	{regexp.MustCompile(`(?i)^yep\b`), "yep"},
	{regexp.MustCompile(`(?i)^ok(?:ay)?\b`), "okay"},
	{regexp.MustCompile(`(?i)^sure\b`), "sure"},
	{regexp.MustCompile(`(?i)^right\b`), "right"},
	{regexp.MustCompile(`(?i)^definitely\b`), "definitely"},
	{regexp.MustCompile(`(?i)^totally\b`), "totally"},
	{regexp.MustCompile(`(?i)^exactly\b`), "exactly"},
	{regexp.MustCompile(`(?i)^absolutely\b`), "absolutely"},
}

var intensifierPatterns = []tokenPattern{
	{regexp.MustCompile(`(?i)\bso\b`), "so"},
	{regexp.MustCompile(`(?i)\breally\b`), "really"},
	{regexp.MustCompile(`(?i)\bvery\b`), "very"},
	{regexp.MustCompile(`(?i)\bsuper\b`), "super"},
	{regexp.MustCompile(`(?i)\bpretty\b`), "pretty"},
	{regexp.MustCompile(`(?i)\bliterally\b`), "literally"},
	{regexp.MustCompile(`(?i)\bactually\b`), "actually"},
	{regexp.MustCompile(`(?i)\bhonestly\b`), "honestly"},
	{regexp.MustCompile(`(?i)\bquite\b`), "quite"},
}

var hedgePatterns = []tokenPattern{
	{regexp.MustCompile(`(?i)\bi think\b`), "i think"},
	{regexp.MustCompile(`(?i)\bi feel like\b`), "i feel like"},
	{regexp.MustCompile(`(?i)\bmaybe\b`), "maybe"},
	{regexp.MustCompile(`(?i)\bprobably\b`), "probably"},
	{regexp.MustCompile(`(?i)\bnot sure\b`), "not sure"},
	{regexp.MustCompile(`(?i)\bi guess\b`), "i guess"},
	{regexp.MustCompile(`(?i)\bkind of\b`), "kind of"},
	{regexp.MustCompile(`(?i)\bi mean\b`), "i mean"},
}

// detectTokens returns all matching labels. Does NOT deduplicate — both
// "haha" patterns firing returns ["haha", "haha"].
func detectTokens(text string, patterns []tokenPattern) []string {
	var labels []string
	for _, tp := range patterns {
		if tp.pattern.MatchString(text) {
			labels = append(labels, tp.label)
		}
	}
	return labels
}

// ComputeObservables extracts style features from a single user message.
//
// Pure function — no locks required, no package-level mutable state after init.
func ComputeObservables(text string) Observables {
	if text == "" {
		return Observables{}
	}

	words := strings.Fields(text)
	wordCount := len(words)
	runeCount := utf8.RuneCountInString(text)

	var alphaCount, upperCount int
	for _, r := range text {
		if unicode.IsLetter(r) {
			alphaCount++
			if unicode.IsUpper(r) {
				upperCount++
			}
		}
	}

	var capRatio float64
	if alphaCount > 0 {
		capRatio = float64(upperCount) / float64(alphaCount)
	}

	emojiMatches := emojiRE.FindAllString(text, -1)
	emojiCount := len(emojiMatches)

	var emojiDensity float64
	if runeCount > 0 {
		emojiDensity = float64(emojiCount) / float64(runeCount)
	}

	opener := ""
	if len(words) > 0 {
		opener = strings.TrimRight(strings.ToLower(words[0]), ",.!?")
	}

	trimmed := strings.TrimSpace(text)
	parts := multiSentenceRE.Split(trimmed, -1)

	return Observables{
		WordCount:          wordCount,
		MessageLength:      runeCount,
		CapitalizationRatio: capRatio,
		EmojiCount:         emojiCount,
		EmojiDensity:       emojiDensity,
		HasContraction:     contractionRE.MatchString(text),
		HasQuestion:        strings.HasSuffix(trimmed, "?"),
		HasPeriod:          strings.HasSuffix(trimmed, "."),
		HasExclamation:     strings.Contains(text, "!"),
		IsAllLowercase:     text == strings.ToLower(text),
		IsMultiSentence:    len(parts) > 2,
		Laughter:           detectTokens(text, laughterPatterns),
		Affirmation:        detectTokens(text, affirmationPatterns),
		Intensifiers:       detectTokens(text, intensifierPatterns),
		Hedges:             detectTokens(text, hedgePatterns),
		Opener:             opener,
		Mode:               ClassifyMode(text),
	}
}
