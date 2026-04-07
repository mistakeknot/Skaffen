package openai

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// ErrSensitiveContent is returned when the content filter detects credentials
// or secrets in messages about to be sent to an external provider.
var ErrSensitiveContent = fmt.Errorf("openai: message contains sensitive content — aborting request to external API")

// sensitiveFilePatterns matches file paths that should never be sent externally.
var sensitiveFilePatterns = []string{
	".env",
	"id_rsa",
	"id_ed25519",
	".pem",
	"credentials",
	".key",
	".secret",
}

// sensitiveContentPattern matches credential-like content in message text.
var sensitiveContentPattern = regexp.MustCompile(
	`(?i)(BEGIN\s+(PRIVATE|RSA|EC|DSA)\s+KEY|` +
		`(API_KEY|SECRET_KEY|ACCESS_KEY|PRIVATE_KEY|AUTH_TOKEN|PASSWORD)\s*[=:]\s*\S+|` +
		`_KEY=\S{8,}|_SECRET=\S{8,}|_TOKEN=\S{8,})`,
)

// FilterMessages scans messages for sensitive content before sending to
// external providers. Returns an error (not silent redaction) so the model
// gets explicit feedback about why the turn was aborted.
func FilterMessages(messages []oaiMessage) error {
	for _, msg := range messages {
		if err := checkContent(msg.Content); err != nil {
			return err
		}
	}
	return nil
}

// FilterProviderMessages scans provider.Messages (Anthropic format) for sensitive content.
func FilterProviderMessages(messages []provider.Message) error {
	for _, msg := range messages {
		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				if err := checkContent(block.Text); err != nil {
					return err
				}
			case "tool_result":
				if err := checkContent(block.ResultContent); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func checkContent(text string) error {
	if text == "" {
		return nil
	}

	// Check for sensitive file path references.
	lower := strings.ToLower(text)
	for _, pattern := range sensitiveFilePatterns {
		if strings.Contains(lower, pattern) {
			return fmt.Errorf("%w (matched file pattern: %s)", ErrSensitiveContent, pattern)
		}
	}

	// Check for credential-like content.
	if sensitiveContentPattern.MatchString(text) {
		return fmt.Errorf("%w (matched credential pattern)", ErrSensitiveContent)
	}

	return nil
}
