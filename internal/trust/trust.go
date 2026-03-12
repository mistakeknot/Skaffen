package trust

import (
	"path/filepath"
	"sync"
)

// Decision represents the trust evaluation result.
type Decision int

const (
	Allow  Decision = iota
	Prompt
	Block
)

func (d Decision) String() string {
	switch d {
	case Allow:
		return "allow"
	case Prompt:
		return "prompt"
	case Block:
		return "block"
	default:
		return "unknown"
	}
}

// Scope determines the lifetime of a learned override.
type Scope int

const (
	ScopeSession Scope = iota
	ScopeProject
	ScopeGlobal
)

// Override is a learned trust rule.
type Override struct {
	Pattern  string
	Decision Decision
	Scope    Scope
	Count    int
}

// Config holds trust configuration (loaded from trust.toml).
type Config struct {
	Overrides []Override
}

// PromoteThreshold is the number of session-scoped Learn calls on the same
// pattern before auto-promoting to ScopeGlobal.
const PromoteThreshold = 10

// Evaluator implements the three-tier trust evaluation pipeline.
type Evaluator struct {
	mu           sync.RWMutex
	session      map[string]Decision
	sessionCount map[string]int // per-pattern Learn call count
	overrides    []Override
}

// NewEvaluator creates an Evaluator. Pass nil for built-in rules only.
func NewEvaluator(cfg *Config) *Evaluator {
	e := &Evaluator{
		session:      make(map[string]Decision),
		sessionCount: make(map[string]int),
	}
	if cfg != nil {
		e.overrides = cfg.Overrides
	}
	return e
}

// Evaluate runs the three-tier pipeline: session → learned → built-in.
func (e *Evaluator) Evaluate(toolName, paramsJSON string) Decision {
	key := buildKey(toolName, paramsJSON)

	e.mu.RLock()
	defer e.mu.RUnlock()

	// Tier 1: session overrides (exact match)
	if d, ok := e.session[key]; ok {
		return d
	}

	// Tier 2: learned overrides (glob match)
	for _, o := range e.overrides {
		if matchGlob(o.Pattern, key) {
			return o.Decision
		}
	}

	// Tier 3: built-in rules
	return evaluateBuiltIn(toolName, paramsJSON)
}

// Learn adds a trust override. For session-scoped overrides, it increments a
// confidence counter and auto-promotes to ScopeGlobal once PromoteThreshold
// is reached.
func (e *Evaluator) Learn(pattern string, decision Decision, scope Scope) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if scope == ScopeSession {
		e.session[pattern] = decision
		e.sessionCount[pattern]++
		if e.sessionCount[pattern] >= PromoteThreshold {
			// Auto-promote: move from session to learned overrides
			e.overrides = append(e.overrides, Override{
				Pattern:  pattern,
				Decision: decision,
				Scope:    ScopeGlobal,
				Count:    e.sessionCount[pattern],
			})
			delete(e.session, pattern)
			delete(e.sessionCount, pattern)
		}
		return
	}

	// Non-session overrides: check for existing pattern and increment count
	for i := range e.overrides {
		if e.overrides[i].Pattern == pattern {
			e.overrides[i].Count++
			e.overrides[i].Decision = decision
			return
		}
	}
	e.overrides = append(e.overrides, Override{
		Pattern:  pattern,
		Decision: decision,
		Scope:    scope,
		Count:    1,
	})
}

// SessionCount returns the confidence count for a session-scoped pattern.
func (e *Evaluator) SessionCount(pattern string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.sessionCount[pattern]
}

// Overrides returns a snapshot of the current learned overrides.
func (e *Evaluator) Overrides() []Override {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Override, len(e.overrides))
	copy(out, e.overrides)
	return out
}

func buildKey(toolName, paramsJSON string) string {
	// For bash, extract command for the key
	if toolName == "bash" {
		cmd := extractBashCommand(paramsJSON)
		if cmd != "" {
			return "bash:" + cmd
		}
	}
	return toolName
}

func matchGlob(pattern, key string) bool {
	if pattern == key {
		return true
	}
	matched, _ := filepath.Match(pattern, key)
	return matched
}
