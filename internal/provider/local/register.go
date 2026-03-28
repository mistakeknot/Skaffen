package local

import (
	"os"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

func init() {
	provider.Register("local", func(cfg provider.ProviderConfig) (provider.Provider, error) {
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = os.Getenv("INTERFERE_URL")
		}
		if baseURL == "" {
			baseURL = "http://localhost:8421"
		}

		var opts []Option
		opts = append(opts, WithBaseURL(baseURL))
		if cfg.Model != "" {
			opts = append(opts, WithModel(cfg.Model))
		}

		return New(opts...), nil
	})
}
