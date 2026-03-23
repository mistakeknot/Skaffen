package tmuxagent

import (
	"github.com/mistakeknot/Zaka/pkg/adapter"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

func init() {
	// Register provider constructors for each adapter.
	// Adapter registrations happen in Zaka's init() functions
	// (claude.go, codex.go, generic adapters are registered there).
	for _, name := range adapter.List() {
		adapterName := name // capture loop var
		provider.Register("tmux-"+adapterName, func(cfg provider.ProviderConfig) (provider.Provider, error) {
			a := adapter.Get(adapterName)
			return New(
				WithAdapter(a),
				WithWorkDir(cfg.WorkDir),
			)
		})
	}
}
