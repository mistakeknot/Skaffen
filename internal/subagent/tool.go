package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
)

// AgentTool is an agentloop.Tool that allows the LLM to spawn subagents.
type AgentTool struct {
	registry *TypeRegistry
	runner   *Runner
}

// agentToolInput defines the JSON input schema for the Agent tool.
type agentToolInput struct {
	SubagentType string   `json:"subagent_type"`
	Prompt       string   `json:"prompt"`
	Description  string   `json:"description"`
	Context      []string `json:"context,omitempty"`
	FilePatterns []string `json:"file_patterns,omitempty"`
}

// NewAgentTool creates an Agent tool. Runner can be nil during schema-only
// usage (e.g., startup before provider is connected).
func NewAgentTool(reg *TypeRegistry, runner *Runner) *AgentTool {
	return &AgentTool{registry: reg, runner: runner}
}

// SetRunner sets the runner after construction (for lazy initialization).
func (t *AgentTool) SetRunner(r *Runner) {
	t.runner = r
}

func (t *AgentTool) Name() string { return "Agent" }

func (t *AgentTool) Description() string {
	return "Launch a subagent to handle a focused task autonomously. " +
		"Use for parallel research, codebase exploration, or delegating independent work. " +
		"Multiple Agent calls in a single turn run concurrently."
}

func (t *AgentTool) Schema() json.RawMessage {
	typeNames := t.registry.Names()
	typeEnum, _ := json.Marshal(typeNames)

	// Build type descriptions for the enum.
	var typeDescs []string
	for _, name := range typeNames {
		st, _ := t.registry.Get(name)
		typeDescs = append(typeDescs, fmt.Sprintf("%s: %s", name, st.Description))
	}
	typeDescText := "The type of subagent to spawn. Available types:\n" + strings.Join(typeDescs, "\n")
	typeDescJSON, _ := json.Marshal(typeDescText)

	schema := fmt.Sprintf(`{
		"type": "object",
		"required": ["subagent_type", "prompt", "description"],
		"properties": {
			"subagent_type": {
				"type": "string",
				"enum": %s,
				"description": %s
			},
			"prompt": {
				"type": "string",
				"description": "The task for the subagent to perform. Be specific and self-contained."
			},
			"description": {
				"type": "string",
				"description": "A short (3-5 word) description shown in the UI status."
			},
			"context": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Optional context strings to inject into the subagent session."
			},
			"file_patterns": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Glob patterns for files this subagent will modify. Required for write-capable types."
			}
		}
	}`, typeEnum, typeDescJSON)

	return json.RawMessage(schema)
}

func (t *AgentTool) Execute(ctx context.Context, params json.RawMessage) agentloop.ToolResult {
	var input agentToolInput
	if err := json.Unmarshal(params, &input); err != nil {
		return agentloop.ToolResult{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}
	}

	// Validate type exists.
	if _, err := t.registry.Get(input.SubagentType); err != nil {
		return agentloop.ToolResult{Content: err.Error(), IsError: true}
	}

	if t.runner == nil {
		return agentloop.ToolResult{Content: "subagent runner not initialized", IsError: true}
	}

	// Build injected context from context array.
	injected := strings.Join(input.Context, "\n\n")

	task := SubagentTask{
		Type:            input.SubagentType,
		Prompt:          input.Prompt,
		Description:     input.Description,
		InjectedContext: injected,
		FilePatterns:    input.FilePatterns,
	}

	results, err := t.runner.Run(ctx, []SubagentTask{task})
	if err != nil {
		return agentloop.ToolResult{Content: fmt.Sprintf("subagent failed: %v", err), IsError: true}
	}
	if len(results) == 0 {
		return agentloop.ToolResult{Content: "no results from subagent", IsError: true}
	}

	r := results[0]
	if r.Error != nil {
		return agentloop.ToolResult{
			Content: fmt.Sprintf("Subagent %q failed: %v", r.Description, r.Error),
			IsError: true,
		}
	}

	content := fmt.Sprintf("Subagent %q completed (%d turns, %d tokens):\n\n%s",
		r.Description, r.Turns, r.Usage.InputTokens+r.Usage.OutputTokens, r.Response)

	return agentloop.ToolResult{Content: content, IsError: false}
}
