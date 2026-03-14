package trust

import (
	"encoding/json"
	"strings"
)

// Safe tools that are always allowed (local filesystem only).
// web_search and web_fetch are deliberately NOT here — web_search costs money
// per call and sends queries to Exa; web_fetch has SSRF risk. The user gate
// on web_fetch is load-bearing for prompt injection defense.
var safeTools = map[string]bool{
	"read":  true,
	"write": true,
	"edit":  true,
	"grep":  true,
	"glob":  true,
	"ls":    true,
}

// Safe bash command prefixes
var safeBashPrefixes = []string{
	"go test", "go build", "go vet", "go run", "go mod",
	"git status", "git diff", "git log", "git show", "git branch",
	"ls", "cat", "head", "tail", "wc", "echo", "mkdir", "pwd",
	"tree", "find", "which", "command -v",
}

// Dangerous bash patterns — deny-by-default for destructive commands.
// These return Block regardless of learned overrides.
var dangerousPatterns = []string{
	"rm -rf", "rm -r ", "sudo", "chmod 777",
	"curl", "wget", "nc ", "ncat",
	".env",
	"git push --force", "git push -f", "git reset --hard",
	"git clean -f", "git checkout -- .",
	"find . -delete", "find . -exec rm",
	"truncate ", "> /", "dd if=",
	"mkfs.", "fdisk",
}

func evaluateBuiltIn(toolName, paramsJSON string) Decision {
	// Safe tools always allowed
	if safeTools[toolName] {
		return Allow
	}

	// Bash: check command
	if toolName == "bash" {
		cmd := extractBashCommand(paramsJSON)
		return evaluateBashCommand(cmd)
	}

	// Everything else: prompt
	return Prompt
}

func evaluateBashCommand(cmd string) Decision {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return Prompt
	}

	// Check dangerous patterns first
	lower := strings.ToLower(cmd)
	for _, p := range dangerousPatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return Block
		}
	}

	// Check safe prefixes
	for _, prefix := range safeBashPrefixes {
		if strings.HasPrefix(cmd, prefix) {
			return Allow
		}
	}

	return Prompt
}

func extractBashCommand(paramsJSON string) string {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return ""
	}
	if cmd, ok := params["command"].(string); ok {
		return cmd
	}
	return ""
}
