package tmuxagent

import (
	"github.com/mistakeknot/Skaffen/internal/provider"
)

func init() {
	// Register well-known adapters from CASS connectors that have
	// dedicated adapter implementations.
	// Claude Code and Codex register themselves via their own init().
	// Additional agents use the generic adapter.
	RegisterAdapter(NewGenericAdapter("gemini", "gemini", "gemini"))
	RegisterAdapter(NewGenericAdapter("amp", "amp", "amp"))
	RegisterAdapter(NewGenericAdapter("aider", "aider", "aider"))
	RegisterAdapter(NewGenericAdapter("cline", "cline", "cline"))
	RegisterAdapter(NewGenericAdapter("cursor", "cursor", "cursor"))

	// Register provider constructors for each adapter.
	for _, name := range ListAdapters() {
		adapterName := name // capture loop var
		provider.Register("tmux-"+adapterName, func(cfg provider.ProviderConfig) (provider.Provider, error) {
			adapter := GetAdapter(adapterName)
			return New(
				WithAdapter(adapter),
				WithWorkDir(cfg.WorkDir),
			)
		})
	}
}
