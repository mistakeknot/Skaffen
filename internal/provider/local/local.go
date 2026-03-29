// Package local implements a provider that calls the interfere local inference
// server via its OpenAI-compatible /v1/chat/completions SSE endpoint.
package local

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

var (
	ErrCloudFallback = errors.New("interfere: cascade routed to cloud")
	ErrOverloaded    = errors.New("interfere: server overloaded")
	ErrUnavailable   = errors.New("interfere: server unavailable")
)

// Option configures the LocalProvider.
type Option func(*LocalProvider)

func WithBaseURL(url string) Option {
	return func(p *LocalProvider) { p.baseURL = url }
}

func WithModel(model string) Option {
	return func(p *LocalProvider) { p.model = model }
}

func WithHTTPClient(client *http.Client) Option {
	return func(p *LocalProvider) { p.client = client }
}

// LocalProvider implements provider.Provider via interfere's OpenAI-compatible API.
type LocalProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

func New(opts ...Option) *LocalProvider {
	p := &LocalProvider{
		baseURL: "http://localhost:8421",
		model:   "default",
		client:  http.DefaultClient,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *LocalProvider) Name() string { return "local" }

// openAIMessage is the message format for OpenAI-compatible APIs.
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIRequest is the request body for /v1/chat/completions.
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Stream      bool            `json:"stream"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature,omitempty"`
}

// Stream sends a streaming request to interfere's /v1/chat/completions endpoint.
func (p *LocalProvider) Stream(ctx context.Context, messages []provider.Message, tools []provider.ToolDef, config provider.Config) (*provider.StreamResponse, error) {
	model := config.Model
	if model == "" {
		model = p.model
	}
	maxTokens := config.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Convert Skaffen messages to OpenAI format.
	// interfere only supports text content, not tool_use/tool_result.
	oaiMessages := convertMessages(messages, config.System)

	temp := config.Temperature
	if temp < 0 {
		temp = 0.7 // interfere default
	}

	reqBody := openAIRequest{
		Model:       model,
		Messages:    oaiMessages,
		Stream:      true,
		MaxTokens:   maxTokens,
		Temperature: temp,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	if resp.StatusCode == http.StatusServiceUnavailable {
		defer resp.Body.Close()
		return nil, parseOverloadedError(resp)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, parseHTTPError(resp)
	}

	// Check for cascade cloud fallback (non-streaming JSON response).
	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		defer resp.Body.Close()
		return nil, parseCascadeFallback(resp.Body)
	}

	events := make(chan provider.StreamEvent, 16)
	go processOpenAIStream(resp.Body, events, model)

	return provider.NewStreamResponse(events), nil
}

// convertMessages transforms Skaffen messages to OpenAI format.
// Prepends system message if provided. Concatenates content blocks into text.
func convertMessages(messages []provider.Message, system string) []openAIMessage {
	var result []openAIMessage

	if system != "" {
		result = append(result, openAIMessage{Role: "system", Content: system})
	}

	for _, msg := range messages {
		var text strings.Builder
		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				text.WriteString(block.Text)
			case "tool_result":
				// Include tool results as text for local models
				text.WriteString(fmt.Sprintf("[Tool result for %s]: %s", block.ToolUseID, block.ResultContent))
			case "tool_use":
				// Include tool calls as text for local models
				text.WriteString(fmt.Sprintf("[Tool call: %s(%s)]", block.Name, string(block.Input)))
			}
		}
		if text.Len() > 0 {
			result = append(result, openAIMessage{
				Role:    string(msg.Role),
				Content: text.String(),
			})
		}
	}

	return result
}

// processOpenAIStream reads OpenAI SSE chunks and emits Skaffen StreamEvents.
func processOpenAIStream(body io.ReadCloser, events chan<- provider.StreamEvent, model string) {
	defer close(events)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	// Allow large lines (some chunks can be big)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	totalTokens := 0

	for scanner.Scan() {
		line := scanner.Text()

		// SSE format: "data: {...}" or "data: [DONE]"
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		if data == "[DONE]" {
			events <- provider.StreamEvent{
				Type: provider.EventDone,
				Usage: &provider.Usage{
					OutputTokens: totalTokens,
				},
				StopReason: "end_turn",
			}
			return
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]

			if choice.Delta.Content != "" {
				totalTokens++
				events <- provider.StreamEvent{
					Type: provider.EventTextDelta,
					Text: choice.Delta.Content,
				}
			}

			if choice.FinishReason != nil && *choice.FinishReason == "stop" {
				usage := provider.Usage{OutputTokens: totalTokens}
				if chunk.Usage != nil {
					usage.InputTokens = chunk.Usage.PromptTokens
					usage.OutputTokens = chunk.Usage.CompletionTokens
				}
				events <- provider.StreamEvent{
					Type:       provider.EventDone,
					Usage:      &usage,
					StopReason: "end_turn",
				}
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		events <- provider.StreamEvent{
			Type: provider.EventError,
			Err:  fmt.Errorf("read stream: %w", err),
		}
		return
	}

	// Stream ended without [DONE] or finish_reason — emit Done anyway
	events <- provider.StreamEvent{
		Type: provider.EventDone,
		Usage: &provider.Usage{
			OutputTokens: totalTokens,
		},
		StopReason: "end_turn",
	}
}

// CascadeError carries structured metadata from interfere's cascade routing.
// Unwrap returns ErrCloudFallback so errors.Is() works.
type CascadeError struct {
	Decision    string   `json:"decision"`     // "cloud"
	Confidence  float64  `json:"confidence"`   // avg confidence from probe
	ModelsTried []string `json:"models_tried"` // models probed before fallback
}

func (e *CascadeError) Error() string {
	return fmt.Sprintf("interfere: cascade routed to cloud: confidence=%.3f, tried=%v", e.Confidence, e.ModelsTried)
}

func (e *CascadeError) Unwrap() error { return ErrCloudFallback }

// parseCascadeFallback reads a JSON response indicating cloud fallback.
func parseCascadeFallback(body io.Reader) error {
	data, _ := io.ReadAll(io.LimitReader(body, 4096))
	var resp struct {
		Cascade     string   `json:"cascade"`
		Confidence  float64  `json:"confidence"`
		Message     string   `json:"message"`
		ModelsTried []string `json:"models_tried"`
	}
	if err := json.Unmarshal(data, &resp); err == nil && resp.Cascade == "cloud_fallback" {
		return &CascadeError{
			Decision:    "cloud",
			Confidence:  resp.Confidence,
			ModelsTried: resp.ModelsTried,
		}
	}
	return fmt.Errorf("%w: unexpected JSON response: %s", ErrUnavailable, string(data))
}

func parseOverloadedError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var apiErr struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &apiErr); err == nil && apiErr.Error.Message != "" {
		return fmt.Errorf("%w: %s", ErrOverloaded, apiErr.Error.Message)
	}
	return fmt.Errorf("%w: HTTP 503", ErrOverloaded)
}

func parseHTTPError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("%w: HTTP %d: %s", ErrUnavailable, resp.StatusCode, string(data))
}
