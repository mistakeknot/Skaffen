// Hassease is a headless multi-model code execution daemon.
// It routes routine code tasks to cheap models (GLM, Qwen) and escalates
// complex work to Claude. Named after the Mind from Excession.
//
// Usage:
//
//	echo "read the file at main.go" | hassease
//	echo "fix the bug in auth.go" | hassease --approve-edits
//	echo "fix auth.go" | hassease --signal --config hassease.yaml
//	hassease --config hassease.yaml < task.txt
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	ossignal "os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/costrouter"
	"github.com/mistakeknot/Skaffen/internal/signal"

	// Provider registration — both must be blank-imported or their
	// init() functions never run and provider.New() returns "unknown provider".
	_ "github.com/mistakeknot/Skaffen/internal/provider/anthropic"
	_ "github.com/mistakeknot/Skaffen/internal/provider/openai"
)

func main() {
	var (
		configPath   = flag.String("config", "", "path to YAML config (uses defaults if empty)")
		approveEdits = flag.Bool("approve-edits", false, "allow edit/write tools (denied by default)")
		approveBash  = flag.Bool("approve-bash", false, "allow bash tool (denied by default)")
		useSignal    = flag.Bool("signal", false, "use Signal transport for tool approval (requires signal config)")
		sessionFlag  = flag.String("session", "", "session ID to resume (generates new if empty)")
		sessionDir   = flag.String("session-dir", "", "directory for session JSONL files (default: ~/.skaffen/hassease/sessions)")
		evidenceDir  = flag.String("evidence-dir", "", "directory for evidence JSONL files (default: ~/.skaffen/hassease/evidence)")
		taskType     = flag.String("task-type", "code", "task type hint: code, chat, analysis")
		urgency      = flag.String("urgency", "batch", "urgency hint: interactive, batch, background")
		maxTurns     = flag.Int("max-turns", 50, "maximum agent loop turns")
	)
	flag.Parse()

	// Session identity — resume or generate.
	sessionID := *sessionFlag
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	// Resolve data directories.
	homeDir, _ := os.UserHomeDir()
	hassDir := filepath.Join(homeDir, ".skaffen", "hassease")
	if *sessionDir == "" {
		*sessionDir = filepath.Join(hassDir, "sessions")
	}
	if *evidenceDir == "" {
		*evidenceDir = filepath.Join(hassDir, "evidence")
	}

	// Cancellation context — catches SIGINT and SIGTERM for graceful shutdown.
	ctx, cancel := ossignal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Load config.
	var cfg *HassConfig
	if *configPath != "" {
		var err error
		cfg, err = loadConfig(*configPath)
		if err != nil {
			fatalf("config: %v", err)
		}
	} else {
		cfg = defaultConfig()
	}

	// Pre-flight: validate API keys.
	backends, err := buildBackends(cfg)
	if err != nil {
		fatalf("backends: %v", err)
	}

	// Pre-flight: require clean git working tree (rollback safety).
	// Skipped if not in a git repo — not all use cases are git-tracked.
	if !gitClean() {
		fmt.Fprintln(os.Stderr, "hassease: warning: git working tree is dirty — edits cannot be reverted with git checkout")
		if *approveEdits || *useSignal {
			fmt.Fprintln(os.Stderr, "hassease: refusing to approve edits on dirty tree")
			os.Exit(1)
		}
	}

	// Read task from stdin.
	task := readStdin()
	if task == "" {
		fatalf("no task provided on stdin")
	}

	// Build the cost router (Router + Emitter + provider dispatch).
	router := costrouter.New(cfg.CostRouter, backends)
	dispatch := &costrouter.DispatchProvider{Router: router}

	// Build tool registry with whitelist.
	registry := buildRegistry(cfg.Tools.Allowed)

	// Build tool approver — Signal transport or CLI-flag headless mode.
	allowed := makeStringSet(cfg.Tools.Allowed)
	autoApprove := makeStringSet(cfg.Tools.AutoApprove)

	var approver agentloop.ToolApprover
	if *useSignal {
		scfg := cfg.Signal.signalClientConfig()
		if scfg.Account == "" || scfg.Recipient == "" {
			fatalf("--signal requires signal.account and signal.recipient in config")
		}
		signalClient, err := signal.New(scfg)
		if err != nil {
			fatalf("signal: %v", err)
		}
		patterns := cfg.Signal.TestPatterns
		if len(patterns) == 0 {
			patterns = defaultTestPatterns()
		}
		approver = signalApprover(ctx, signalClient, allowed, autoApprove, patterns)
		fmt.Fprintf(os.Stderr, "hassease: signal approval enabled -> %s\n", cfg.Signal.Recipient)
	} else {
		approver = headlessApprover(allowed, autoApprove, *approveEdits, *approveBash)
	}

	// System prompt for the code execution agent.
	systemPrompt := `You are Hassease, a headless code execution agent. You execute code tasks precisely and efficiently.

Rules:
- Use the available tools to accomplish the task
- Read files before editing them
- Be concise in your responses
- If you cannot complete the task, explain why`

	// Session persistence — JSONL-backed, supports resume.
	sess := newHassSession(sessionID, *sessionDir, systemPrompt)
	if err := sess.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "hassease: warning: load session: %v\n", err)
	}

	// Evidence emission — tee to CostRouter (failure feedback) + JSONL (persistence).
	jsonlEm := newJSONLEmitter(*evidenceDir, sessionID)
	emitter := newTeeEmitter(router, jsonlEm)

	// Lifecycle — manages file reservations and graceful shutdown.
	workDir, _ := os.Getwd()
	lc := newLifecycle(sessionID, sess, workDir)
	defer lc.Shutdown()

	fmt.Fprintf(os.Stderr, "hassease: session %s\n", sessionID)

	// Lifecycle hooks — shared config with skaffen (user-global + per-project).
	loopOpts := []agentloop.Option{
		agentloop.WithRouter(router),
		agentloop.WithSession(sess),
		agentloop.WithEmitter(emitter),
		agentloop.WithMaxTurns(*maxTurns),
	}
	if hookOpt, hookExec := wrapWithHooks(sessionID, workDir); hookOpt != nil {
		loopOpts = append(loopOpts, hookOpt)
		hookExec.SessionStart(ctx, "hassease")
		fmt.Fprintf(os.Stderr, "hassease: hooks loaded\n")
	}

	// Build the agent loop.
	loop := agentloop.New(dispatch, registry, loopOpts...)
	loop.SetToolApprover(approver)

	// Run.
	result, err := loop.Run(ctx, task, agentloop.LoopConfig{
		Hints: agentloop.SelectionHints{
			TaskType: *taskType,
			Urgency:  *urgency,
		},
	})
	if err != nil {
		fatalf("agent loop: %v", err)
	}

	// Output result.
	if result.Response != "" {
		fmt.Println(result.Response)
	}

	fmt.Fprintf(os.Stderr, "hassease: %d turns, %d input + %d output tokens\n",
		result.Turns,
		result.Usage.InputTokens,
		result.Usage.OutputTokens,
	)
}

// readStdin reads all of stdin, trimming whitespace.
func readStdin() string {
	scanner := bufio.NewScanner(os.Stdin)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// gitClean returns true if the git working tree is clean (or not a git repo).
func gitClean() bool {
	// Quick check: is this a git repo?
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		return true // not a repo, skip check
	}

	cmd := exec.Command("git", "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return true // git not available, skip check
	}
	return strings.TrimSpace(string(out)) == ""
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "hassease: "+format+"\n", args...)
	os.Exit(1)
}
