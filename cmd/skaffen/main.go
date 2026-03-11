package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"

	// Register providers via init()
	_ "github.com/mistakeknot/Skaffen/internal/provider/anthropic"
	_ "github.com/mistakeknot/Skaffen/internal/provider/claudecode"
)

var (
	flagProvider = flag.String("provider", "anthropic", "LLM provider (anthropic, claude-code)")
	flagModel    = flag.String("model", "", "Model override")
	flagPhase    = flag.String("phase", "build", "OODARC phase (brainstorm, plan, build, review, ship)")
	flagPrompt   = flag.String("p", "", "Prompt text (reads stdin if empty)")
	flagMaxTurns = flag.Int("max-turns", 100, "Maximum agent loop turns")
	flagSystem   = flag.String("system", "", "System prompt")
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

	// Initialize provider
	cfg := provider.ProviderConfig{
		Model: *flagModel,
	}
	if *flagProvider == "anthropic" {
		cfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		if cfg.APIKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY not set (use --provider claude-code for Claude Max)")
		}
	}

	p, err := provider.New(*flagProvider, cfg)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}

	// Initialize tool registry
	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	// Configure agent
	opts := []agent.Option{
		agent.WithMaxTurns(*flagMaxTurns),
		agent.WithStartPhase(phase),
	}
	if *flagSystem != "" {
		opts = append(opts, agent.WithSession(&agent.NoOpSession{Prompt: *flagSystem}))
	}

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
