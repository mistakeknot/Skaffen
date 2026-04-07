package openai

import (
	"os"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

func init() {
	provider.Register("openai", func(cfg provider.ProviderConfig) (provider.Provider, error) {
		apiKey := cfg.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}

		var opts []Option
		if cfg.BaseURL != "" {
			opts = append(opts, WithBaseURL(cfg.BaseURL))
		}
		if cfg.Model != "" {
			opts = append(opts, WithModel(cfg.Model))
		}

		return New(apiKey, opts...), nil
	})
}
