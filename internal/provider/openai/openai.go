// Package openai implements a provider for OpenAI-compatible APIs (GLM, Qwen, DeepSeek).
// Unlike the local provider (which converts tool_use to text), this one handles native
// function calling via the tool_calls streaming delta protocol.
package openai

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
	ErrRateLimited = errors.New("openai: rate limited")
	ErrOverloaded  = errors.New("openai: server overloaded")
	ErrAPI         = errors.New("openai: API error")
)

type Option func(*Provider)

func WithBaseURL(url string) Option  { return func(p *Provider) { p.baseURL = url } }
func WithModel(model string) Option  { return func(p *Provider) { p.model = model } }
func WithHTTPClient(c *http.Client) Option { return func(p *Provider) { p.client = c } }
func WithName(name string) Option    { return func(p *Provider) { p.name = name } }

// Provider implements provider.Provider for OpenAI-compatible APIs.
type Provider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
	name    string // provider name for registration (e.g. "openai", "glm", "qwen")
}

func New(apiKey string, opts ...Option) *Provider {
	p := &Provider{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		model:   "gpt-4o",
		client:  http.DefaultClient,
		name:    "openai",
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Provider) Name() string { return p.name }

// Stream sends a streaming chat completion request with tool support.
func (p *Provider) Stream(ctx context.Context, messages []provider.Message, tools []provider.ToolDef, config provider.Config) (*provider.StreamResponse, error) {
	model := config.Model
	if model == "" {
		model = p.model
	}
	maxTokens := config.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Content filter: scan for credentials before sending to external API.
	if err := FilterProviderMessages(messages); err != nil {
		return nil, err
	}

	// Translate Anthropic-canonical messages to OpenAI wire format.
	oaiMessages := translateMessages(messages, config.System)

	// Convert tools to OpenAI function format.
	var oaiTools []oaiTool
	for _, t := range tools {
		oaiTools = append(oaiTools, oaiTool{
			Type: "function",
			Function: oaiFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	temp := config.Temperature
	if temp < 0 {
		temp = 0.7
	}

	reqBody := oaiRequest{
		Model:       model,
		Messages:    oaiMessages,
		Stream:      true,
		MaxTokens:   maxTokens,
		Temperature: temp,
	}
	if len(oaiTools) > 0 {
		reqBody.Tools = oaiTools
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, parseHTTPError(resp)
	}

	events := make(chan provider.StreamEvent, 16)
	go processStream(resp.Body, events)

	return provider.NewStreamResponse(events), nil
}

// --- Wire format types ---

type oaiRequest struct {
	Model       string       `json:"model"`
	Messages    []oaiMessage `json:"messages"`
	Stream      bool         `json:"stream"`
	MaxTokens   int          `json:"max_tokens"`
	Temperature float64      `json:"temperature,omitempty"`
	Tools       []oaiTool    `json:"tools,omitempty"`
}

type oaiMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content,omitempty"`
	ToolCalls  []oaiToolCall   `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
}

type oaiToolCall struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Function oaiCallFunc `json:"function"`
}

type oaiCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type oaiTool struct {
	Type     string      `json:"type"`
	Function oaiFunction `json:"function"`
}

type oaiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// --- Streaming ---

// partialToolCall tracks a tool call being assembled from chunked deltas.
type partialToolCall struct {
	ID   string
	Name string
	Args strings.Builder
}

// processStream reads OpenAI SSE chunks, maintains an indexMap for tool calls,
// and emits Skaffen-compatible StreamEvents.
func processStream(body io.ReadCloser, events chan<- provider.StreamEvent) {
	defer close(events)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	indexMap := map[int]*partialToolCall{}
	var inputTokens, outputTokens int

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		if data == "[DONE]" {
			// Flush any pending tool calls before Done.
			flushToolCalls(indexMap, events)
			events <- provider.StreamEvent{
				Type: provider.EventDone,
				Usage: &provider.Usage{
					InputTokens:  inputTokens,
					OutputTokens: outputTokens,
				},
				StopReason: "end_turn",
			}
			return
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Capture usage if present (some providers send it in the last chunk).
		if chunk.Usage != nil {
			inputTokens = chunk.Usage.PromptTokens
			outputTokens = chunk.Usage.CompletionTokens
		}

		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]

		// Text content
		if choice.Delta.Content != "" {
			events <- provider.StreamEvent{
				Type: provider.EventTextDelta,
				Text: choice.Delta.Content,
			}
		}

		// Tool call deltas — index-keyed reassembly
		for _, tc := range choice.Delta.ToolCalls {
			partial, exists := indexMap[tc.Index]
			if !exists {
				// First chunk for this index — has ID and function.name
				partial = &partialToolCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
				}
				indexMap[tc.Index] = partial
				events <- provider.StreamEvent{
					Type: provider.EventToolUseStart,
					ID:   tc.ID,
					Name: tc.Function.Name,
				}
			}
			// Append argument fragment
			if tc.Function.Arguments != "" {
				partial.Args.WriteString(tc.Function.Arguments)
				events <- provider.StreamEvent{
					Type: provider.EventToolUseDelta,
					Text: tc.Function.Arguments,
				}
			}
		}

		// finish_reason signals end of generation
		if choice.FinishReason != nil {
			reason := *choice.FinishReason
			stopReason := "end_turn"
			if reason == "tool_calls" {
				stopReason = "tool_use"
			} else if reason == "length" {
				stopReason = "max_tokens"
			}

			flushToolCalls(indexMap, events)

			usage := provider.Usage{
				InputTokens:  inputTokens,
				OutputTokens: outputTokens,
			}
			if chunk.Usage != nil {
				usage.InputTokens = chunk.Usage.PromptTokens
				usage.OutputTokens = chunk.Usage.CompletionTokens
			}
			events <- provider.StreamEvent{
				Type:       provider.EventDone,
				Usage:      &usage,
				StopReason: stopReason,
			}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		events <- provider.StreamEvent{
			Type: provider.EventError,
			Err:  fmt.Errorf("read stream: %w", err),
		}
		return
	}

	// Stream ended without finish_reason or [DONE] — emit Done anyway.
	flushToolCalls(indexMap, events)
	events <- provider.StreamEvent{
		Type:       provider.EventDone,
		Usage:      &provider.Usage{InputTokens: inputTokens, OutputTokens: outputTokens},
		StopReason: "end_turn",
	}
}

// flushToolCalls is a no-op — tool calls are flushed incrementally via
// EventToolUseStart + EventToolUseDelta. The Collect() state machine in
// stream.go handles final assembly. This just clears the map.
func flushToolCalls(indexMap map[int]*partialToolCall, _ chan<- provider.StreamEvent) {
	for k := range indexMap {
		delete(indexMap, k)
	}
}

// --- Streaming chunk types ---

type streamChunk struct {
	Choices []streamChoice `json:"choices"`
	Usage   *streamUsage   `json:"usage,omitempty"`
}

type streamChoice struct {
	Delta        streamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

type streamDelta struct {
	Content   string            `json:"content"`
	ToolCalls []streamToolDelta `json:"tool_calls"`
}

type streamToolDelta struct {
	Index    int              `json:"index"`
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function streamFuncDelta  `json:"function"`
}

type streamFuncDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type streamUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// --- Error handling ---

func parseHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))

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
	// Truncate to avoid leaking context in error messages.
	if len(msg) > 256 {
		msg = msg[:256] + "..."
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", ErrRateLimited, msg)
	case http.StatusServiceUnavailable:
		return fmt.Errorf("%w: %s", ErrOverloaded, msg)
	default:
		return fmt.Errorf("%w: HTTP %d: %s", ErrAPI, resp.StatusCode, msg)
	}
}
