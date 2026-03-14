package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

const (
	defaultBaseURL = "https://api.anthropic.com"
	defaultModel   = "claude-sonnet-4-20250514"
	apiVersion     = "2023-06-01"
)

// Sentinel errors for callers to match.
var (
	ErrRateLimited  = errors.New("rate limited")
	ErrOverloaded   = errors.New("API overloaded")
	ErrUnauthorized = errors.New("unauthorized")
	ErrAPI          = errors.New("API error")
)

// Option configures the AnthropicProvider.
type Option func(*AnthropicProvider)

// WithBaseURL overrides the API base URL (for testing).
func WithBaseURL(url string) Option {
	return func(p *AnthropicProvider) { p.baseURL = url }
}

// WithModel sets the default model.
func WithModel(model string) Option {
	return func(p *AnthropicProvider) { p.model = model }
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(p *AnthropicProvider) { p.client = client }
}

// AnthropicProvider implements provider.Provider via the Anthropic Messages API.
type AnthropicProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// New creates an AnthropicProvider with the given API key and options.
func New(apiKey string, opts ...Option) *AnthropicProvider {
	p := &AnthropicProvider{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		model:   defaultModel,
		client:  http.DefaultClient,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name returns "anthropic".
func (p *AnthropicProvider) Name() string { return "anthropic" }

// apiRequest is the JSON body for POST /v1/messages.
type apiRequest struct {
	Model     string                `json:"model"`
	MaxTokens int                   `json:"max_tokens"`
	Stream    bool                  `json:"stream"`
	Messages  []provider.Message    `json:"messages"`
	System    string                `json:"system,omitempty"`
	Tools     []provider.ToolDef    `json:"tools,omitempty"`
	Thinking  *thinkingConfig       `json:"thinking,omitempty"`
}

// thinkingConfig enables extended thinking (reasoning) for supported models.
type thinkingConfig struct {
	Type         string `json:"type"`          // "enabled"
	BudgetTokens int    `json:"budget_tokens"` // max tokens for thinking
}

// Stream sends a streaming request to the Anthropic Messages API.
func (p *AnthropicProvider) Stream(ctx context.Context, messages []provider.Message, tools []provider.ToolDef, config provider.Config) (*provider.StreamResponse, error) {
	model := config.Model
	if model == "" {
		model = p.model
	}
	maxTokens := config.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	reqBody := apiRequest{
		Model:     model,
		MaxTokens: maxTokens,
		Stream:    true,
		Messages:  messages,
		System:    config.System,
		Tools:     tools,
	}

	// Enable extended thinking when budget is set.
	// Extended thinking requires MaxTokens to include the thinking budget.
	if config.ThinkingBudget > 0 {
		reqBody.Thinking = &thinkingConfig{
			Type:         "enabled",
			BudgetTokens: config.ThinkingBudget,
		}
		// MaxTokens must be >= ThinkingBudget + output; bump if needed.
		minTokens := config.ThinkingBudget + maxTokens
		if reqBody.MaxTokens < minTokens {
			reqBody.MaxTokens = minTokens
		}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", p.apiKey)
	req.Header.Set("Anthropic-Version", apiVersion)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, parseHTTPError(resp)
	}

	events := make(chan provider.StreamEvent, 16)
	go p.processStream(resp.Body, events)

	return provider.NewStreamResponse(events), nil
}

// processStream reads SSE events from the response body and sends StreamEvents.
func (p *AnthropicProvider) processStream(body io.ReadCloser, events chan<- provider.StreamEvent) {
	defer close(events)
	defer body.Close()

	reader := NewSSEReader(body)
	var usage provider.Usage
	var stopReason string

	for {
		sse, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			events <- provider.StreamEvent{Type: provider.EventError, Err: fmt.Errorf("read SSE: %w", err)}
			return
		}

		// Skip keepalive pings
		if sse.Event == "ping" {
			continue
		}

		// Mid-stream errors
		if sse.Event == "error" {
			events <- provider.StreamEvent{
				Type: provider.EventError,
				Err:  parseMidStreamError(sse.Data),
			}
			return
		}

		// Parse the data payload
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(sse.Data, &envelope); err != nil {
			continue // skip unparseable events
		}

		switch envelope.Type {
		case "message_start":
			var msg struct {
				Message struct {
					Usage provider.Usage `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal(sse.Data, &msg); err == nil {
				usage = msg.Message.Usage
			}

		case "content_block_start":
			var block struct {
				ContentBlock struct {
					Type  string `json:"type"`
					ID    string `json:"id"`
					Name  string `json:"name"`
				} `json:"content_block"`
			}
			if err := json.Unmarshal(sse.Data, &block); err == nil {
				switch block.ContentBlock.Type {
				case "tool_use":
					events <- provider.StreamEvent{
						Type: provider.EventToolUseStart,
						ID:   block.ContentBlock.ID,
						Name: block.ContentBlock.Name,
					}
				}
				// text blocks: no event needed at start
			}

		case "content_block_delta":
			var delta struct {
				Delta struct {
					Type        string `json:"type"`
					Text        string `json:"text"`
					PartialJSON string `json:"partial_json"`
				} `json:"delta"`
			}
			if err := json.Unmarshal(sse.Data, &delta); err == nil {
				switch delta.Delta.Type {
				case "text_delta":
					events <- provider.StreamEvent{
						Type: provider.EventTextDelta,
						Text: delta.Delta.Text,
					}
				case "input_json_delta":
					events <- provider.StreamEvent{
						Type: provider.EventToolUseDelta,
						Text: delta.Delta.PartialJSON,
					}
				}
			}

		case "message_delta":
			var md struct {
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
				Usage struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal(sse.Data, &md); err == nil {
				stopReason = md.Delta.StopReason
				// output_tokens is cumulative — overwrite, don't add
				usage.OutputTokens = md.Usage.OutputTokens
			}

		case "message_stop":
			events <- provider.StreamEvent{
				Type:       provider.EventDone,
				Usage:      &usage,
				StopReason: stopReason,
			}
		}
	}
}

// parseHTTPError converts a non-200 HTTP response to a sentinel error.
func parseHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	var apiErr struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(body, &apiErr)

	msg := apiErr.Error.Message
	if msg == "" {
		msg = string(body)
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s (retry-after: %s)", ErrRateLimited, msg, resp.Header.Get("Retry-After"))
	case 529:
		return fmt.Errorf("%w: %s", ErrOverloaded, msg)
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: %s", ErrUnauthorized, msg)
	default:
		return fmt.Errorf("%w: HTTP %d: %s", ErrAPI, resp.StatusCode, msg)
	}
}

// parseMidStreamError parses an SSE error event.
func parseMidStreamError(data []byte) error {
	var errEvt struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &errEvt); err == nil && errEvt.Error.Message != "" {
		return fmt.Errorf("%w: %s: %s", ErrAPI, errEvt.Error.Type, errEvt.Error.Message)
	}
	return fmt.Errorf("%w: unknown stream error", ErrAPI)
}
