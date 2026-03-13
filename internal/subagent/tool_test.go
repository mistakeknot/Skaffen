package subagent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAgentTool_Schema(t *testing.T) {
	reg := NewTypeRegistry("")
	tool := NewAgentTool(reg, nil)

	schema := tool.Schema()
	var s map[string]interface{}
	if err := json.Unmarshal(schema, &s); err != nil {
		t.Fatalf("Schema() not valid JSON: %v", err)
	}

	required, _ := s["required"].([]interface{})
	requiredNames := make([]string, len(required))
	for i, r := range required {
		requiredNames[i], _ = r.(string)
	}

	for _, name := range []string{"subagent_type", "prompt", "description"} {
		found := false
		for _, r := range requiredNames {
			if r == name {
				found = true
			}
		}
		if !found {
			t.Errorf("missing required field %q", name)
		}
	}
}

func TestAgentTool_Name(t *testing.T) {
	reg := NewTypeRegistry("")
	tool := NewAgentTool(reg, nil)
	if tool.Name() != "Agent" {
		t.Errorf("Name() = %q, want 'Agent'", tool.Name())
	}
}

func TestAgentTool_InvalidType(t *testing.T) {
	reg := NewTypeRegistry("")
	tool := NewAgentTool(reg, nil)

	input := `{"subagent_type":"nonexistent","prompt":"test","description":"test"}`
	result := tool.Execute(context.Background(), json.RawMessage(input))
	if !result.IsError {
		t.Error("should error on unknown type")
	}
	if !strings.Contains(result.Content, "unknown subagent type") {
		t.Errorf("error message = %q, want 'unknown subagent type'", result.Content)
	}
}

func TestAgentTool_NoRunner(t *testing.T) {
	reg := NewTypeRegistry("")
	tool := NewAgentTool(reg, nil)

	input := `{"subagent_type":"explore","prompt":"test","description":"test"}`
	result := tool.Execute(context.Background(), json.RawMessage(input))
	if !result.IsError {
		t.Error("should error when runner is nil")
	}
	if !strings.Contains(result.Content, "not initialized") {
		t.Errorf("error message = %q, want 'not initialized'", result.Content)
	}
}

func TestAgentTool_InvalidJSON(t *testing.T) {
	reg := NewTypeRegistry("")
	tool := NewAgentTool(reg, nil)

	result := tool.Execute(context.Background(), json.RawMessage(`{invalid`))
	if !result.IsError {
		t.Error("should error on invalid JSON")
	}
}
