package style

import (
	"fmt"
	"strings"
)

// laughAlt returns an alternative laughter token to contrast with.
func laughAlt(top string) string {
	alts := map[string]string{"haha": "lol", "lol": "haha", "lmao": "haha"}
	if alt, ok := alts[top]; ok {
		return alt
	}
	return "lol"
}

// modeContextNote returns a mode-specific behavioral note.
// Strings copied character-for-character from Python style.py:470-496.
func modeContextNote(mode Mode) string {
	switch mode {
	case ModeEmotional:
		return "The user is processing emotions. Be warm and present. " +
			"Don't rush to frameworks — hold space before offering perspective."
	case ModeAnalytical:
		return "The user is in analytical mode. Match their precision. " +
			"Frameworks and structured thinking are welcome here."
	case ModePlayful:
		return "The user is being playful. Be light and brief. " +
			"Match their energy — don't over-explain or get heavy."
	case ModeIntimate:
		return "The user is being personal and warm. " +
			"Be genuine and affectionate in tone. Don't be clinical."
	case ModeLogistics:
		return "The user is coordinating/planning. Be direct and helpful. " +
			"Answer the question, don't philosophize."
	case ModeUpdate:
		return "The user is sharing an update or checking in. " +
			"Be curious and responsive, not analytical."
	default:
		return ""
	}
}

// profileSnapshot holds a copy of ModeProfile fields for lock-free text generation.
type profileSnapshot struct {
	N                   int
	AvgWords            float64
	PctLowercase        float64
	PctPeriod           float64
	PctExclamation      float64
	PctContraction      float64
	Laughter            map[string]int
	Affirmation         map[string]int
	Intensifier         map[string]int
	Hedge               map[string]int
}

// copyProfile copies ModeProfile fields into a snapshot. Maps are deep-copied.
func copyProfile(p *ModeProfile) profileSnapshot {
	s := profileSnapshot{
		N:              p.N,
		AvgWords:       p.AvgWords,
		PctLowercase:   p.PctLowercase,
		PctPeriod:      p.PctPeriod,
		PctExclamation: p.PctExclamation,
		PctContraction: p.PctContraction,
		Laughter:       make(map[string]int, len(p.Laughter)),
		Affirmation:    make(map[string]int, len(p.Affirmation)),
		Intensifier:    make(map[string]int, len(p.Intensifier)),
		Hedge:          make(map[string]int, len(p.Hedge)),
	}
	for k, v := range p.Laughter {
		s.Laughter[k] = v
	}
	for k, v := range p.Affirmation {
		s.Affirmation[k] = v
	}
	for k, v := range p.Intensifier {
		s.Intensifier[k] = v
	}
	for k, v := range p.Hedge {
		s.Hedge[k] = v
	}
	return s
}

// maxKey returns the key with the highest value in a map.
func maxKey(m map[string]int) (string, int) {
	var bestKey string
	var bestVal int
	for k, v := range m {
		if bestKey == "" || v > bestVal {
			bestKey = k
			bestVal = v
		}
	}
	return bestKey, bestVal
}

// BuildMirroring generates specific mirroring instructions from the fingerprint.
// Thread-safe: copies fields under lock, generates text after release.
func (f *Fingerprint) BuildMirroring(mode Mode) string {
	f.mu.Lock()

	if f.MessageCount < 3 {
		f.mu.Unlock()
		return ""
	}

	// Select profile with fallback chain
	profile := f.Modes[mode]
	if profile == nil || profile.N < 2 {
		profile = f.Global
	}
	if profile == nil || profile.N < 3 {
		f.mu.Unlock()
		return ""
	}

	// Copy fields under lock
	snap := copyProfile(profile)
	cadence := f.Cadence

	f.mu.Unlock()

	// Generate instructions from snapshot (no lock held)
	var instructions []string

	// Length
	if snap.AvgWords <= 5 {
		instructions = append(instructions, "Keep responses very short — 1-2 sentences.")
	} else if snap.AvgWords <= 10 {
		instructions = append(instructions, "Keep responses concise — 2-3 sentences max.")
	} else if snap.AvgWords <= 20 {
		instructions = append(instructions, "Medium-length responses are fine — a few sentences.")
	} else {
		instructions = append(instructions, "This person writes longer messages. Match their depth.")
	}

	// Capitalization
	if snap.PctLowercase > 0.6 {
		instructions = append(instructions, "Use lowercase. Don't capitalize unless they do.")
	} else if snap.PctLowercase < 0.2 {
		instructions = append(instructions, "Use proper capitalization — this person does.")
	}

	// Punctuation
	if snap.PctPeriod < 0.05 {
		instructions = append(instructions, "Don't end messages with periods.")
	}
	if snap.PctExclamation > 0.25 {
		instructions = append(instructions, "Exclamation marks are natural for this person — use them warmly.")
	} else if snap.PctExclamation < 0.05 {
		instructions = append(instructions, "Avoid exclamation marks — this person rarely uses them.")
	}

	// Contractions
	if snap.PctContraction > 0.15 {
		instructions = append(instructions, "Always use contractions (I'm, that's, can't, don't).")
	} else if snap.PctContraction < 0.05 {
		instructions = append(instructions, "Avoid contractions — this person writes them out.")
	}

	// Laughter
	if len(snap.Laughter) > 0 {
		topLaugh, _ := maxKey(snap.Laughter)
		total := 0
		for _, v := range snap.Laughter {
			total += v
		}
		if total > 0 {
			dominance := float64(snap.Laughter[topLaugh]) / float64(total)
			if dominance > 0.7 {
				alt := laughAlt(topLaugh)
				instructions = append(instructions,
					fmt.Sprintf("If something is funny, use '%s' — never '%s'.", topLaugh, alt))
			}
		}
	}

	// Affirmation
	if len(snap.Affirmation) > 0 {
		topAffirm, _ := maxKey(snap.Affirmation)
		instructions = append(instructions,
			fmt.Sprintf("For agreement, prefer '%s' over other affirmations.", topAffirm))
	}

	// Intensifier
	if len(snap.Intensifier) > 0 {
		topIntense, _ := maxKey(snap.Intensifier)
		total := 0
		for _, v := range snap.Intensifier {
			total += v
		}
		if total > 0 {
			dominance := float64(snap.Intensifier[topIntense]) / float64(total)
			if dominance > 0.3 {
				instructions = append(instructions,
					fmt.Sprintf("Primary intensifier: '%s'.", topIntense))
			}
		}
	}

	// Hedge
	if len(snap.Hedge) > 0 {
		topHedge, _ := maxKey(snap.Hedge)
		if mode == ModeEmotional {
			if _, ok := snap.Hedge["i feel like"]; ok {
				instructions = append(instructions, "When hedging, use 'i feel like' not 'i think'.")
			} else if topHedge != "" {
				instructions = append(instructions,
					fmt.Sprintf("When hedging, prefer '%s'.", topHedge))
			}
		} else if topHedge != "" {
			instructions = append(instructions,
				fmt.Sprintf("When hedging, prefer '%s'.", topHedge))
		}
	}

	// Cadence
	if cadence.AvgBurstSize >= 1.8 {
		instructions = append(instructions,
			"This person sends rapid-fire short messages instead of one long one. "+
				"Mirror this: send 2-3 short messages, not one paragraph.")
	}

	// Mode context
	if note := modeContextNote(mode); note != "" {
		instructions = append(instructions, note)
	}

	if len(instructions) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n## Communication Style — Mirror This Person\n")
	for i, inst := range instructions {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("- ")
		b.WriteString(inst)
	}
	b.WriteString("\n\nMirror their style by default. Use their vocabulary and tone.\n")
	return b.String()
}

// BuildInstantMirroring generates mirroring instructions from a single message.
// Used for early conversations (< 3 messages) before the fingerprint has enough data.
// Fixes Python bug at lines 536-542 (`.keys()` called on a list).
func BuildInstantMirroring(text string) string {
	obs := ComputeObservables(text)
	if obs.WordCount == 0 {
		return ""
	}

	var instructions []string

	// Length
	if obs.WordCount <= 5 {
		instructions = append(instructions,
			fmt.Sprintf("They wrote ~%d words. Reply in 1-2 short sentences max.", obs.WordCount))
	} else if obs.WordCount <= 15 {
		instructions = append(instructions,
			fmt.Sprintf("They wrote ~%d words. Keep your reply about the same length.", obs.WordCount))
	}

	// Case
	if obs.IsAllLowercase {
		instructions = append(instructions, "They write in all lowercase — you do too. no caps.")
	} else if obs.CapitalizationRatio < 0.05 {
		instructions = append(instructions, "Minimal capitalization. Keep it casual.")
	}

	// Punctuation
	if !obs.HasPeriod && !obs.HasExclamation && !obs.HasQuestion {
		instructions = append(instructions, "No end punctuation. Don't add periods or exclamation marks.")
	} else if obs.HasExclamation {
		instructions = append(instructions, "They use exclamation marks — match that energy.")
	}

	// Contractions
	if obs.HasContraction {
		instructions = append(instructions, "They use contractions. Always use contractions.")
	}

	// Vocabulary — fixed from Python: use slice index, not .keys()
	if len(obs.Laughter) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("They use '%s' — use that, not alternatives.", obs.Laughter[0]))
	}
	if len(obs.Affirmation) > 0 {
		instructions = append(instructions,
			fmt.Sprintf("They say '%s' — mirror that.", obs.Affirmation[0]))
	}

	// Mode
	if note := modeContextNote(obs.Mode); note != "" {
		instructions = append(instructions, note)
	}

	if len(instructions) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n## Communication Style — Match This Message\n")
	for i, inst := range instructions {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("- ")
		b.WriteString(inst)
	}
	b.WriteByte('\n')
	return b.String()
}

// DetectCurrentMode detects the current conversation mode from recent messages
// using majority vote over the last `window` messages.
//
// Does not hold the Fingerprint mutex. The caller must ensure the messages slice
// is not concurrently modified.
func DetectCurrentMode(messages []string, window int) Mode {
	if len(messages) == 0 {
		return ModeGeneral
	}

	start := len(messages) - window
	if start < 0 {
		start = 0
	}
	recent := messages[start:]

	counts := make(map[Mode]int)
	for _, m := range recent {
		mode := ClassifyMode(m)
		if mode != ModeGeneral {
			counts[mode]++
		}
	}

	if len(counts) == 0 {
		return ModeGeneral
	}

	// Find max using modePriority for deterministic tie-breaking
	bestMode := ModeGeneral
	bestCount := 0
	for _, mode := range modePriority {
		if c, ok := counts[mode]; ok && c > bestCount {
			bestCount = c
			bestMode = mode
		}
	}
	return bestMode
}
