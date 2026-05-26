package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Soundpost is the canonical lens-response shape. Mirrors
// schemas/lens-response.schema.json in the Auraken Hermes distribution.
//
// JSON-marshalling honors omitempty for the optional fields so empty=true
// responses do not emit lens/rationale/next_question keys at all (matching
// the schema's "lens=null when empty=true" semantics).
type Soundpost struct {
	Empty        bool   `json:"empty"`
	Lens         string `json:"lens,omitempty"`
	Rationale    string `json:"rationale,omitempty"`
	NextQuestion string `json:"next_question,omitempty"`
	Error        string `json:"error,omitempty"`
}

// Validate enforces the schema's allOf+if/then constraints in Go. Returns
// nil iff the response shape is well-formed for the soundpost schema.
func (s Soundpost) Validate() error {
	if s.Empty {
		if s.Lens != "" || s.Rationale != "" || s.NextQuestion != "" {
			return errors.New("empty=true but lens/rationale/next_question is set")
		}
		return nil
	}
	if s.Lens == "" || s.Rationale == "" || s.NextQuestion == "" {
		return errors.New("empty=false requires lens, rationale, and next_question")
	}
	if len(s.Lens) > 128 {
		return fmt.Errorf("lens exceeds 128 char schema limit: %d", len(s.Lens))
	}
	if len(s.Rationale) > 800 {
		return fmt.Errorf("rationale exceeds 800 char schema limit: %d", len(s.Rationale))
	}
	if len(s.NextQuestion) > 400 {
		return fmt.Errorf("next_question exceeds 400 char schema limit: %d", len(s.NextQuestion))
	}
	return nil
}

// parseSoundpost extracts a soundpost JSON object from raw model output.
// The model may wrap its response in prose / markdown fences / json
// preamble; this function locates the outermost JSON object and parses it.
func parseSoundpost(raw string) (Soundpost, error) {
	start := -1
	depth := 0
	inString := false
	escape := false
	var end int
	for i, r := range raw {
		switch {
		case escape:
			escape = false
		case r == '\\' && inString:
			escape = true
		case r == '"':
			inString = !inString
		case inString:
			// nothing
		case r == '{':
			if depth == 0 {
				start = i
			}
			depth++
		case r == '}':
			depth--
			if depth == 0 && start >= 0 {
				end = i + 1
				goto done
			}
		}
	}
done:
	if start < 0 || end <= start {
		return Soundpost{}, errors.New("no JSON object found in model output")
	}
	body := raw[start:end]
	var sp Soundpost
	if err := json.Unmarshal([]byte(body), &sp); err != nil {
		return Soundpost{}, fmt.Errorf("json unmarshal: %w", err)
	}
	if err := sp.Validate(); err != nil {
		return Soundpost{}, fmt.Errorf("schema validation: %w", err)
	}
	return sp, nil
}
