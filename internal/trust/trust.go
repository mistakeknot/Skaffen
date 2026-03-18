package trust

import (
	"path/filepath"
	"strings"
	"sync"
)

// Decision represents the trust evaluation result.
type Decision int

const (
	Allow Decision = iota
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
	Overrides        []Override
	PromoteThreshold int // 0 = use DefaultPromoteThreshold
}

// DefaultPromoteThreshold is the number of session-scoped Learn calls on the
// same pattern before auto-promoting to ScopeGlobal.
const DefaultPromoteThreshold = 5

// Evaluator implements the three-tier trust evaluation pipeline.
type Evaluator struct {
	mu               sync.RWMutex
	session          map[string]Decision
	sessionCount     map[string]int // per-pattern Learn call count
	overrides        []Override
	exactOverrides   map[string]Decision // O(1) lookup for patterns without glob chars
	globOverrides    []Override          // only patterns containing *, ?, [, {
	promoteThreshold int
}

// isGlobPattern returns true if the pattern contains glob metacharacters.
func isGlobPattern(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[{")
}

// NewEvaluator creates an Evaluator. Pass nil for built-in rules only.
func NewEvaluator(cfg *Config) *Evaluator {
	threshold := DefaultPromoteThreshold
	e := &Evaluator{
		session:          make(map[string]Decision),
		sessionCount:     make(map[string]int),
		exactOverrides:   make(map[string]Decision),
		promoteThreshold: threshold,
	}
	if cfg != nil {
		e.overrides = cfg.Overrides
		if cfg.PromoteThreshold > 0 {
			e.promoteThreshold = cfg.PromoteThreshold
		}
		e.rebuildPartitions()
	}
	return e
}

// rebuildPartitions splits overrides into exact-match map and glob-only slice.
// Must be called with mu held (or during init before concurrent access).
func (e *Evaluator) rebuildPartitions() {
	e.exactOverrides = make(map[string]Decision, len(e.overrides))
	e.globOverrides = e.globOverrides[:0]
	for _, o := range e.overrides {
		if isGlobPattern(o.Pattern) {
			e.globOverrides = append(e.globOverrides, o)
		} else {
			e.exactOverrides[o.Pattern] = o.Decision
		}
	}
}

// PromoteThreshold returns the current promotion threshold for external access.
func (e *Evaluator) PromoteThreshold() int {
	return e.promoteThreshold
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

	// Tier 2a: learned overrides — exact match (O(1) map lookup)
	if d, ok := e.exactOverrides[key]; ok {
		return d
	}

	// Tier 2b: learned overrides — glob match (linear scan, glob patterns only)
	for _, o := range e.globOverrides {
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
		if e.sessionCount[pattern] >= e.promoteThreshold {
			// Auto-promote: move from session to learned overrides
			o := Override{
				Pattern:  pattern,
				Decision: decision,
				Scope:    ScopeGlobal,
				Count:    e.sessionCount[pattern],
			}
			e.overrides = append(e.overrides, o)
			e.addToPartition(o)
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
			// Update partition if decision changed
			if !isGlobPattern(pattern) {
				e.exactOverrides[pattern] = decision
			}
			return
		}
	}
	o := Override{
		Pattern:  pattern,
		Decision: decision,
		Scope:    scope,
		Count:    1,
	}
	e.overrides = append(e.overrides, o)
	e.addToPartition(o)
}

// addToPartition adds an override to the appropriate partition.
// Must be called with mu held.
func (e *Evaluator) addToPartition(o Override) {
	if isGlobPattern(o.Pattern) {
		e.globOverrides = append(e.globOverrides, o)
	} else {
		e.exactOverrides[o.Pattern] = o.Decision
	}
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

// Revoke removes a learned override by pattern. Returns true if found and removed.
func (e *Evaluator) Revoke(pattern string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, o := range e.overrides {
		if o.Pattern == pattern {
			e.overrides = append(e.overrides[:i], e.overrides[i+1:]...)
			e.rebuildPartitions()
			return true
		}
	}
	// Also clear from session overrides
	if _, ok := e.session[pattern]; ok {
		delete(e.session, pattern)
		delete(e.sessionCount, pattern)
		return true
	}
	return false
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
