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

// Evaluator implements the three-tier trust evaluation pipeline.
type Evaluator struct {
	mu        sync.RWMutex
	session   map[string]Decision
	overrides []Override
}

// NewEvaluator creates an Evaluator. Pass nil for built-in rules only.
func NewEvaluator(cfg *Config) *Evaluator {
	e := &Evaluator{
		session: make(map[string]Decision),
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

// Learn adds a trust override.
func (e *Evaluator) Learn(pattern string, decision Decision, scope Scope) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if scope == ScopeSession {
		e.session[pattern] = decision
		return
	}
	e.overrides = append(e.overrides, Override{
		Pattern:  pattern,
		Decision: decision,
		Scope:    scope,
	})
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
