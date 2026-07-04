// Package auraken implements the Auraken persona's context providers for
// the Skaffen agent (PRD F5, Auraken→Skaffen migration, sylveste-benl.3).
//
// Each provider populates one section of the OODARC system prompt by
// calling the corresponding pkg/ library (pkg/prompts section builders,
// pkg/style mirroring, pkg/lens selection). Providers plug into
// agent.ContextPipeline, which handles async pre-turn execution, caching
// (invalidated on cache-key change only), per-section token budgets, and
// template-slot composition.
//
// Cache keys follow the PRD F5 spec:
//   - lens:        hash(message + last 3 turns)
//   - style:       hash(message text)
//   - profile:     entity count + latest entity timestamp
//
// Profile data (working profile, entities, fingerprint, feedback) comes
// from a State implementation. The DB-backed State arrives with the
// shared identity/profile database (F11, sylveste-benl.10); until then
// callers supply in-memory state.
package auraken

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/pkg/prompts"
	"github.com/mistakeknot/Skaffen/pkg/style"
)

// State supplies profile-derived data for the Auraken context providers.
//
// EntityStats must be cheap and infallible: it feeds cache keys, which are
// consulted synchronously every turn. Implementations backed by a database
// should maintain the stats in memory and refresh them on writes.
type State interface {
	// WorkingProfile returns the LLM-generated profile narrative, or ""
	// for a user with no profile yet.
	WorkingProfile(ctx context.Context) (string, error)
	// ExploredDomains reports which preference domains have been explored.
	// A nil map means exploration tracking has not started.
	ExploredDomains(ctx context.Context) (map[string]bool, error)
	// KnownEntities returns the user's preference entities.
	KnownEntities(ctx context.Context) ([]prompts.Entity, error)
	// FeedbackEntities returns explicit behavioral feedback entities.
	FeedbackEntities(ctx context.Context) ([]prompts.Feedback, error)
	// Fingerprint returns the user's style fingerprint, or nil if none.
	Fingerprint(ctx context.Context) (*style.Fingerprint, error)
	// EntityStats returns the entity count and latest entity timestamp,
	// used as the profile cache key (PRD F5).
	EntityStats() (count int, latest time.Time)
}

// LensSource selects relevant lenses for a message. The production
// implementation wraps pkg/lens.Selector (wired in with the persona
// config, sylveste-benl.8); tests use fakes.
type LensSource interface {
	RelevantLenses(ctx context.Context, message string, history []string) ([]prompts.Lens, error)
}

// toPromptTurns converts pipeline turns to prompts turns.
func toPromptTurns(turns []agent.ContextTurn) []prompts.Turn {
	if len(turns) == 0 {
		return nil
	}
	out := make([]prompts.Turn, len(turns))
	for i, t := range turns {
		out[i] = prompts.Turn{Role: t.Role, Content: t.Content}
	}
	return out
}

// entityStatsKey renders EntityStats as a cache key fragment.
func entityStatsKey(st State) string {
	count, latest := st.EntityStats()
	return fmt.Sprintf("%d|%s", count, latest.UTC().Format(time.RFC3339Nano))
}

// --- Feedback ---

// FeedbackProvider renders explicit user feedback as behavioral
// instructions ({feedback_context}).
type FeedbackProvider struct {
	State State
}

func (p *FeedbackProvider) Name() string { return "feedback" }

func (p *FeedbackProvider) Provide(ctx context.Context, _ agent.TurnContext) (string, error) {
	fbs, err := p.State.FeedbackEntities(ctx)
	if err != nil {
		return "", err
	}
	return prompts.BuildFeedbackContext(fbs), nil
}

// CacheKey tracks entity stats: feedback entities are preference entities,
// so any feedback change moves the count/latest-timestamp pair.
func (p *FeedbackProvider) CacheKey(_ agent.TurnContext) string {
	return "fb|" + entityStatsKey(p.State)
}

// --- Lens ---

// LensProvider selects relevant lenses for the current message and renders
// them as framework suggestions ({lens_context}).
type LensProvider struct {
	Source LensSource
}

func (p *LensProvider) Name() string { return "lens" }

func (p *LensProvider) Provide(ctx context.Context, turn agent.TurnContext) (string, error) {
	history := make([]string, 0, len(turn.RecentTurns))
	for _, t := range turn.RecentTurns {
		history = append(history, t.Content)
	}
	lenses, err := p.Source.RelevantLenses(ctx, turn.Message, history)
	if err != nil {
		return "", err
	}
	return prompts.BuildLensContext(lenses), nil
}

// CacheKey is hash(message + last 3 turns), per PRD F5.
func (p *LensProvider) CacheKey(turn agent.TurnContext) string {
	h := sha256.New()
	h.Write([]byte(turn.Message))
	turns := turn.RecentTurns
	if len(turns) > 3 {
		turns = turns[len(turns)-3:]
	}
	for _, t := range turns {
		h.Write([]byte{0})
		h.Write([]byte(t.Role))
		h.Write([]byte{0})
		h.Write([]byte(t.Content))
	}
	return "lens|" + hex.EncodeToString(h.Sum(nil))
}

// --- Profile ---

// ProfileProvider renders the working profile narrative, or the persona's
// new-user intro when no profile exists ({profile_context}).
type ProfileProvider struct {
	State  State
	Config prompts.PromptConfig
}

func (p *ProfileProvider) Name() string { return "profile" }

func (p *ProfileProvider) Provide(ctx context.Context, _ agent.TurnContext) (string, error) {
	wp, err := p.State.WorkingProfile(ctx)
	if err != nil {
		return "", err
	}
	return prompts.BuildProfileContext(p.Config, wp), nil
}

// CacheKey is entity count + latest entity timestamp, per PRD F5.
func (p *ProfileProvider) CacheKey(_ agent.TurnContext) string {
	return "profile|" + entityStatsKey(p.State)
}

// --- Steering ---

// SteeringProvider renders conversation guidance: gap detection, profile
// echo, follow-up/reflect checks, pattern hunting ({steering_context}).
type SteeringProvider struct {
	State State
}

func (p *SteeringProvider) Name() string { return "steering" }

func (p *SteeringProvider) Provide(ctx context.Context, turn agent.TurnContext) (string, error) {
	explored, err := p.State.ExploredDomains(ctx)
	if err != nil {
		return "", err
	}
	entities, err := p.State.KnownEntities(ctx)
	if err != nil {
		return "", err
	}
	return prompts.BuildSteeringContext(explored, entities, turn.InteractionCount), nil
}

// CacheKey combines the interaction count (cadence triggers) with entity
// stats (echo citations, gap detection follow entity extraction).
func (p *SteeringProvider) CacheKey(turn agent.TurnContext) string {
	return fmt.Sprintf("steer|%d|%s", turn.InteractionCount, entityStatsKey(p.State))
}

// --- Style ---

// StyleProvider renders register-matching instructions from the style
// fingerprint, falling back to instant mirroring ({style_context}).
type StyleProvider struct {
	State State
}

func (p *StyleProvider) Name() string { return "style" }

func (p *StyleProvider) Provide(ctx context.Context, turn agent.TurnContext) (string, error) {
	fp, err := p.State.Fingerprint(ctx)
	if err != nil {
		return "", err
	}
	return prompts.BuildStyleContext(fp, toPromptTurns(turn.RecentTurns), turn.Message), nil
}

// CacheKey is hash(message text), per PRD F5.
func (p *StyleProvider) CacheKey(turn agent.TurnContext) string {
	sum := sha256.Sum256([]byte(turn.Message))
	return "style|" + hex.EncodeToString(sum[:])
}

// --- Session ---

// SessionProvider renders the recent conversation ({session_context}).
type SessionProvider struct{}

func (p *SessionProvider) Name() string { return "session" }

func (p *SessionProvider) Provide(_ context.Context, turn agent.TurnContext) (string, error) {
	return prompts.BuildSessionContext(toPromptTurns(turn.RecentTurns)), nil
}

func (p *SessionProvider) CacheKey(turn agent.TurnContext) string {
	h := sha256.New()
	for _, t := range turn.RecentTurns {
		h.Write([]byte{0})
		h.Write([]byte(t.Role))
		h.Write([]byte{0})
		h.Write([]byte(t.Content))
	}
	return "session|" + hex.EncodeToString(h.Sum(nil))
}

// --- Bootstrap ---

// BootstrapProvider renders the graduated onboarding protocol for early
// conversations ({bootstrap_context}). Empty after the third exchange.
type BootstrapProvider struct{}

func (p *BootstrapProvider) Name() string { return "bootstrap" }

func (p *BootstrapProvider) Provide(_ context.Context, turn agent.TurnContext) (string, error) {
	return prompts.BuildBootstrapContext(turn.IsNewUser, turn.InteractionCount), nil
}

func (p *BootstrapProvider) CacheKey(turn agent.TurnContext) string {
	return fmt.Sprintf("bootstrap|%t|%d", turn.IsNewUser, turn.InteractionCount)
}

// --- Pipeline assembly ---

// DefaultBudgets are the per-section token budgets (chars/4 heuristic,
// consistent with the agent loop's estimator). Generous enough that
// normal sections pass through untouched; oversized sections are
// truncated oldest-first by the pipeline.
var DefaultBudgets = map[string]int{
	"feedback":  600,
	"lens":      2000,
	"profile":   1200,
	"steering":  800,
	"style":     600,
	"session":   2400,
	"bootstrap": 600,
}

// NewPipeline builds the Auraken persona context pipeline: six context
// providers plus the bootstrap protocol, composed into the persona's
// system template. The bootstrap section is appended after the template
// body, matching Auraken's build_system_prompt.
func NewPipeline(cfg prompts.PromptConfig, st State, lenses LensSource) *agent.ContextPipeline {
	template := cfg.SystemTemplate + "{bootstrap_context}"
	p := agent.NewContextPipeline(template,
		&FeedbackProvider{State: st},
		&LensProvider{Source: lenses},
		&ProfileProvider{State: st, Config: cfg},
		&SteeringProvider{State: st},
		&StyleProvider{State: st},
		&SessionProvider{},
		&BootstrapProvider{},
	)
	for name, budget := range DefaultBudgets {
		p.SetBudget(name, budget)
	}
	return p
}
