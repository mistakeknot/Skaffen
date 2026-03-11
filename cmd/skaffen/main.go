package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/evidence"
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/router"
	"github.com/mistakeknot/Skaffen/internal/session"
	"github.com/mistakeknot/Skaffen/internal/tool"

	// Register providers via init()
	_ "github.com/mistakeknot/Skaffen/internal/provider/anthropic"
	_ "github.com/mistakeknot/Skaffen/internal/provider/claudecode"
)

var (
	flagProvider = flag.String("provider", "", "LLM provider: claude-code (default), anthropic")
	flagModel    = flag.String("model", "", "Model override")
	flagPhase    = flag.String("phase", "build", "OODARC phase (brainstorm, plan, build, review, ship)")
	flagPrompt   = flag.String("p", "", "Prompt text (reads stdin if empty)")
	flagMaxTurns = flag.Int("max-turns", 100, "Maximum agent loop turns")
	flagSystem   = flag.String("system", "", "System prompt")
	flagSession  = flag.String("session", "", "Session ID for persistence (creates ~/.skaffen/sessions/<id>.jsonl)")
	flagBudget   = flag.Int("budget", 0, "Per-session token budget (0 = unlimited)")
)

func main() {
	flag.Parse()

	// Subcommand routing
	if flag.NArg() > 0 {
		switch flag.Arg(0) {
		case "version":
			printVersion()
			return
		default:
			fmt.Fprintf(os.Stderr, "skaffen: unknown command %q\n", flag.Arg(0))
			os.Exit(1)
		}
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "skaffen: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Validate phase
	phase := tool.Phase(*flagPhase)
	switch phase {
	case tool.PhaseBrainstorm, tool.PhasePlan, tool.PhaseBuild, tool.PhaseReview, tool.PhaseShip:
		// valid
	default:
		return fmt.Errorf("invalid phase %q (must be brainstorm, plan, build, review, or ship)", *flagPhase)
	}

	// Read prompt from -p flag or stdin
	prompt := *flagPrompt
	if prompt == "" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
		prompt = strings.TrimSpace(string(data))
	}
	if prompt == "" {
		return fmt.Errorf("no prompt provided (use -p or pipe to stdin)")
	}

	// Resolve provider: auto-detect if not specified
	providerName := *flagProvider
	if providerName == "" {
		if os.Getenv("ANTHROPIC_API_KEY") != "" {
			providerName = "anthropic"
		} else {
			providerName = "claude-code" // default — works with Claude Max OAuth
		}
	}

	cfg := provider.ProviderConfig{}
	if providerName == "anthropic" {
		cfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		if cfg.APIKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY not set (omit --provider to use Claude Max via claude-code)")
		}
	}

	p, err := provider.New(providerName, cfg)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}

	// Initialize tool registry
	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	// Load routing config (optional file, env vars always checked)
	routingPath := filepath.Join(os.Getenv("HOME"), ".skaffen", "routing.json")
	routerCfg, err := router.LoadConfig(routingPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skaffen: warning: routing config: %v\n", err)
		routerCfg = &router.Config{}
	}

	// CLI --budget flag overrides config file budget
	if *flagBudget > 0 {
		routerCfg.Budget = &router.BudgetConfig{
			MaxTokens: *flagBudget,
			Mode:      "graceful",
			DegradeAt: 0.8,
		}
	}

	// CLI --model flag: set as override for all phases (backward compat)
	if *flagModel != "" {
		if routerCfg.Phases == nil {
			routerCfg.Phases = make(map[tool.Phase]string)
		}
		for _, ph := range []tool.Phase{tool.PhaseBrainstorm, tool.PhasePlan, tool.PhaseBuild, tool.PhaseReview, tool.PhaseShip} {
			routerCfg.Phases[ph] = *flagModel
		}
	}

	modelRouter := router.New(routerCfg)

	// Configure agent
	opts := []agent.Option{
		agent.WithMaxTurns(*flagMaxTurns),
		agent.WithStartPhase(phase),
		agent.WithRouter(modelRouter),
	}

	// Session ID — used for both session persistence and evidence
	sessionID := *flagSession
	if sessionID == "" {
		sessionID = fmt.Sprintf("skaffen-%d", os.Getpid())
	}

	if *flagSession != "" {
		dir := filepath.Join(os.Getenv("HOME"), ".skaffen", "sessions")
		sess := session.New(*flagSession, dir, *flagSystem, 20)
		if err := sess.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: warning: load session: %v\n", err)
		}
		opts = append(opts, agent.WithSession(sess))
	} else if *flagSystem != "" {
		opts = append(opts, agent.WithSession(&agent.NoOpSession{Prompt: *flagSystem}))
	}

	// Evidence emission — always enabled
	evidenceDir := filepath.Join(os.Getenv("HOME"), ".skaffen", "evidence")
	emitter := evidence.New(evidenceDir, sessionID)
	opts = append(opts, agent.WithEmitter(emitter), agent.WithSessionID(sessionID))

	a := agent.New(p, reg, opts...)

	// Run agent loop
	result, err := a.Run(ctx, prompt)
	if err != nil {
		return err
	}

	// Print response to stdout
	fmt.Print(result.Response)

	// Print usage to stderr
	fmt.Fprintf(os.Stderr, "\n[%d turns, %d in / %d out tokens]\n",
		result.Turns, result.Usage.InputTokens, result.Usage.OutputTokens)

	return nil
}

func printVersion() {
	version := "dev"
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		version = info.Main.Version
	}
	fmt.Printf("skaffen %s (%s)\n", version, runtime.Version())
}
