package style

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

// Python _empty_mode_profile() keys, in order.
var pythonModeProfileKeys = []string{
	"n", "avg_words", "capitalization_ratio", "emoji_density",
	"pct_contraction", "pct_question", "pct_period", "pct_exclamation",
	"pct_lowercase", "pct_multi_sentence",
	"laughter", "affirmation", "intensifier", "hedge", "opener",
}

var pythonFingerprintKeys = []string{
	"message_count", "global", "modes", "cadence",
}

func TestModeProfileJSONTagParity(t *testing.T) {
	rt := reflect.TypeOf(ModeProfile{})
	if rt.NumField() != 15 {
		t.Fatalf("ModeProfile has %d fields, want 15", rt.NumField())
	}

	var tags []string
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("json")
		if tag == "" {
			t.Errorf("field %s has no json tag", rt.Field(i).Name)
			continue
		}
		tags = append(tags, tag)
	}

	sort.Strings(tags)
	expected := make([]string, len(pythonModeProfileKeys))
	copy(expected, pythonModeProfileKeys)
	sort.Strings(expected)

	if !reflect.DeepEqual(tags, expected) {
		t.Errorf("ModeProfile JSON tags = %v, want %v", tags, expected)
	}
}

func TestFingerprintJSONTagParity(t *testing.T) {
	rt := reflect.TypeOf(Fingerprint{})
	var tags []string
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("json")
		if tag == "" {
			continue // mu has no tag — correct
		}
		tags = append(tags, tag)
	}

	sort.Strings(tags)
	expected := make([]string, len(pythonFingerprintKeys))
	copy(expected, pythonFingerprintKeys)
	sort.Strings(expected)

	if !reflect.DeepEqual(tags, expected) {
		t.Errorf("Fingerprint JSON tags = %v, want %v", tags, expected)
	}
}

func TestNewModeProfileMapsNonNil(t *testing.T) {
	p := NewModeProfile()
	if p.Laughter == nil {
		t.Error("Laughter map is nil")
	}
	if p.Affirmation == nil {
		t.Error("Affirmation map is nil")
	}
	if p.Intensifier == nil {
		t.Error("Intensifier map is nil")
	}
	if p.Hedge == nil {
		t.Error("Hedge map is nil")
	}
	if p.Opener == nil {
		t.Error("Opener map is nil")
	}
}

func TestModeProfileMarshalEmptyMaps(t *testing.T) {
	p := NewModeProfile()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"laughter", "affirmation", "intensifier", "hedge", "opener"} {
		v, ok := raw[key]
		if !ok {
			t.Errorf("key %q missing from marshaled JSON", key)
			continue
		}
		if string(v) != "{}" {
			t.Errorf("key %q = %s, want {}", key, string(v))
		}
	}
}

func TestFingerprintUnmarshalEnsuresMaps(t *testing.T) {
	// JSON with null vocabulary counters — must not panic after unmarshal
	data := `{
		"message_count": 5,
		"global": {"n": 3, "avg_words": 10, "laughter": null, "affirmation": null,
			"intensifier": null, "hedge": null, "opener": null,
			"capitalization_ratio": 0, "emoji_density": 0,
			"pct_contraction": 0, "pct_question": 0, "pct_period": 0,
			"pct_exclamation": 0, "pct_lowercase": 0, "pct_multi_sentence": 0},
		"modes": {"emotional": {"n": 2, "avg_words": 8, "laughter": null,
			"affirmation": null, "intensifier": null, "hedge": null, "opener": null,
			"capitalization_ratio": 0, "emoji_density": 0,
			"pct_contraction": 0, "pct_question": 0, "pct_period": 0,
			"pct_exclamation": 0, "pct_lowercase": 0, "pct_multi_sentence": 0}},
		"cadence": {"avg_burst_size": 1.0, "burst_count": 0}
	}`

	var fp Fingerprint
	if err := json.Unmarshal([]byte(data), &fp); err != nil {
		t.Fatal(err)
	}

	// All maps must be non-nil after unmarshal
	if fp.Global.Laughter == nil {
		t.Error("Global.Laughter is nil after unmarshal")
	}
	if fp.Global.Opener == nil {
		t.Error("Global.Opener is nil after unmarshal")
	}

	ep, ok := fp.Modes[ModeEmotional]
	if !ok {
		t.Fatal("emotional mode missing after unmarshal")
	}
	if ep.Laughter == nil {
		t.Error("emotional.Laughter is nil after unmarshal")
	}

	// Verify we can write to the maps without panic
	fp.Global.Laughter["haha"]++
	ep.Laughter["haha"]++
}

func TestNewFingerprint(t *testing.T) {
	fp := NewFingerprint()
	if fp.Global == nil {
		t.Error("Global is nil")
	}
	if fp.Modes == nil {
		t.Error("Modes is nil")
	}
	if fp.Cadence.AvgBurstSize != 1.0 {
		t.Errorf("Cadence.AvgBurstSize = %f, want 1.0", fp.Cadence.AvgBurstSize)
	}
}
