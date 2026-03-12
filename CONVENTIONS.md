# Skaffen Conventions

## Naming

- Package names: lowercase, single word (`agent`, `tool`, `mcp`)
- Interface names: noun (`Provider`, `Router`, `Session`, `Emitter`, `Tool`)
- Concrete types: descriptive noun (`Client`, `Manager`, `Registry`, `Evaluator`)
- Test helpers: `stub` prefix for mocks (`stubTool`, `stubProvider`)
- MCP tool names: `plugin_server_tool` format to prevent collisions

## File Layout

- One primary type per file (e.g., `registry.go` → `Registry`)
- Test file mirrors source: `registry_test.go`
- Adapters and bridges in the file that uses them (e.g., adapters in `agent.go`)
- Package doc in `doc.go`

## Error Handling

- Return `error` up the stack. Don't swallow errors silently.
- Wrap with context: `fmt.Errorf("mcp connect: %w", err)`
- Optional dependencies: warn to stderr, continue without error
- Tool execution errors: return `ToolResult{IsError: true}`, not Go errors

## Testing

- Every package has tests. Target: comprehensive coverage of public API.
- Use table-driven tests for multiple cases.
- Mock external dependencies via interfaces, not build tags.
- No tests that require network access or external services.
- Test names: `TestType_Method` or `TestFunction` pattern.

## Dependencies

- Direct dependencies must be justified. Prefer stdlib when reasonable.
- `internal/` for all non-CLI packages — nothing is exported from the module.
- Provider packages register via `init()` and blank imports in `main.go`.

## Git

- Conventional commits: `feat(scope):`, `fix(scope):`, `docs:`, `refactor:`
- Scope is the package name: `feat(mcp):`, `fix(router):`, `refactor(agent):`
- Trunk-based: commit directly to `main`

## Configuration

- CLI flags for runtime config. No viper, no cobra — stdlib `flag` only.
- File-based config: TOML for plugins, JSON for routing.
- Environment variables: `ANTHROPIC_API_KEY`, `SKAFFEN_MODEL_<PHASE>`
- Config directory: `~/.skaffen/`
