package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Default timeouts per event type (seconds).
const (
	DefaultTimeoutPreToolUse  = 10
	DefaultTimeoutPostToolUse = 5
	DefaultTimeoutSession     = 30
	DefaultTimeoutNotify      = 5
	MaxTimeout                = 120 // hard cap on any hook timeout
)

// credentialPrefixes are env var name prefixes stripped from hook environments.
var credentialPrefixes = []string{
	"ANTHROPIC_API_KEY",
	"OPENAI_API_KEY",
	"AWS_ACCESS_KEY_ID",
	"AWS_SECRET_ACCESS_KEY",
	"AWS_SESSION_TOKEN",
	"AWS_SECURITY_TOKEN",
	"GITHUB_TOKEN",
	"GH_TOKEN",
	"GITLAB_TOKEN",
	"BITBUCKET_TOKEN",
	"NPM_TOKEN",
	"PYPI_API_TOKEN",
	"DATABASE_URL",
}

// credentialSuffixes are env var name suffixes that indicate credentials.
var credentialSuffixes = []string{
	"_SECRET",
	"_TOKEN",
	"_API_KEY",
	"_PASSWORD",
}

// Executor runs hook commands for lifecycle events.
type Executor struct {
	config    *Config
	sessionID string
	workDir   string
	phase     string
	logger    *log.Logger
}

// NewExecutor creates a hook executor. Pass empty config for no-op behavior.
func NewExecutor(cfg *Config, sessionID, workDir, phase string) *Executor {
	if cfg == nil {
		cfg = &Config{Hooks: make(map[Event][]HookGroup)}
	}
	return &Executor{
		config:    cfg,
		sessionID: sessionID,
		workDir:   workDir,
		phase:     phase,
		logger:    log.New(os.Stderr, "skaffen: ", 0),
	}
}

// SetPhase updates the current OODARC phase for env var injection.
func (e *Executor) SetPhase(phase string) { e.phase = phase }

// PreToolUse runs PreToolUse hooks and returns the most restrictive decision.
// deny > ask > allow. First "deny" short-circuits.
// On error/timeout: respects hook's OnError field ("allow" default, "deny" for fail-closed).
func (e *Executor) PreToolUse(ctx context.Context, toolName string, input json.RawMessage) (Decision, error) {
	groups := e.matchingGroups(EventPreToolUse, toolName)
	if len(groups) == 0 {
		return DecisionAllow, nil
	}

	payload := PreToolUsePayload{ToolName: toolName, ToolInput: input}
	mostRestrictive := DecisionAllow

	for _, group := range groups {
		for _, hook := range group.Hooks {
			result, err := e.runHook(ctx, hook, DefaultTimeoutPreToolUse, payload)
			if err != nil {
				e.logger.Printf("warning: PreToolUse hook %q: %v", hook.Command, err)
				if hook.OnError == "deny" {
					return DecisionDeny, nil // fail-closed for security hooks
				}
				continue // fail-open (default)
			}
			switch result.Decision {
			case DecisionDeny:
				return DecisionDeny, nil // short-circuit on deny
			case DecisionAsk:
				mostRestrictive = DecisionAsk // escalate, but keep running remaining hooks
			}
		}
	}
	return mostRestrictive, nil
}

// PostToolUse runs PostToolUse hooks (advisory, fire-and-forget).
func (e *Executor) PostToolUse(ctx context.Context, toolName string, input json.RawMessage, result string, isError bool) {
	groups := e.matchingGroups(EventPostToolUse, toolName)
	if len(groups) == 0 {
		return
	}

	payload := PostToolUsePayload{
		ToolName:   toolName,
		ToolInput:  input,
		ToolResult: result,
		IsError:    isError,
	}

	for _, group := range groups {
		for _, hook := range group.Hooks {
			if _, err := e.runHook(ctx, hook, DefaultTimeoutPostToolUse, payload); err != nil {
				e.logger.Printf("warning: PostToolUse hook %q: %v", hook.Command, err)
			}
		}
	}
}

// SessionStart runs SessionStart hooks (advisory).
func (e *Executor) SessionStart(ctx context.Context, mode string) {
	groups := e.config.Hooks[EventSessionStart]
	if len(groups) == 0 {
		return
	}

	payload := SessionStartPayload{
		SessionID: e.sessionID,
		WorkDir:   e.workDir,
		Mode:      mode,
	}

	for _, group := range groups {
		for _, hook := range group.Hooks {
			if _, err := e.runHook(ctx, hook, DefaultTimeoutSession, payload); err != nil {
				e.logger.Printf("warning: SessionStart hook %q: %v", hook.Command, err)
			}
		}
	}
}

// Notify runs Notification hooks (advisory, fire-and-forget).
func (e *Executor) Notify(ctx context.Context, eventType, message, severity string) {
	groups := e.config.Hooks[EventNotification]
	if len(groups) == 0 {
		return
	}

	payload := NotificationPayload{
		EventType: eventType,
		Message:   message,
		Severity:  severity,
	}

	for _, group := range groups {
		for _, hook := range group.Hooks {
			if _, err := e.runHook(ctx, hook, DefaultTimeoutNotify, payload); err != nil {
				e.logger.Printf("warning: Notification hook %q: %v", hook.Command, err)
			}
		}
	}
}

// matchingGroups returns hook groups whose matcher matches the tool name.
func (e *Executor) matchingGroups(event Event, toolName string) []HookGroup {
	var matched []HookGroup
	for _, group := range e.config.Hooks[event] {
		if group.Matcher == "*" || group.Matcher == toolName {
			matched = append(matched, group)
			continue
		}
		if ok, _ := filepath.Match(group.Matcher, toolName); ok {
			matched = append(matched, group)
		}
	}
	return matched
}

// safeEnv returns os.Environ() with credential-bearing env vars stripped.
// Matches by exact prefix on the key name and by suffix patterns like _SECRET, _TOKEN.
func safeEnv() []string {
	var filtered []string
	for _, kv := range os.Environ() {
		eqIdx := strings.IndexByte(kv, '=')
		if eqIdx < 0 {
			continue
		}
		key := kv[:eqIdx]
		upper := strings.ToUpper(key)

		if isCredentialKey(upper) {
			continue
		}
		filtered = append(filtered, kv)
	}
	return filtered
}

// isCredentialKey returns true if the env var name looks like a credential.
func isCredentialKey(key string) bool {
	for _, prefix := range credentialPrefixes {
		if key == prefix {
			return true
		}
	}
	for _, suffix := range credentialSuffixes {
		if strings.HasSuffix(key, suffix) {
			return true
		}
	}
	return false
}

// runHook executes a single hook command with timeout and JSON stdin/stdout.
func (e *Executor) runHook(ctx context.Context, hook HookDef, defaultTimeout int, payload interface{}) (*HookResult, error) {
	timeout := hook.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if timeout > MaxTimeout {
		timeout = MaxTimeout
	}
	hookCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	cmd := exec.CommandContext(hookCtx, "sh", "-c", hook.Command)
	cmd.Stdin = bytes.NewReader(payloadBytes)
	// WaitDelay ensures I/O goroutines drain after context cancellation.
	// Without this, cmd.Wait blocks if child processes inherit pipes.
	cmd.WaitDelay = 500 * time.Millisecond
	cmd.Env = append(safeEnv(),
		"SKAFFEN_SESSION_ID="+e.sessionID,
		"SKAFFEN_WORK_DIR="+e.workDir,
		"SKAFFEN_PHASE="+e.phase,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("hook %q: %w (stderr: %s)", hook.Command, err, stderr.String())
	}

	result := &HookResult{Output: stdout.String()}
	// Parse JSON output for decision (PreToolUse)
	if stdout.Len() > 0 {
		if err := json.Unmarshal(stdout.Bytes(), result); err != nil {
			e.logger.Printf("warning: hook %q returned invalid JSON: %v", hook.Command, err)
			// Fail-open: treat as allow
		}
	}
	return result, nil
}
