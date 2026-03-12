package agent

import (
	"encoding/json"

	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"
	"github.com/mistakeknot/Masaq/priompt"
)

// buildAssistantMessage constructs the assistant message from a collected response.
// Kept as a package-level function for test compatibility.
func buildAssistantMessage(c *provider.CollectedResponse) provider.Message {
	var blocks []provider.ContentBlock
	if c.Text != "" {
		blocks = append(blocks, provider.ContentBlock{Type: "text", Text: c.Text})
	}
	for _, tc := range c.ToolCalls {
		blocks = append(blocks, provider.ContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Name,
			Input: tc.Input,
		})
	}
	return provider.Message{Role: provider.RoleAssistant, Content: blocks}
}

// convertToolDefs converts tool.ToolDef to provider.ToolDef.
func convertToolDefs(defs []tool.ToolDef) []provider.ToolDef {
	out := make([]provider.ToolDef, len(defs))
	for i, d := range defs {
		out[i] = provider.ToolDef{
			Name:        d.Name,
			Description: d.Description,
			InputSchema: json.RawMessage(d.InputSchema),
		}
	}
	return out
}

// estimateMessageTokens estimates total tokens for a message slice using the
// same CharHeuristic tokenizer that priompt.Render uses internally. This
// ensures budget computation in the loop stays consistent with prompt rendering.
func estimateMessageTokens(msgs []provider.Message) int {
	h := priompt.CharHeuristic{Ratio: 4}
	total := 0
	for _, m := range msgs {
		for _, b := range m.Content {
			switch b.Type {
			case "text":
				total += h.Count(b.Text)
			case "tool_use":
				total += h.Count(b.Name)
				total += h.Count(string(b.Input))
			case "tool_result":
				total += h.Count(b.ResultContent)
			}
		}
	}
	return total
}
