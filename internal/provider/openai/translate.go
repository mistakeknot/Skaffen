package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// translateMessages converts Anthropic-canonical provider.Messages to OpenAI wire format.
// The agentloop accumulates messages in Anthropic format (tool_use/tool_result ContentBlocks).
// OpenAI requires: assistant messages with tool_calls array, separate role:"tool" messages.
func translateMessages(messages []provider.Message, system string) []oaiMessage {
	var result []oaiMessage

	if system != "" {
		result = append(result, oaiMessage{Role: "system", Content: system})
	}

	for _, msg := range messages {
		translated := translateMessage(msg)
		result = append(result, translated...)
	}

	return result
}

// translateMessage converts a single Anthropic-format message to one or more OpenAI messages.
// An assistant message with tool_use blocks becomes one message with tool_calls.
// A user message with tool_result blocks becomes separate role:"tool" messages.
func translateMessage(msg provider.Message) []oaiMessage {
	// Check what kinds of content blocks we have.
	hasToolUse := false
	hasToolResult := false
	for _, block := range msg.Content {
		switch block.Type {
		case "tool_use":
			hasToolUse = true
		case "tool_result":
			hasToolResult = true
		}
	}

	// Assistant message with tool calls.
	if msg.Role == provider.RoleAssistant && hasToolUse {
		return translateAssistantWithTools(msg)
	}

	// User message with tool results (response to tool calls).
	if msg.Role == provider.RoleUser && hasToolResult {
		return translateToolResults(msg)
	}

	// Plain text message — concatenate text blocks.
	var text strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" && block.Text != "" {
			text.WriteString(block.Text)
		}
	}
	if text.Len() == 0 {
		return nil
	}
	return []oaiMessage{{Role: string(msg.Role), Content: text.String()}}
}

// translateAssistantWithTools converts an assistant message containing tool_use
// blocks into an OpenAI assistant message with tool_calls.
func translateAssistantWithTools(msg provider.Message) []oaiMessage {
	var textParts strings.Builder
	var toolCalls []oaiToolCall

	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			textParts.WriteString(block.Text)
		case "tool_use":
			toolCalls = append(toolCalls, oaiToolCall{
				ID:   block.ID,
				Type: "function",
				Function: oaiCallFunc{
					Name:      block.Name,
					Arguments: string(block.Input),
				},
			})
		}
	}

	oai := oaiMessage{
		Role:      "assistant",
		ToolCalls: toolCalls,
	}
	if textParts.Len() > 0 {
		oai.Content = textParts.String()
	}
	return []oaiMessage{oai}
}

// translateToolResults converts tool_result blocks into separate role:"tool" messages.
// OpenAI requires each tool result to be a separate message with tool_call_id.
func translateToolResults(msg provider.Message) []oaiMessage {
	var results []oaiMessage

	for _, block := range msg.Content {
		switch block.Type {
		case "tool_result":
			content := block.ResultContent
			if block.IsError {
				content = fmt.Sprintf("Error: %s", content)
			}
			results = append(results, oaiMessage{
				Role:       "tool",
				Content:    content,
				ToolCallID: block.ToolUseID,
			})
		case "text":
			// Text blocks mixed with tool results — include as user message.
			if block.Text != "" {
				results = append(results, oaiMessage{
					Role:    "user",
					Content: block.Text,
				})
			}
		}
	}

	return results
}

// translateResponseToCanonical converts an OpenAI tool_calls response back to
// Anthropic-canonical ContentBlocks. Called by the agentloop's collectWithCallbacks
// via the standard StreamEvent protocol — this function is for non-streaming use.
func translateResponseToCanonical(toolCalls []oaiToolCall) []provider.ContentBlock {
	var blocks []provider.ContentBlock
	for _, tc := range toolCalls {
		blocks = append(blocks, provider.ContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}
	return blocks
}
