package main

import (
	"fmt"
	"os"

	"github.com/mistakeknot/Skaffen/internal/costrouter"
	"github.com/mistakeknot/Skaffen/internal/provider"
	oai "github.com/mistakeknot/Skaffen/internal/provider/openai"
	"gopkg.in/yaml.v3"
)

// HassConfig is the top-level YAML configuration.
type HassConfig struct {
	CostRouter costrouter.Config  `yaml:"cost_router"`
	Providers  map[string]ProvCfg `yaml:"providers"`
	Tools      ToolsConfig        `yaml:"tools"`
}

// ProvCfg describes a provider backend.
type ProvCfg struct {
	BaseURL     string `yaml:"base_url"`
	APIKeyEnv   string `yaml:"api_key_env"`
	ModelPrefix string `yaml:"model_prefix"`
}

// ToolsConfig controls which tools are available.
type ToolsConfig struct {
	Allowed        []string `yaml:"allowed"`
	AutoApprove    []string `yaml:"auto_approve"`
	RequireApprove []string `yaml:"require_approval"`
}

// loadConfig reads a YAML config file.
func loadConfig(path string) (*HassConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg HassConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// defaultConfig returns a sensible default configuration.
func defaultConfig() *HassConfig {
	return &HassConfig{
		CostRouter: costrouter.Config{
			DefaultModel:    "qwen-plus-latest",
			EscalationModel: "claude-sonnet-4-6",
			PlanningModel:   "claude-opus-4-6",
			ReadModel:       "glm-4-plus",
		},
		Providers: map[string]ProvCfg{
			"glm": {
				BaseURL:     "https://open.bigmodel.cn/api/paas/v4",
				APIKeyEnv:   "GLM_API_KEY",
				ModelPrefix: "glm-",
			},
			"qwen": {
				BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
				APIKeyEnv:   "DASHSCOPE_API_KEY",
				ModelPrefix: "qwen-",
			},
			"anthropic": {
				APIKeyEnv:   "ANTHROPIC_API_KEY",
				ModelPrefix: "claude-",
			},
		},
		Tools: ToolsConfig{
			Allowed:        []string{"read", "edit", "grep", "glob", "ls", "bash"},
			AutoApprove:    []string{"read", "grep", "glob", "ls"},
			RequireApprove: []string{"edit", "write", "bash"},
		},
	}
}

// buildBackends creates provider instances from config, validating API keys.
func buildBackends(cfg *HassConfig) ([]costrouter.Backend, error) {
	var backends []costrouter.Backend

	for name, pcfg := range cfg.Providers {
		apiKey := os.Getenv(pcfg.APIKeyEnv)
		if apiKey == "" {
			return nil, fmt.Errorf("provider %q: env var %s is not set", name, pcfg.APIKeyEnv)
		}

		var p provider.Provider
		if name == "anthropic" {
			// Use the registered anthropic provider.
			var err error
			p, err = provider.New("anthropic", provider.ProviderConfig{APIKey: apiKey})
			if err != nil {
				return nil, fmt.Errorf("create anthropic provider: %w", err)
			}
		} else {
			// OpenAI-compatible providers (GLM, Qwen, etc.)
			var opts []oai.Option
			if pcfg.BaseURL != "" {
				opts = append(opts, oai.WithBaseURL(pcfg.BaseURL))
			}
			opts = append(opts, oai.WithName(name))
			p = oai.New(apiKey, opts...)
		}

		backends = append(backends, costrouter.Backend{
			Prefix:   pcfg.ModelPrefix,
			Provider: p,
		})
	}

	return backends, nil
}
