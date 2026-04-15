// Package extraction provides the preference extraction pipeline — detects
// user context signals from conversation, extracts structured entities via LLM,
// and diffs against existing profile data.
//
// Port of Auraken's extraction.py. DB operations are behind interfaces
// (EntityStore, EpisodeStore) for implementation by the shared identity package.
package extraction
