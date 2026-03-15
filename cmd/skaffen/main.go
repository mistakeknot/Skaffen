package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
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
	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/command"
	"github.com/mistakeknot/Skaffen/internal/config"
	"github.com/mistakeknot/Skaffen/internal/hooks"
	"github.com/mistakeknot/Skaffen/internal/contextfiles"
	"github.com/mistakeknot/Skaffen/internal/evidence"
	"github.com/mistakeknot/Skaffen/internal/mcp"
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/router"
	"github.com/mistakeknot/Skaffen/internal/sandbox"
	"github.com/mistakeknot/Skaffen/internal/session"
	"github.com/mistakeknot/Skaffen/internal/skill"
	"github.com/mistakeknot/Skaffen/internal/subagent"
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
	flagPhase    = flag.String("phase", "act", "OODARC phase (orient, decide, act, reflect, compound)")
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
	flagPlanMode  = flag.Bool("plan-mode", false, "Start in read-only plan mode")
	flagYolo      = flag.Bool("yolo", false, "Disable all sandbox enforcement (alias for --dangerously-disable-sandbox)")
	flagNoSandbox = flag.Bool("dangerously-disable-sandbox", false, "Disable all sandbox enforcement")
	flagSandboxMode = flag.String("sandbox", "default", "Sandbox mode: default, strict")
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
		for _, ph := range []tool.Phase{tool.PhaseOrient, tool.PhaseDecide, tool.PhaseAct, tool.PhaseReflect, tool.PhaseCompound} {
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
	case tool.PhaseOrient, tool.PhaseDecide, tool.PhaseAct, tool.PhaseReflect, tool.PhaseCompound:
		// valid
	default:
		return fmt.Errorf("invalid phase %q (must be orient, decide, act, reflect, or compound)", *flagPhase)
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

	// Initialize sandbox
	sb := initSandbox(cfg.WorkDir())
	reg.SetSandbox(sb)

	// Inject sandbox into BashTool
	if bt, ok := reg.Get("bash"); ok {
		if bashTool, ok := bt.(*tool.BashTool); ok {
			bashTool.Sandbox = sb
		}
	}

	// Load MCP plugins
	mcpMgr := loadMCPPluginsFromConfig(ctx, reg, pluginsCfg, sb)
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

	// Plan mode
	if *flagPlanMode {
		a.SetPlanMode(true)
		fmt.Fprintln(os.Stderr, "skaffen: plan mode (read-only)")
	}

	// Expand @mentions: extract images first, then text files
	workDir := cfg.WorkDir()
	displayText, imageBlocks := tui.ExpandImageMentions(prompt, workDir)
	expandedPrompt := tui.ExpandAtMentions(displayText, workDir)

	// Run agent loop
	var result *agent.RunResult
	if len(imageBlocks) > 0 {
		result, err = a.RunWithImages(ctx, expandedPrompt, imageBlocks)
	} else {
		result, err = a.Run(ctx, expandedPrompt)
	}
	if err != nil {
		return err
	}

	// Print response to stdout
	fmt.Print(result.Response)

	// Print usage to stderr (include cache tokens if nonzero)
	if result.Usage.CacheReadInputTokens > 0 || result.Usage.CacheCreationInputTokens > 0 {
		fmt.Fprintf(os.Stderr, "\n[%d turns, %d in / %d out tokens, %d cache_read / %d cache_create]\n",
			result.Turns, result.Usage.InputTokens, result.Usage.OutputTokens,
			result.Usage.CacheReadInputTokens, result.Usage.CacheCreationInputTokens)
	} else {
		fmt.Fprintf(os.Stderr, "\n[%d turns, %d in / %d out tokens]\n",
			result.Turns, result.Usage.InputTokens, result.Usage.OutputTokens)
	}

	// Report tokens to intercore (best-effort, fire-and-forget)
	if ic != nil {
		ic.ReportTokens(router.TokenReport{
			SessionID:           sessionID,
			InputTokens:         result.Usage.InputTokens,
			OutputTokens:        result.Usage.OutputTokens,
			CacheCreationTokens: result.Usage.CacheCreationInputTokens,
			CacheReadTokens:     result.Usage.CacheReadInputTokens,
		})
	}

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

	wd, _ := os.Getwd()
	pcfg := provider.ProviderConfig{WorkDir: wd}
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

	// Initialize sandbox (print mode)
	sb := initSandbox(cfg.WorkDir())
	reg.SetSandbox(sb)
	if bt, ok := reg.Get("bash"); ok {
		if bashTool, ok := bt.(*tool.BashTool); ok {
			bashTool.Sandbox = sb
		}
	}

	// Load MCP plugins (use timeout context since TUI manages its own context)
	mcpCtx, mcpCancel := context.WithTimeout(context.Background(), 30*time.Second)
	mcpMgr := loadMCPPluginsFromConfig(mcpCtx, reg, pluginsCfg, sb)
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

	// Load skills from SKILL.md files
	skills := skill.LoadAll(cfg.SkillDirs()...)

	// Subagent system: registry + tool + runner (runner wired after TUI starts)
	subReg := subagent.NewTypeRegistry(filepath.Join(cfg.WorkDir(), ".skaffen", "agents"))
	agentTool := subagent.NewAgentTool(subReg, nil) // runner set lazily
	reg.RegisterForPhases(&agentloopToolBridge{inner: agentTool}, []tool.Phase{tool.PhaseAct})

	// Create agent
	a := agent.New(p, reg, opts...)

	// Plan mode
	if *flagPlanMode {
		a.SetPlanMode(true)
	}

	// Run TUI — pass subagent wiring info so it can create the runner
	// and connect StatusCB to program.Send.
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
		Skills:         skills,
		HistoryPath:    cfg.HistoryPath(),
		SandboxLabel:     sandboxLabel(sb),
		KeybindingsPaths: cfg.KeybindingsPaths(),
		SubagentInit: &tui.SubagentInit{
			AgentTool:   agentTool,
			Registry:    subReg,
			Provider:    p,
			Reservation: subagent.NewReservationBridge(cfg.WorkDir()),
		},
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
func loadMCPPluginsFromConfig(ctx context.Context, reg *tool.Registry, pluginsCfg map[string]mcp.PluginConfig, sb *sandbox.Sandbox) *mcp.Manager {
	if len(pluginsCfg) == 0 {
		return nil
	}
	mgr := mcp.NewManager(pluginsCfg, reg, sb)
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

// initSandbox creates the sandbox based on CLI flags and config.
func initSandbox(workDir string) *sandbox.Sandbox {
	if *flagYolo || *flagNoSandbox {
		fmt.Fprintln(os.Stderr, "skaffen: WARNING: sandbox disabled (--yolo)")
		return sandbox.New(sandbox.DisabledPolicy(), sandbox.ModeDisabled)
	}
	if *flagSandboxMode == "strict" {
		return sandbox.New(sandbox.StrictPolicy(workDir), sandbox.ModeStrict)
	}

	policy, err := sandbox.Load(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skaffen: warning: sandbox config error: %v (using defaults)\n", err)
		policy = sandbox.DefaultPolicy(workDir)
	}
	return sandbox.New(policy, sandbox.ModeDefault)
}

// sandboxLabel returns a human-readable label for the TUI status bar.
func sandboxLabel(sb *sandbox.Sandbox) string {
	if sb == nil {
		return ""
	}
	switch sb.Mode() {
	case sandbox.ModeDisabled:
		return "YOLO"
	case sandbox.ModeStrict:
		return "strict"
	default:
		return "sandbox"
	}
}

func printVersion() {
	fmt.Printf("skaffen %s (%s)\n", skaffenVersion(), runtime.Version())
}

// agentloopToolBridge adapts an agentloop.Tool to tool.Tool so it can be
// registered in the phase-gated tool.Registry. The reverse of agent.toolBridge.
type agentloopToolBridge struct {
	inner agentloop.Tool
}

func (b *agentloopToolBridge) Name() string                    { return b.inner.Name() }
func (b *agentloopToolBridge) Description() string             { return b.inner.Description() }
func (b *agentloopToolBridge) Schema() json.RawMessage         { return b.inner.Schema() }
func (b *agentloopToolBridge) Execute(ctx context.Context, params json.RawMessage) tool.ToolResult {
	r := b.inner.Execute(ctx, params)
	return tool.ToolResult{Content: r.Content, IsError: r.IsError}
}
