package extraction

import (
	"encoding/json"
	"strings"
)

// ParseExtractionResponse parses the LLM extraction response, handling
// markdown code fences that LLMs commonly wrap JSON in.
func ParseExtractionResponse(raw string) (ExtractionResult, error) {
	cleaned := stripMarkdownFences(raw)
	var result ExtractionResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return ExtractionResult{HasPreferenceSignal: false}, err
	}
	return result, nil
}

// ParseFeedbackResponse parses the feedback extraction LLM response.
func ParseFeedbackResponse(raw string) (FeedbackResult, error) {
	cleaned := stripMarkdownFences(raw)
	var result FeedbackResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return FeedbackResult{HasFeedback: false}, err
	}
	return result, nil
}

// stripMarkdownFences removes ```json ... ``` wrapping if present.
func stripMarkdownFences(raw string) string {
	cleaned := strings.TrimSpace(raw)
	if !strings.HasPrefix(cleaned, "```") {
		return cleaned
	}
	lines := strings.Split(cleaned, "\n")
	// Skip first line (```json or ```)
	var filtered []string
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "```" {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}
