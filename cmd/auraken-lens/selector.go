package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mistakeknot/Skaffen/pkg/lens"
)

// Config holds the runtime knobs for the lens-selection LLM call. All
// fields have sensible CLIProxyAPI-friendly defaults.
type Config struct {
	APIBase string        // e.g. http://127.0.0.1:8317/v1
	APIKey  string        // bearer; empty is allowed for unauthenticated proxies
	Model   string        // e.g. claude-opus-4-7
	Timeout time.Duration // per-request HTTP timeout
}

// systemPrompt frames the LLM as Auraken's lens selector. It commits the
// model to (a) pick AT MOST ONE lens or none, (b) emit a single JSON
// object with the soundpost shape, (c) never enumerate a menu. The
// soundpost geometry is reinforced verbally; the JSON parser then
// enforces it structurally.
const systemPrompt = `You are Auraken's lens selector.

A "lens" is a conceptual framework from a curated catalog (systems
dynamics, design thinking, complexity, cognitive science, etc.).

For a given user message + optional context, you do exactly ONE of:

  A. Pick ONE lens that applies to the user's thinking-through context,
     and respond with a single JSON object:

       {
         "empty": false,
         "lens": "<lens name from the catalog, copied verbatim>",
         "rationale": "<one or two prose sentences explaining why this
                       lens applies — spoken AT the user, never about
                       the user, never name the lens itself in the
                       rationale>",
         "next_question": "<the single next question Auraken would ask
                           through this lens — exactly one question,
                           not a list, not a menu>"
       }

  B. Respond with {"empty": true} — when the user's message is
     factual, casual, a greeting, or does not call for a lens.
     Returning empty IS a valid and frequent outcome. Do not force
     a lens when none fits.

CRITICAL constraints:
  - Output ONLY the JSON object. No prose, no markdown fences, no
    preamble, no trailing text.
  - Never return a list of lenses. Never offer the user a menu of
    options. Pick one or pick none.
  - The "lens" value must be the lens "name" (not the id) from the
    catalog, copied verbatim. If you cannot match, return empty.
  - "rationale" is 1-2 sentences, max 800 chars. "next_question" is
    exactly one question, max 400 chars.
  - Do not include any field other than empty/lens/rationale/next_question.`

// selectSoundpost performs the single chat-completions call and returns
// the parsed + validated soundpost.
func selectSoundpost(cfg Config, in Input, lenses []lens.Lens) (Soundpost, error) {
	if cfg.APIBase == "" {
		return Soundpost{}, errors.New("API base URL not configured")
	}
	if cfg.Model == "" {
		return Soundpost{}, errors.New("model not configured")
	}

	user := buildUserPrompt(in, lenses)

	req := chatRequest{
		Model: cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: user},
		},
		Temperature: 0.0,
		MaxTokens:   600,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return Soundpost{}, fmt.Errorf("encode request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(cfg.APIBase, "/")+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return Soundpost{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	client := &http.Client{Timeout: cfg.Timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return Soundpost{}, fmt.Errorf("chat-completions request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Soundpost{}, fmt.Errorf(
			"chat-completions status %d: %s",
			resp.StatusCode, strings.TrimSpace(string(preview)),
		)
	}

	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return Soundpost{}, fmt.Errorf("decode response: %w", err)
	}
	if len(cr.Choices) == 0 || cr.Choices[0].Message.Content == "" {
		return Soundpost{}, errors.New("empty model response")
	}

	return parseSoundpost(cr.Choices[0].Message.Content)
}

// buildUserPrompt assembles the user-role message: context summary, user
// text, history, and the lens catalog.
//
// The catalog is rendered as a flat "name :: definition" list. Definitions
// are truncated so the prompt stays well under any model's context budget.
// The 291-lens library serializes to ~30 KB at 100-char-truncated
// definitions — small enough for any chat-completions model in v0.1's
// compatibility matrix.
func buildUserPrompt(in Input, lenses []lens.Lens) string {
	var b strings.Builder
	b.WriteString("User message:\n")
	b.WriteString(in.Text)
	b.WriteString("\n")

	if in.ContextSummary != "" {
		b.WriteString("\nPrior context summary:\n")
		b.WriteString(in.ContextSummary)
		b.WriteString("\n")
	}
	if len(in.History) > 0 {
		b.WriteString("\nRecent conversation history:\n")
		for _, h := range in.History {
			b.WriteString("- ")
			b.WriteString(h)
			b.WriteString("\n")
		}
	}

	b.WriteString("\nLens catalog (name :: short definition):\n")
	for _, l := range lenses {
		b.WriteString("- ")
		b.WriteString(l.Name)
		b.WriteString(" :: ")
		b.WriteString(truncate(l.Definition, 140))
		b.WriteString("\n")
	}
	b.WriteString("\nRespond with the soundpost JSON object only.\n")
	return b.String()
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// chat-completions request / response shapes (OpenAI-compatible).

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}
