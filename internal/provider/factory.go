package provider

import "fmt"

// ProviderConfig holds provider initialization settings.
type ProviderConfig struct {
	APIKey  string // for anthropic provider
	Model   string // model override
	BaseURL string // API base URL override (for testing)
	WorkDir string // working directory for subprocess providers (claude-code)
}

// Constructor creates a Provider from config.
type Constructor func(cfg ProviderConfig) (Provider, error)

var registry = map[string]Constructor{}

// Register adds a provider constructor to the registry.
func Register(name string, ctor Constructor) {
	registry[name] = ctor
}

// New creates a provider by name using the global registry.
func New(name string, cfg ProviderConfig) (Provider, error) {
	ctor, ok := registry[name]
	if !ok {
		names := make([]string, 0, len(registry))
		for k := range registry {
			names = append(names, k)
		}
		return nil, fmt.Errorf("unknown provider %q, available: %v", name, names)
	}
	return ctor(cfg)
}

// Default returns the default provider name.
func Default() string { return "claude-code" }
