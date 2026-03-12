package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/mistakeknot/Masaq/theme"
	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/command"
	"github.com/mistakeknot/Skaffen/internal/config"
	"github.com/mistakeknot/Skaffen/internal/hooks"
	"github.com/mistakeknot/Skaffen/internal/contextfiles"
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
	flagPlugins   = flag.String("plugins", "", "Path to plugins.toml (overrides config hierarchy)")
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

// initConfig loads and merges configuration from user-global and per-project
// directories, applies CLI flag overrides, and returns the resolved config,
// routing config, and plugin configs.
func initConfig() (*config.Config, *router.Config, map[string]mcp.PluginConfig, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("getwd: %w", err)
	}
	cfg, err := config.Load(workDir)
	if err != nil {
		return nil, nil, nil, err
	}

	// Load and merge routing configs (user-global base, per-project overlay)
	var routerCfg *router.Config
	routingPaths := cfg.RoutingPaths()
	if len(routingPaths) == 0 {
		routerCfg = &router.Config{Phases: make(map[tool.Phase]string)}
	} else {
		base, err := router.LoadConfig(routingPaths[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: warning: routing config: %v\n", err)
			routerCfg = &router.Config{Phases: make(map[tool.Phase]string)}
		} else {
			routerCfg = base
			if len(routingPaths) > 1 {
				project, err := router.LoadConfig(routingPaths[1])
				if err != nil {
					fmt.Fprintf(os.Stderr, "skaffen: warning: project routing config: %v\n", err)
				} else {
					routerCfg = router.MergeConfig(base, project)
				}
			}
		}
	}

	// CLI --budget flag overrides config file budget
	if *flagBudget > 0 {
		routerCfg.Budget = &router.BudgetConfig{
			MaxTokens: *flagBudget,
			Mode:      "graceful",
			DegradeAt: 0.8,
		}
	}

	// CLI --model flag: set as override for all phases
	if *flagModel != "" {
		if routerCfg.Phases == nil {
			routerCfg.Phases = make(map[tool.Phase]string)
		}
		for _, ph := range []tool.Phase{tool.PhaseBrainstorm, tool.PhasePlan, tool.PhaseBuild, tool.PhaseReview, tool.PhaseShip} {
			routerCfg.Phases[ph] = *flagModel
		}
	}

	// Load and merge plugin configs
	var pluginsCfg map[string]mcp.PluginConfig
	if *flagPlugins != "" {
		// CLI flag overrides entire config hierarchy
		pluginsCfg, err = mcp.LoadConfig(*flagPlugins)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: warning: plugins config: %v\n", err)
			pluginsCfg = make(map[string]mcp.PluginConfig)
		}
	} else {
		pluginPaths := cfg.PluginPaths()
		for _, p := range pluginPaths {
			pcfg, err := mcp.LoadConfig(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "skaffen: warning: plugins config %s: %v\n", p, err)
				continue
			}
			if pluginsCfg == nil {
				pluginsCfg = pcfg
			} else {
				pluginsCfg = mcp.MergePluginConfigs(pluginsCfg, pcfg)
			}
		}
		if pluginsCfg == nil {
			pluginsCfg = make(map[string]mcp.PluginConfig)
		}
	}

	return cfg, routerCfg, pluginsCfg, nil
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

	pcfg := provider.ProviderConfig{}
	if providerName == "anthropic" {
		pcfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		if pcfg.APIKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY not set (omit --provider to use Claude Max via claude-code)")
		}
	}

	p, err := provider.New(providerName, pcfg)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}

	// Initialize tool registry
	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	// Load config (user-global + per-project + CLI overrides)
	cfg, routerCfg, pluginsCfg, err := initConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// Load MCP plugins
	mcpMgr := loadMCPPluginsFromConfig(ctx, reg, pluginsCfg)
	if mcpMgr != nil {
		defer mcpMgr.Shutdown()
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

	// Build system prompt from context files + --system flag
	systemPrompt := buildSystemPrompt(cfg.WorkDir(), *flagSystem)

	if *flagSession != "" {
		sess := session.New(*flagSession, cfg.SessionDir(), systemPrompt, 20)
		if err := sess.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: warning: load session: %v\n", err)
		}
		opts = append(opts, agent.WithSession(sess))
	} else if systemPrompt != "" {
		opts = append(opts, agent.WithSession(&agent.NoOpSession{Prompt: systemPrompt}))
	}

	// Evidence emission — always enabled
	emitter := evidence.New(cfg.EvidenceDir(), sessionID)
	opts = append(opts, agent.WithEmitter(emitter), agent.WithSessionID(sessionID))

	// Lifecycle hooks — only gate in headless mode (no trust evaluator)
	if hookExec := loadHooks(cfg, sessionID, string(phase)); hookExec != nil {
		hookExec.SessionStart(ctx, "print")
		opts = append(opts, agent.WithHooks(hookExec))
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

	pcfg := provider.ProviderConfig{}
	if providerName == "anthropic" {
		pcfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		if pcfg.APIKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY not set")
		}
	}

	p, err := provider.New(providerName, pcfg)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}

	// Tool registry
	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	// Load config (user-global + per-project + CLI overrides)
	cfg, routerCfg, pluginsCfg, err := initConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// Load MCP plugins (use timeout context since TUI manages its own context)
	mcpCtx, mcpCancel := context.WithTimeout(context.Background(), 30*time.Second)
	mcpMgr := loadMCPPluginsFromConfig(mcpCtx, reg, pluginsCfg)
	mcpCancel()
	if mcpMgr != nil {
		defer mcpMgr.Shutdown()
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

	// Working directory + context files
	systemPrompt := buildSystemPrompt(cfg.WorkDir(), *flagSystem)

	var tuiSession *session.JSONLSession
	if *flagSession != "" {
		tuiSession = session.New(*flagSession, cfg.SessionDir(), systemPrompt, 20)
		if err := tuiSession.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: warning: load session: %v\n", err)
		}
		opts = append(opts, agent.WithSession(tuiSession))
	} else if systemPrompt != "" {
		opts = append(opts, agent.WithSession(&agent.NoOpSession{Prompt: systemPrompt}))
	}

	// Evidence
	emitter := evidence.New(cfg.EvidenceDir(), sessionID)
	opts = append(opts, agent.WithEmitter(emitter), agent.WithSessionID(sessionID))

	// Trust evaluator
	trustEval := trust.NewEvaluator(nil)

	// Lifecycle hooks — pre-filter before trust evaluator
	if hookExec := loadHooks(cfg, sessionID, string(phase)); hookExec != nil {
		hookExec.SessionStart(context.Background(), "tui")
		opts = append(opts, agent.WithHooks(hookExec))
	}

	// Load custom slash commands from disk
	customCmds := command.LoadAll(cfg.CommandDirs()...)

	// Create agent
	a := agent.New(p, reg, opts...)

	// Run TUI
	return tui.Run(tui.Config{
		Agent:          a,
		Trust:          trustEval,
		Session:        tuiSession,
		SessionID:      sessionID,
		Verbose:        false,
		WorkDir:        cfg.WorkDir(),
		SkaffenVer:     skaffenVersion(),
		MasaqVer:       masaqVersion(),
		CustomCommands: customCmds,
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

// loadMCPPluginsFromConfig loads pre-resolved plugin configs into the registry.
// Returns the manager (may be nil if no plugins configured) — caller must defer Shutdown().
func loadMCPPluginsFromConfig(ctx context.Context, reg *tool.Registry, pluginsCfg map[string]mcp.PluginConfig) *mcp.Manager {
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

// loadHooks loads and merges hook configs (user-global + per-project),
// returning an Executor ready for use. Returns nil if no hooks are configured.
func loadHooks(cfg *config.Config, sessionID, phase string) *hooks.Executor {
	hookPaths := cfg.HookPaths()
	if len(hookPaths) == 0 {
		return nil
	}

	base, err := hooks.LoadConfig(hookPaths[0])
	if err != nil {
		log.Printf("skaffen: warning: hook config: %v", err)
		return nil
	}
	merged := base
	if len(hookPaths) > 1 {
		project, err := hooks.LoadConfig(hookPaths[1])
		if err != nil {
			log.Printf("skaffen: warning: project hook config: %v", err)
		} else {
			merged = hooks.MergeConfig(base, project)
		}
	}

	workDir, _ := os.Getwd()
	return hooks.NewExecutor(merged, sessionID, workDir, phase)
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

// Version variables — set via -ldflags or default to compiled-in values.
// Example: go build -ldflags="-X main.version=1.2.3" ./cmd/skaffen
var (
	version       = "0.1.0"
	masaqVersion_ = "0.1.0"
)

func skaffenVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "dev"
}

func masaqVersion() string {
	if masaqVersion_ != "" {
		return masaqVersion_
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if strings.HasSuffix(dep.Path, "Masaq") {
				if dep.Version != "" {
					return dep.Version
				}
			}
		}
	}
	return "dev"
}

// buildSystemPrompt assembles the system prompt from project context files
// (CLAUDE.md, AGENTS.md) found in the directory hierarchy, plus any explicit
// --system flag. Context files are loaded outermost-first (home → project).
func buildSystemPrompt(workDir, explicit string) string {
	ctx := contextfiles.Load(workDir)
	switch {
	case ctx != "" && explicit != "":
		return ctx + "\n\n---\n\n" + explicit
	case ctx != "":
		return ctx
	default:
		return explicit
	}
}

func printVersion() {
	fmt.Printf("skaffen %s (%s)\n", skaffenVersion(), runtime.Version())
}
