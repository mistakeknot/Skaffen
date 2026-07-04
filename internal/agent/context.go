package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/mistakeknot/Masaq/priompt"
	"github.com/mistakeknot/Skaffen/internal/provider"
)

// TurnContext carries per-turn conversational state to context providers.
// It is the Go analogue of the arguments Auraken's build_system_prompt
// receives per message: providers combine it with their own state sources
// (profile DB, fingerprint store) to produce prompt sections.
type TurnContext struct {
	// Message is the current user message being processed.
	Message string
	// RecentTurns is the visible conversation history, oldest first.
	RecentTurns []ContextTurn
	// InteractionCount is the number of prior user interactions.
	InteractionCount int
	// IsNewUser reports whether this user has no prior history.
	IsNewUser bool
}

// ContextTurn is one prior exchange visible to context providers.
type ContextTurn struct {
	Role    string // "user" or "assistant"
	Content string
}

// ContextProvider produces one named section of the system prompt.
// Name() doubles as the template slot: the provider's output replaces
// the "{<name>_context}" marker in the pipeline template.
//
// Providers are called pre-turn and may be skipped entirely when their
// CacheKey matches the previous turn's key (PRD F5: invalidate on cache
// key change only).
type ContextProvider interface {
	// Name identifies the provider and its template slot.
	Name() string
	// Provide returns the section content for this turn.
	Provide(ctx context.Context, turn TurnContext) (string, error)
	// CacheKey returns a stable key for the inputs that determine this
	// provider's output. Equal keys across turns reuse the cached section.
	CacheKey(turn TurnContext) string
}

// SectionTruncator is an optional ContextProvider interface for
// structure-aware token budget enforcement. Providers that know their
// section's internal structure (e.g. a turn list) can drop whole units;
// others get the pipeline's default oldest-first truncation.
type SectionTruncator interface {
	TruncateToBudget(output string, maxTokens int) string
}

// contextTokenizer is the token estimator shared with the agent loop
// (chars/4 heuristic, consistent with priompt rendering).
var contextTokenizer = priompt.CharHeuristic{Ratio: 4}

// ContextPipeline assembles a system prompt from an ordered list of
// context providers (PRD F5, Auraken→Skaffen migration).
//
// Per turn, Assemble:
//  1. checks each provider's CacheKey — unchanged keys reuse the cached
//     section without calling Provide,
//  2. runs all cache-miss providers concurrently (async pre-turn hooks),
//  3. enforces each provider's token budget with an oldest-first
//     overflow strategy,
//  4. composes sections into the template by named slot — placement is
//     semantic (the template decides where each section lands), not
//     concatenation order.
type ContextPipeline struct {
	template  string
	providers []ContextProvider
	budgets   map[string]int // provider name -> max tokens (0 = unlimited)

	mu    sync.Mutex
	cache map[string]contextCacheEntry
}

type contextCacheEntry struct {
	key   string
	value string
}

// NewContextPipeline creates a pipeline that fills the template's
// "{<name>_context}" slots from the given providers, in order.
func NewContextPipeline(template string, providers ...ContextProvider) *ContextPipeline {
	return &ContextPipeline{
		template:  template,
		providers: providers,
		budgets:   make(map[string]int),
		cache:     make(map[string]contextCacheEntry),
	}
}

// SetBudget sets the maximum token budget for a provider's section.
// A budget of 0 (the default) means unlimited.
func (p *ContextPipeline) SetBudget(providerName string, maxTokens int) {
	p.budgets[providerName] = maxTokens
}

// Providers returns the registered providers in call order.
func (p *ContextPipeline) Providers() []ContextProvider {
	out := make([]ContextProvider, len(p.providers))
	copy(out, p.providers)
	return out
}

// InvalidateAll drops all cached sections, forcing every provider to run
// on the next Assemble.
func (p *ContextPipeline) InvalidateAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache = make(map[string]contextCacheEntry)
}

// Assemble runs the providers for this turn and composes the system
// prompt. Provider errors abort assembly: a persona prompt missing a
// section is a correctness failure, not a degraded mode.
func (p *ContextPipeline) Assemble(ctx context.Context, turn TurnContext) (string, error) {
	type result struct {
		out string
		err error
	}
	results := make([]result, len(p.providers))

	var wg sync.WaitGroup
	for i, prov := range p.providers {
		key := prov.CacheKey(turn)

		p.mu.Lock()
		entry, ok := p.cache[prov.Name()]
		p.mu.Unlock()
		if ok && entry.key == key {
			results[i] = result{out: entry.value}
			continue
		}

		wg.Add(1)
		go func(i int, prov ContextProvider, key string) {
			defer wg.Done()
			out, err := prov.Provide(ctx, turn)
			if err != nil {
				results[i] = result{err: fmt.Errorf("context provider %q: %w", prov.Name(), err)}
				return
			}
			out = p.enforceBudget(prov, out)
			p.mu.Lock()
			p.cache[prov.Name()] = contextCacheEntry{key: key, value: out}
			p.mu.Unlock()
			results[i] = result{out: out}
		}(i, prov, key)
	}
	wg.Wait()

	pairs := make([]string, 0, len(p.providers)*2)
	for i, prov := range p.providers {
		if results[i].err != nil {
			return "", results[i].err
		}
		pairs = append(pairs, "{"+prov.Name()+"_context}", results[i].out)
	}
	return strings.NewReplacer(pairs...).Replace(p.template), nil
}

// enforceBudget bounds a section to its token budget. Providers that
// implement SectionTruncator control their own truncation; the default
// strategy preserves the section header (first line) and drops the
// oldest content first, keeping the newest tail (PRD F5 overflow rule).
func (p *ContextPipeline) enforceBudget(prov ContextProvider, out string) string {
	budget := p.budgets[prov.Name()]
	if budget <= 0 || contextTokenizer.Count(out) <= budget {
		return out
	}
	if tr, ok := prov.(SectionTruncator); ok {
		return tr.TruncateToBudget(out, budget)
	}
	return truncateOldestFirst(out, budget)
}

// truncateOldestFirst keeps a section's header line and the newest tail
// of its content within maxTokens, marking the elision.
func truncateOldestFirst(s string, maxTokens int) string {
	const marker = "[earlier context truncated]\n"

	// Preserve the header: everything through the first newline that
	// follows a non-empty line (sections typically start "\n## Title\n").
	header := ""
	rest := s
	if idx := headerEnd(s); idx > 0 {
		header = s[:idx]
		rest = s[idx:]
	}

	allowedChars := maxTokens*contextTokenizer.Ratio - len(header) - len(marker)
	if allowedChars <= 0 {
		// Budget cannot even hold the header plus marker; hard-cut the tail.
		allowed := maxTokens * contextTokenizer.Ratio
		if allowed >= len(s) {
			return s
		}
		return s[len(s)-allowed:]
	}
	if len(rest) <= allowedChars {
		return s
	}

	tail := rest[len(rest)-allowedChars:]
	// Snap to the next line boundary so we don't emit a torn line.
	if idx := strings.IndexByte(tail, '\n'); idx >= 0 && idx+1 < len(tail) {
		tail = tail[idx+1:]
	}
	return header + marker + tail
}

// buildTurnContext derives a TurnContext from the incoming content blocks
// and the session's message history. Persona-specific state (profile,
// entities, fingerprint) is not represented here — providers fetch that
// from their own state sources.
func buildTurnContext(content []provider.ContentBlock, history []provider.Message) TurnContext {
	turn := TurnContext{}

	for _, b := range content {
		if b.Type == "text" && b.Text != "" {
			turn.Message = b.Text // last text block wins (images precede text)
		}
	}

	userCount := 0
	for _, m := range history {
		var texts []string
		for _, b := range m.Content {
			if b.Type == "text" && b.Text != "" {
				texts = append(texts, b.Text)
			}
		}
		if len(texts) == 0 {
			continue
		}
		role := "assistant"
		if m.Role == provider.RoleUser {
			role = "user"
			userCount++
		}
		turn.RecentTurns = append(turn.RecentTurns, ContextTurn{Role: role, Content: strings.Join(texts, "\n")})
	}
	turn.InteractionCount = userCount
	turn.IsNewUser = len(history) == 0

	return turn
}

// headerEnd returns the index just past the section's first non-empty
// line, or 0 if there is none.
func headerEnd(s string) int {
	i := 0
	for i < len(s) && s[i] == '\n' {
		i++
	}
	if i >= len(s) {
		return 0
	}
	if idx := strings.IndexByte(s[i:], '\n'); idx >= 0 {
		return i + idx + 1
	}
	return 0
}
