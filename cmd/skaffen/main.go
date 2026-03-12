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
	"time"

	"github.com/mistakeknot/Masaq/theme"
	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/evidence"
	"github.com/mistakeknot/Skaffen/internal/mcp"
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/router"
	"github.com/mistakeknot/Skaffen/internal/session"
	"github.com/mistakeknot/Skaffen/internal/tool"
	"github.com/mistakeknot/Skaffen/internal/trust"
	"github.com/mistakeknot/Skaffen/internal/tui"

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
	flagMode     = flag.String("mode", "tui", "Execution mode: tui (default), print")
	flagResume    = flag.Bool("c", false, "Resume last session")
	flagResumeID  = flag.String("r", "", "Resume specific session by ID")
	flagPlugins   = flag.String("plugins", "", "Path to plugins.toml (default: ~/.skaffen/plugins.toml)")
	flagColorMode = flag.String("color-mode", "", "Color mode: dark, light (default: auto-detect)")
	flagTheme     = flag.String("theme", "", "Theme: tokyonight, catppuccin (default: Tokyo Night)")
)

func main() {
	flag.Parse()

	// Theme & color mode — apply before any rendering
	setupTheme()

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

	switch *flagMode {
	case "tui":
		if err := runTUI(); err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: %v\n", err)
			os.Exit(1)
		}
	case "print":
		if err := runPrint(); err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "skaffen: unknown mode %q\n", *flagMode)
		os.Exit(1)
	}
}

func runPrint() error {
	// Context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ic := checkIntercore()

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

	// Load MCP plugins
	mcpMgr := loadMCPPlugins(ctx, reg)
	if mcpMgr != nil {
		defer mcpMgr.Shutdown()
	}

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

	// Session ID — used for both session persistence and evidence
	sessionID := *flagSession
	if sessionID == "" {
		sessionID = fmt.Sprintf("skaffen-%d", os.Getpid())
	}

	modelRouter := router.NewWithIC(routerCfg, ic, sessionID)

	// Configure agent
	opts := []agent.Option{
		agent.WithMaxTurns(*flagMaxTurns),
		agent.WithStartPhase(phase),
		agent.WithRouter(modelRouter),
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

func runTUI() error {
	ic := checkIntercore()

	// Resolve provider (same logic as runPrint)
	providerName := *flagProvider
	if providerName == "" {
		if os.Getenv("ANTHROPIC_API_KEY") != "" {
			providerName = "anthropic"
		} else {
			providerName = "claude-code"
		}
	}

	cfg := provider.ProviderConfig{}
	if providerName == "anthropic" {
		cfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		if cfg.APIKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY not set")
		}
	}

	p, err := provider.New(providerName, cfg)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}

	// Tool registry
	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	// Load MCP plugins (use timeout context since TUI manages its own context)
	mcpCtx, mcpCancel := context.WithTimeout(context.Background(), 30*time.Second)
	mcpMgr := loadMCPPlugins(mcpCtx, reg)
	mcpCancel()
	if mcpMgr != nil {
		defer mcpMgr.Shutdown()
	}

	// Router
	routingPath := filepath.Join(os.Getenv("HOME"), ".skaffen", "routing.json")
	routerCfg, err := router.LoadConfig(routingPath)
	if err != nil {
		routerCfg = &router.Config{}
	}
	if *flagBudget > 0 {
		routerCfg.Budget = &router.BudgetConfig{
			MaxTokens: *flagBudget,
			Mode:      "graceful",
			DegradeAt: 0.8,
		}
	}
	if *flagModel != "" {
		if routerCfg.Phases == nil {
			routerCfg.Phases = make(map[tool.Phase]string)
		}
		for _, ph := range []tool.Phase{tool.PhaseBrainstorm, tool.PhasePlan, tool.PhaseBuild, tool.PhaseReview, tool.PhaseShip} {
			routerCfg.Phases[ph] = *flagModel
		}
	}
	// Session
	sessionID := *flagSession
	if sessionID == "" {
		sessionID = fmt.Sprintf("skaffen-%d", os.Getpid())
	}

	modelRouter := router.NewWithIC(routerCfg, ic, sessionID)

	phase := tool.Phase(*flagPhase)

	opts := []agent.Option{
		agent.WithMaxTurns(*flagMaxTurns),
		agent.WithStartPhase(phase),
		agent.WithRouter(modelRouter),
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

	// Evidence
	evidenceDir := filepath.Join(os.Getenv("HOME"), ".skaffen", "evidence")
	emitter := evidence.New(evidenceDir, sessionID)
	opts = append(opts, agent.WithEmitter(emitter), agent.WithSessionID(sessionID))

	// Trust evaluator
	trustEval := trust.NewEvaluator(nil)

	// Create agent
	a := agent.New(p, reg, opts...)

	// Working directory
	workDir, _ := os.Getwd()

	// Run TUI
	return tui.Run(tui.Config{
		Agent:     a,
		Trust:     trustEval,
		SessionID: sessionID,
		Verbose:   false,
		WorkDir:   workDir,
	})
}

// checkIntercore detects the ic (Intercore CLI) for evidence and routing integration.
// Returns nil client (not error) if ic is unavailable — Skaffen degrades gracefully.
func checkIntercore() *router.ICClient {
	ic, err := router.NewICClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "skaffen: warning: intercore unavailable: %v\n", err)
		return nil
	}
	if err := ic.Health(); err != nil {
		fmt.Fprintf(os.Stderr, "skaffen: warning: intercore unhealthy: %v\n", err)
		return nil
	}
	return ic
}

// loadMCPPlugins loads configured MCP plugins into the registry.
// Returns the manager (may be nil if no plugins configured) — caller must defer Shutdown().
func loadMCPPlugins(ctx context.Context, reg *tool.Registry) *mcp.Manager {
	pluginsPath := *flagPlugins
	if pluginsPath == "" {
		pluginsPath = filepath.Join(os.Getenv("HOME"), ".skaffen", "plugins.toml")
	}
	pluginsCfg, err := mcp.LoadConfig(pluginsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skaffen: warning: plugins config: %v\n", err)
		return nil
	}
	if len(pluginsCfg) == 0 {
		return nil
	}
	mgr := mcp.NewManager(pluginsCfg, reg)
	if err := mgr.LoadAll(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "skaffen: warning: MCP plugins: %v\n", err)
	}
	fmt.Fprintf(os.Stderr, "skaffen: loaded %d MCP plugin(s), %d tool(s)\n",
		mgr.PluginCount(), mgr.ToolCount())
	return mgr
}

// setupTheme configures the Masaq theme and color mode from CLI flags.
// Falls back to DetectMode() for color mode, TokyoNight for theme.
func setupTheme() {
	// Color mode: CLI flag > env var > terminal detection
	if *flagColorMode != "" {
		switch strings.ToLower(*flagColorMode) {
		case "light":
			theme.SetMode(theme.Light)
		default:
			theme.SetMode(theme.Dark)
		}
	} else {
		theme.SetMode(theme.DetectMode())
	}

	// Theme selection
	if *flagTheme != "" {
		if t, ok := theme.ThemeByName(*flagTheme); ok {
			theme.SetCurrent(t)
		} else {
			fmt.Fprintf(os.Stderr, "skaffen: unknown theme %q, using default\n", *flagTheme)
		}
	}
}

func printVersion() {
	version := "dev"
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		version = info.Main.Version
	}
	fmt.Printf("skaffen %s (%s)\n", version, runtime.Version())
}
