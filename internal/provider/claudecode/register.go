package claudecode

import (
	"github.com/mistakeknot/Skaffen/internal/provider"
)

func init() {
	provider.Register("claude-code", func(cfg provider.ProviderConfig) (provider.Provider, error) {
		var opts []Option
		if cfg.Model != "" {
			opts = append(opts, WithModel(cfg.Model))
		}
		if cfg.WorkDir != "" {
			opts = append(opts, WithWorkDir(cfg.WorkDir))
		}
		return New(opts...), nil
	})
}
