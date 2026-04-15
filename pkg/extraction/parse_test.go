package extraction

import "testing"

func TestParseExtractionCleanJSON(t *testing.T) {
	raw := `{"has_preference_signal": true, "entities": [{"domain": "goals", "type": "career", "value": "wants to lead a team", "valence": "positive", "origin": "stated", "confidence": 0.9, "action": "ADD"}]}`
	result, err := ParseExtractionResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasPreferenceSignal {
		t.Error("expected has_preference_signal=true")
	}
	if len(result.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(result.Entities))
	}
	if result.Entities[0].Domain != "goals" {
		t.Errorf("domain = %q, want %q", result.Entities[0].Domain, "goals")
	}
	if result.Entities[0].Action != ActionAdd {
		t.Errorf("action = %q, want %q", result.Entities[0].Action, ActionAdd)
	}
}

func TestParseExtractionMarkdownFenced(t *testing.T) {
	raw := "```json\n{\"has_preference_signal\": false, \"entities\": []}\n```"
	result, err := ParseExtractionResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.HasPreferenceSignal {
		t.Error("expected has_preference_signal=false")
	}
}

func TestParseExtractionMalformed(t *testing.T) {
	raw := "I'm sorry, I can't extract preferences from this."
	_, err := ParseExtractionResponse(raw)
	if err == nil {
		t.Error("expected error for malformed input")
	}
}

func TestParseExtractionNoSignal(t *testing.T) {
	raw := `{"has_preference_signal": false, "entities": []}`
	result, err := ParseExtractionResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.HasPreferenceSignal {
		t.Error("expected false")
	}
	if len(result.Entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(result.Entities))
	}
}

func TestParseFeedbackCleanJSON(t *testing.T) {
	raw := `{"has_feedback": true, "entities": [{"type": "delivery", "value": "be more direct", "valence": "negative", "confidence": 0.85}]}`
	result, err := ParseFeedbackResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasFeedback {
		t.Error("expected has_feedback=true")
	}
	if len(result.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(result.Entities))
	}
	if result.Entities[0].Value != "be more direct" {
		t.Errorf("value = %q, want %q", result.Entities[0].Value, "be more direct")
	}
}

func TestParseFeedbackMarkdownFenced(t *testing.T) {
	raw := "```json\n{\"has_feedback\": false, \"entities\": []}\n```\n"
	result, err := ParseFeedbackResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.HasFeedback {
		t.Error("expected false")
	}
}

func TestStripMarkdownFences(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`{"key": "value"}`, `{"key": "value"}`},
		{"```json\n{\"key\": \"value\"}\n```", `{"key": "value"}`},
		{"```\n{\"key\": \"value\"}\n```", `{"key": "value"}`},
		{"  ```json\n{\"k\": \"v\"}\n```  ", `{"k": "v"}`},
	}
	for _, tt := range tests {
		got := stripMarkdownFences(tt.input)
		if got != tt.want {
			t.Errorf("stripMarkdownFences(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
