package experiment

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Direction indicates whether the metric should be minimized or maximized.
type Direction string

const (
	Minimize Direction = "minimize"
	Maximize Direction = "maximize"
)

// Campaign defines an autoresearch optimization campaign loaded from YAML.
type Campaign struct {
	Name             string            `yaml:"name"`
	Metric           MetricConfig      `yaml:"metric"`
	SecondaryMetrics []SecondaryMetric  `yaml:"secondary_metrics"`
	Benchmark        BenchmarkConfig   `yaml:"benchmark"`
	Git              GitConfig         `yaml:"git"`
	Budget           BudgetConfig      `yaml:"budget"`
	Ideas            []string          `yaml:"ideas"`
}

// MetricConfig defines the primary metric to optimize.
type MetricConfig struct {
	Name      string    `yaml:"name"`
	Unit      string    `yaml:"unit"`
	Direction Direction `yaml:"direction"`
	Baseline  float64   `yaml:"baseline"`
}

// SecondaryMetric tracks an additional metric with a regression threshold.
type SecondaryMetric struct {
	Name                string    `yaml:"name"`
	Direction           Direction `yaml:"direction"`
	Baseline            float64   `yaml:"baseline"`
	RegressionThreshold float64   `yaml:"regression_threshold"`
}

// BenchmarkConfig defines how to run and measure experiments.
type BenchmarkConfig struct {
	Command           string            `yaml:"command"`
	MetricPattern     string            `yaml:"metric_pattern"`
	SecondaryPatterns map[string]string  `yaml:"secondary_patterns"`
	Timeout           time.Duration     `yaml:"timeout"`
	WorkingDir        string            `yaml:"working_dir"`
}

// GitConfig controls git worktree behavior.
type GitConfig struct {
	Worktree   *bool `yaml:"worktree"`
	AutoCommit *bool `yaml:"auto_commit"`
}

// UseWorktree returns whether worktree isolation is enabled (default: true).
func (g GitConfig) UseWorktree() bool {
	if g.Worktree == nil {
		return true
	}
	return *g.Worktree
}

// UseAutoCommit returns whether auto-commit on keep is enabled (default: true).
func (g GitConfig) UseAutoCommit() bool {
	if g.AutoCommit == nil {
		return true
	}
	return *g.AutoCommit
}

// BudgetConfig defines experiment budget limits.
type BudgetConfig struct {
	MaxExperiments          int `yaml:"max_experiments"`
	MaxConsecutiveFailures  int `yaml:"max_consecutive_failures"`
	TokenBudget             int `yaml:"token_budget"`
}

// MaxExperimentsOrDefault returns the max experiments limit (default: 50).
func (b BudgetConfig) MaxExperimentsOrDefault() int {
	if b.MaxExperiments <= 0 {
		return 50
	}
	return b.MaxExperiments
}

// MaxConsecutiveFailuresOrDefault returns the failure cap (default: 5).
func (b BudgetConfig) MaxConsecutiveFailuresOrDefault() int {
	if b.MaxConsecutiveFailures <= 0 {
		return 5
	}
	return b.MaxConsecutiveFailures
}

const maxSecondaryMetrics = 3

// LoadCampaign reads and validates a campaign YAML file.
func LoadCampaign(path string) (*Campaign, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load campaign: %w", err)
	}

	var c Campaign
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("load campaign: parse yaml: %w", err)
	}

	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("load campaign %q: %w", path, err)
	}

	// Apply defaults
	if c.Benchmark.Timeout == 0 {
		c.Benchmark.Timeout = 120 * time.Second
	}

	return &c, nil
}

// FindCampaign searches for a campaign YAML by name. Checks project-local
// .skaffen/campaigns/ first, then ~/.skaffen/campaigns/.
func FindCampaign(name string) (*Campaign, error) {
	// Project-local path
	localPath := filepath.Join(".skaffen", "campaigns", name+".yaml")
	if _, err := os.Stat(localPath); err == nil {
		return LoadCampaign(localPath)
	}

	// User home path
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find campaign: %w", err)
	}
	homePath := filepath.Join(home, ".skaffen", "campaigns", name+".yaml")
	if _, err := os.Stat(homePath); err == nil {
		return LoadCampaign(homePath)
	}

	return nil, fmt.Errorf("find campaign: %q not found in .skaffen/campaigns/ or ~/.skaffen/campaigns/", name)
}

func (c *Campaign) validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}

	if c.Metric.Name == "" {
		return fmt.Errorf("metric.name is required")
	}

	if c.Metric.Direction != Minimize && c.Metric.Direction != Maximize {
		return fmt.Errorf("metric.direction must be %q or %q, got %q", Minimize, Maximize, c.Metric.Direction)
	}

	if c.Metric.Baseline <= 0 {
		return fmt.Errorf("metric.baseline must be > 0, got %v", c.Metric.Baseline)
	}

	if len(c.SecondaryMetrics) > maxSecondaryMetrics {
		return fmt.Errorf("at most %d secondary metrics allowed, got %d", maxSecondaryMetrics, len(c.SecondaryMetrics))
	}

	for i, sm := range c.SecondaryMetrics {
		if sm.Name == "" {
			return fmt.Errorf("secondary_metrics[%d].name is required", i)
		}
		if sm.Direction != Minimize && sm.Direction != Maximize {
			return fmt.Errorf("secondary_metrics[%d].direction must be %q or %q", i, Minimize, Maximize)
		}
	}

	if c.Benchmark.Command == "" {
		return fmt.Errorf("benchmark.command is required")
	}

	if c.Benchmark.MetricPattern == "" {
		return fmt.Errorf("benchmark.metric_pattern is required")
	}

	return nil
}
