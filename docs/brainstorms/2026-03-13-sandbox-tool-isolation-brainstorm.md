---
artifact_type: brainstorm
bead: Demarch-6i0.10
stage: discover
---

# Sandbox / Tool Isolation

## What We're Building

OS-level sandboxing for Skaffen that restricts what the LLM's tool calls and MCP plugins can do on the user's system. Two isolation boundaries:

1. **LLM tool calls** — bash commands, file reads/writes, subagent execution
2. **MCP plugin subprocesses** — Interverse plugins running as separate processes

Cross-platform: Linux (bwrap) + macOS (Seatbelt). Symmetric security posture on both OSes.

## Threat Model

**Both attack surfaces equally:**
- Untrusted LLM output (model generates dangerous commands)
- Untrusted MCP plugins (third-party code with filesystem/network access)

Defense-in-depth: sandbox enforcement complements the existing trust system (policy layer) and phase gating (availability layer) with OS-level enforcement (kernel/process layer).

## Architecture

### Platform Abstraction

Single `SandboxPolicy` struct drives platform-specific backends:

```
SandboxPolicy {
    WriteDirs  []string   // read-write access
    ReadDirs   []string   // read-only access
    DenyDirs   []string   // always blocked (overrides read)
    AllowNet   []string   // allowed network domains
    DenyNet    bool       // block all network by default
}
```

**Linux backend:** bubblewrap (`bwrap`) — wraps each subprocess (bash, MCP) in namespace isolation (mount, PID, network). Requires `bwrap` binary (packaged in all major distros).

**macOS backend:** Seatbelt — generates `.sb` profile from SandboxPolicy, wraps each subprocess in `sandbox-exec`. Uses the OS-native sandbox framework.

**In-process tools (read, write, edit, grep, glob, ls):** Go-level path validation against SandboxPolicy before any file operation. Not kernel-enforced but covers the realistic threat (LLM requesting reads/writes outside allowed paths). Works identically on both OSes.

### Default Policy

```json
{
  "write": ["$WORKDIR", "/tmp/skaffen-*"],
  "read": ["/usr", "/bin", "/lib", "/etc", "$HOME"],
  "deny": [
    "~/.ssh", "~/.gnupg", "~/.aws", "~/.config/gh",
    "~/.*_history", "~/.env*", "~/.netrc"
  ],
  "network": {
    "deny_all": true,
    "allow": ["api.anthropic.com"]
  }
}
```

User overrides via `.skaffen/sandbox.json` (per-project) or `~/.skaffen/sandbox.json` (global).

### Sandbox Modes

Three modes, selected via CLI flag or config:

| Mode | Flag | Behavior |
|------|------|----------|
| **Default** | (none) | Project-scoped policy. Sandbox active. |
| **Strict** | `--sandbox=strict` | Minimal policy. Only workdir accessible. |
| **Yolo** | `--dangerously-disable-sandbox` or `--yolo` | All restrictions off. For trusted environments. |

### Execution Flow

```
User prompt
  → LLM generates tool call
  → Trust system evaluates (Allow/Prompt/Block)
  → Phase gating checks (tool available in current phase?)
  → Sandbox enforcement:
      In-process tool → Go path validation (CheckPath)
      Bash command    → bwrap/sandbox-exec wrapper
      MCP tool call   → MCP subprocess already sandboxed at spawn
  → Tool executes (or returns ErrSandboxDenied)
```

### Integration Points

1. **tool/registry.go** — inject sandbox check before `Execute()` for in-process tools
2. **tool/bash.go** — wrap `exec.CommandContext` with bwrap/sandbox-exec
3. **mcp/manager.go** — apply sandbox policy when spawning MCP server subprocesses
4. **subagent/runner.go** — subagents inherit parent sandbox (bash tools wrapped, in-process tools validated)
5. **config/config.go** — load `sandbox.json` from `.skaffen/` and `~/.skaffen/`
6. **cmd/skaffen/main.go** — `--yolo` / `--dangerously-disable-sandbox` / `--sandbox=strict` flags

### MCP Plugin Sandboxing

Each MCP plugin gets its own sandbox policy, derived from:
1. Default project policy (base)
2. Plugin-specific overrides in `plugins.toml` (if declared)
3. Plugin cannot escalate beyond project policy (intersection, not union)

## Why This Approach

1. **bwrap + Seatbelt** gives symmetric subprocess isolation on both platforms without inventing anything new — both are battle-tested (Flatpak, Claude Code, Codex).
2. **Go path validation** for in-process tools covers the gap that subprocess wrappers can't reach (read/write/edit/grep/glob/ls are pure Go, not subprocesses).
3. **Default-on with yolo escape hatch** balances security with developer friction — sandbox is always active unless explicitly disabled.
4. **Config-driven policy** means enterprises can lock down the sandbox while hobbyists can `--yolo`.

## Key Decisions

- **bwrap over Landlock for Linux subprocess isolation** — bwrap provides PID + mount namespace isolation (stronger) and works on older kernels (3.x+ vs 5.13+). Hard dependency on bwrap binary is acceptable.
- **Go path validation over kernel enforcement for in-process tools** — keeps the implementation symmetric across OSes. Kernel enforcement (Landlock) could be added later as a hardening layer without changing the architecture.
- **Deny-list for sensitive dirs** rather than allowlist-only — `~/.ssh`, `~/.aws`, etc. are always blocked even though `$HOME` is readable. This catches the highest-value exfiltration targets.
- **Network deny-all by default** — only `api.anthropic.com` allowed. Users add domains as needed in sandbox.json. Prevents data exfiltration via curl/wget even if bash is available.
- **Yolo mode** (`--dangerously-disable-sandbox`) — for trusted environments, CI, or when sandbox friction blocks legitimate workflows.

## Open Questions

- **Seatbelt profile complexity:** macOS Seatbelt profiles are notoriously finicky. Need to test with real workloads (git, npm, python, cargo) to find what breaks. Claude Code had bugs here (dotfile access, Library access).
- **bwrap availability:** Should Skaffen auto-detect bwrap and fall back to unsandboxed + warning? Or hard-fail if bwrap is missing?
- **Network granularity:** bwrap does all-or-nothing network isolation (unshare-net). For domain-level filtering, we'd need a network proxy or iptables rules. Is all-or-nothing sufficient for v1?
- **MCP plugin policy format:** How do plugins declare their required filesystem/network access? Frontmatter in plugin.toml? Separate manifest?
- **Windows:** Not a target now, but if it becomes one, Windows has no bwrap equivalent. Would need a different approach (maybe AppContainer or Job Objects).

## Prior Art

- [Anthropic sandbox-runtime](https://github.com/anthropic-experimental/sandbox-runtime) — TypeScript, bwrap + Seatbelt. Architecture reference.
- [Landrun](https://github.com/Zouuup/landrun) — Go CLI for Landlock sandboxing. Could be used for future Landlock hardening layer.
- [go-landlock](https://github.com/landlock-lsm/go-landlock) — Official Go library for Landlock LSM. Self-sandboxing API.
- [Pierce Freeman: Agent Sandbox Deep Dive](https://pierce.dev/notes/a-deep-dive-on-agent-sandboxes) — Comparative analysis of agent sandboxing approaches.
