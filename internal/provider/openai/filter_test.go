package openai

import (
	"errors"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

func TestFilterMessages_Clean(t *testing.T) {
	msgs := []oaiMessage{
		{Role: "user", Content: "read the file at main.go"},
		{Role: "assistant", Content: "package main\nfunc main() {}"},
	}
	if err := FilterMessages(msgs); err != nil {
		t.Errorf("expected no error for clean messages, got: %v", err)
	}
}

func TestFilterMessages_DotEnv(t *testing.T) {
	msgs := []oaiMessage{
		{Role: "user", Content: "read the file at /home/user/.env"},
	}
	if err := FilterMessages(msgs); err == nil {
		t.Error("expected error for .env file reference")
	} else if !errors.Is(err, ErrSensitiveContent) {
		t.Errorf("expected ErrSensitiveContent, got: %v", err)
	}
}

func TestFilterMessages_PrivateKey(t *testing.T) {
	msgs := []oaiMessage{
		{Role: "tool", Content: "-----BEGIN PRIVATE KEY-----\nMIIE...", ToolCallID: "t1"},
	}
	if err := FilterMessages(msgs); err == nil {
		t.Error("expected error for private key")
	}
}

func TestFilterMessages_APIKey(t *testing.T) {
	msgs := []oaiMessage{
		{Role: "tool", Content: "API_KEY=sk-1234567890abcdef", ToolCallID: "t1"},
	}
	if err := FilterMessages(msgs); err == nil {
		t.Error("expected error for API key")
	}
}

func TestFilterMessages_SSHKey(t *testing.T) {
	msgs := []oaiMessage{
		{Role: "user", Content: "show me the contents of id_rsa"},
	}
	if err := FilterMessages(msgs); err == nil {
		t.Error("expected error for SSH key reference")
	}
}

func TestFilterMessages_PemFile(t *testing.T) {
	msgs := []oaiMessage{
		{Role: "tool", Content: "reading server.pem: certificate data...", ToolCallID: "t1"},
	}
	if err := FilterMessages(msgs); err == nil {
		t.Error("expected error for .pem file")
	}
}

func TestFilterProviderMessages_ToolResult(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{
			{Type: "tool_result", ToolUseID: "t1", ResultContent: "SECRET_KEY=mysupersecretvalue123"},
		}},
	}
	if err := FilterProviderMessages(msgs); err == nil {
		t.Error("expected error for secret in tool result")
	}
}

func TestFilterProviderMessages_Clean(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{
			{Type: "text", Text: "read main.go"},
		}},
		{Role: provider.RoleUser, Content: []provider.ContentBlock{
			{Type: "tool_result", ToolUseID: "t1", ResultContent: "package main\nfunc main() {}"},
		}},
	}
	if err := FilterProviderMessages(msgs); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestFilterMessages_EnvVarToken(t *testing.T) {
	msgs := []oaiMessage{
		{Role: "tool", Content: "AUTH_TOKEN=eyJhbGciOiJIUzI1NiJ9.long.token.here"},
	}
	if err := FilterMessages(msgs); err == nil {
		t.Error("expected error for auth token")
	}
}
