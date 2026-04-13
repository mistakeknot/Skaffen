package style

import (
	"encoding/json"
	"sync"
)

// Mode represents a conversation mode detected from message content.
type Mode string

const (
	ModeEmotional Mode = "emotional"
	ModeAnalytical Mode = "analytical"
	ModePlayful   Mode = "playful"
	ModeIntimate  Mode = "intimate"
	ModeLogistics Mode = "logistics"
	ModeUpdate    Mode = "update"
	ModeGeneral   Mode = "general"
)

// modePriority defines the canonical tie-breaking order for mode classification.
// Matches Python _RAW_SIGNALS insertion order.
var modePriority = []Mode{
	ModeEmotional,
	ModeAnalytical,
	ModePlayful,
	ModeIntimate,
	ModeLogistics,
	ModeUpdate,
}

// Observables holds per-message style features extracted by ComputeObservables.
// This struct is transient (not stored in JSONB).
type Observables struct {
	WordCount          int      `json:"word_count"`
	MessageLength      int      `json:"message_length"`
	CapitalizationRatio float64 `json:"capitalization_ratio"`
	EmojiCount         int      `json:"emoji_count"`
	EmojiDensity       float64  `json:"emoji_density"`
	HasContraction     bool     `json:"has_contraction"`
	HasQuestion        bool     `json:"has_question"`
	HasPeriod          bool     `json:"has_period"`
	HasExclamation     bool     `json:"has_exclamation"`
	IsAllLowercase     bool     `json:"is_all_lowercase"`
	IsMultiSentence    bool     `json:"is_multi_sentence"`
	Laughter           []string `json:"laughter"`
	Affirmation        []string `json:"affirmation"`
	Intensifiers       []string `json:"intensifiers"`
	Hedges             []string `json:"hedges"`
	Opener             string   `json:"opener"`
	Mode               Mode     `json:"mode"`
}

// ModeProfile holds per-mode EMA accumulators and vocabulary counters.
// JSON tags match Python _empty_mode_profile() keys exactly for wire compatibility.
type ModeProfile struct {
	N                   int            `json:"n"`
	AvgWords            float64        `json:"avg_words"`
	CapitalizationRatio float64        `json:"capitalization_ratio"`
	EmojiDensity        float64        `json:"emoji_density"`
	PctContraction      float64        `json:"pct_contraction"`
	PctQuestion         float64        `json:"pct_question"`
	PctPeriod           float64        `json:"pct_period"`
	PctExclamation      float64        `json:"pct_exclamation"`
	PctLowercase        float64        `json:"pct_lowercase"`
	PctMultiSentence    float64        `json:"pct_multi_sentence"`
	Laughter            map[string]int `json:"laughter"`
	Affirmation         map[string]int `json:"affirmation"`
	Intensifier         map[string]int `json:"intensifier"`
	Hedge               map[string]int `json:"hedge"`
	Opener              map[string]int `json:"opener"`
}

// NewModeProfile returns a ModeProfile with all maps initialized (never nil).
func NewModeProfile() *ModeProfile {
	return &ModeProfile{
		Laughter:    make(map[string]int),
		Affirmation: make(map[string]int),
		Intensifier: make(map[string]int),
		Hedge:       make(map[string]int),
		Opener:      make(map[string]int),
	}
}

// ensureMaps guarantees all vocabulary counter maps are non-nil.
// Called after JSON unmarshal to prevent nil-map panics.
func (p *ModeProfile) ensureMaps() {
	if p.Laughter == nil {
		p.Laughter = make(map[string]int)
	}
	if p.Affirmation == nil {
		p.Affirmation = make(map[string]int)
	}
	if p.Intensifier == nil {
		p.Intensifier = make(map[string]int)
	}
	if p.Hedge == nil {
		p.Hedge = make(map[string]int)
	}
	if p.Opener == nil {
		p.Opener = make(map[string]int)
	}
}

// Cadence tracks message burst patterns.
type Cadence struct {
	AvgBurstSize float64 `json:"avg_burst_size"`
	BurstCount   int     `json:"burst_count"`
}

// Fingerprint is the top-level style fingerprint stored in JSONB.
// All mutation methods are thread-safe via internal mutex.
type Fingerprint struct {
	mu           sync.Mutex
	MessageCount int                  `json:"message_count"`
	Global       *ModeProfile         `json:"global"`
	Modes        map[Mode]*ModeProfile `json:"modes"`
	Cadence      Cadence              `json:"cadence"`
}

// NewFingerprint returns an initialized Fingerprint ready for use.
func NewFingerprint() *Fingerprint {
	return &Fingerprint{
		Global:  NewModeProfile(),
		Modes:   make(map[Mode]*ModeProfile),
		Cadence: Cadence{AvgBurstSize: 1.0},
	}
}

// UnmarshalJSON implements custom JSON unmarshaling that acquires the mutex
// and ensures all maps are non-nil after deserialization.
func (f *Fingerprint) UnmarshalJSON(data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	type alias Fingerprint
	aux := &struct {
		*alias
	}{
		alias: (*alias)(f),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	if f.Global == nil {
		f.Global = NewModeProfile()
	} else {
		f.Global.ensureMaps()
	}
	if f.Modes == nil {
		f.Modes = make(map[Mode]*ModeProfile)
	}
	for _, p := range f.Modes {
		if p != nil {
			p.ensureMaps()
		}
	}
	return nil
}
