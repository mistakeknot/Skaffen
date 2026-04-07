// Hassease is a headless multi-model code execution daemon.
// It routes routine code tasks to cheap models (GLM, Qwen) and escalates
// complex work to Claude. Named after the Mind from Excession.
//
// Usage:
//
//	echo "read the file at main.go" | hassease
//	echo "fix the bug in auth.go" | hassease --approve-edits
//	hassease --config hassease.yaml < task.txt
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/costrouter"

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
		taskType     = flag.String("task-type", "code", "task type hint: code, chat, analysis")
		urgency      = flag.String("urgency", "batch", "urgency hint: interactive, batch, background")
		maxTurns     = flag.Int("max-turns", 50, "maximum agent loop turns")
	)
	flag.Parse()

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
		if *approveEdits {
			fmt.Fprintln(os.Stderr, "hassease: refusing to run with --approve-edits on dirty tree")
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

	// Build headless approver.
	allowed := makeStringSet(cfg.Tools.Allowed)
	autoApprove := makeStringSet(cfg.Tools.AutoApprove)
	approver := headlessApprover(allowed, autoApprove, *approveEdits, *approveBash)

	// System prompt for the code execution agent.
	systemPrompt := `You are Hassease, a headless code execution agent. You execute code tasks precisely and efficiently.

Rules:
- Use the available tools to accomplish the task
- Read files before editing them
- Be concise in your responses
- If you cannot complete the task, explain why`

	session := &agentloop.NoOpSession{Prompt: systemPrompt}

	// Build the agent loop.
	loop := agentloop.New(dispatch, registry,
		agentloop.WithRouter(router),
		agentloop.WithSession(session),
		agentloop.WithEmitter(router), // CostRouter implements Emitter for failure feedback
		agentloop.WithMaxTurns(*maxTurns),
	)
	loop.SetToolApprover(approver)

	// Run with cancellation support.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

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
