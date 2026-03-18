package skill

import "testing"

var sampleSkillMD = []byte(`---
name: test-skill
description: A benchmark test skill for measuring parse performance
user_invocable: true
triggers:
  - "run tests"
  - "execute benchmark"
  - "check performance"
args: "<file> [--verbose]"
model: sonnet
---

# Test Skill

This is the body of the skill. It contains instructions for the agent.

## Usage

When invoked, this skill will:
1. Parse the arguments
2. Run the appropriate benchmark
3. Report results

## Notes

This body is intentionally medium-length to represent a typical skill file.
`)

func BenchmarkParseFrontmatter(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		_, _ = parseFrontmatter(sampleSkillMD)
	}
}

func BenchmarkExtractBody(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		_ = extractBody(sampleSkillMD)
	}
}

var largeFrontmatter = []byte(`---
name: large-skill
description: A skill with many triggers to test parsing performance at scale
user_invocable: true
triggers:
  - "trigger one"
  - "trigger two"
  - "trigger three"
  - "trigger four"
  - "trigger five"
  - "trigger six"
  - "trigger seven"
  - "trigger eight"
  - "trigger nine"
  - "trigger ten"
  - "trigger eleven"
  - "trigger twelve"
  - "trigger thirteen"
  - "trigger fourteen"
  - "trigger fifteen"
  - "trigger sixteen"
  - "trigger seventeen"
  - "trigger eighteen"
  - "trigger nineteen"
  - "trigger twenty"
args: "<input> [--format json|yaml] [--output path] [--verbose] [--dry-run]"
model: opus
---

Body content here.
`)

func BenchmarkParseFrontmatterLarge(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		_, _ = parseFrontmatter(largeFrontmatter)
	}
}
