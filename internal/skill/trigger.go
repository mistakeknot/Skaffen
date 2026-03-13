package skill

import "strings"

// MatchTriggers checks a user message against all skill trigger phrases.
// Returns skills whose triggers match (case-insensitive substring).
// Only matches skills where UserInvocable is true.
// Complexity: O(skills × triggers) — acceptable for <100 skills.
func MatchTriggers(skills map[string]Def, message string) []Def {
	lower := strings.ToLower(message)
	var matched []Def
	for _, d := range skills {
		if !d.UserInvocable {
			continue
		}
		for _, trigger := range d.Triggers {
			if strings.Contains(lower, strings.ToLower(trigger)) {
				matched = append(matched, d)
				break // one trigger match per skill is enough
			}
		}
	}
	return matched
}
