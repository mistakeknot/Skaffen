package experiment

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCampaign(t *testing.T) {
	c, err := LoadCampaign("testdata/routing-opt.yaml")
	if err != nil {
		t.Fatalf("LoadCampaign: %v", err)
	}

	if c.Name != "routing-opt" {
		t.Errorf("Name = %q, want %q", c.Name, "routing-opt")
	}
	if c.Metric.Direction != Minimize {
		t.Errorf("Direction = %q, want %q", c.Metric.Direction, Minimize)
	}
	if c.Metric.Baseline != 0.42 {
		t.Errorf("Baseline = %v, want 0.42", c.Metric.Baseline)
	}
	if len(c.SecondaryMetrics) != 2 {
		t.Errorf("SecondaryMetrics = %d, want 2", len(c.SecondaryMetrics))
	}
	if len(c.Ideas) != 3 {
		t.Errorf("Ideas = %d, want 3", len(c.Ideas))
	}
	if c.Benchmark.Timeout.Seconds() != 120 {
		t.Errorf("Timeout = %v, want 120s", c.Benchmark.Timeout)
	}
}

func TestLoadCampaign_Defaults(t *testing.T) {
	yaml := `
name: minimal
metric:
  name: speed
  direction: maximize
  baseline: 1.0
benchmark:
  command: "echo test"
  metric_pattern: "speed=([0-9.]+)"
`
	path := writeTemp(t, "minimal.yaml", yaml)
	c, err := LoadCampaign(path)
	if err != nil {
		t.Fatalf("LoadCampaign: %v", err)
	}

	if c.Benchmark.Timeout.Seconds() != 120 {
		t.Errorf("default Timeout = %v, want 120s", c.Benchmark.Timeout)
	}
	if c.Budget.MaxExperimentsOrDefault() != 50 {
		t.Errorf("default MaxExperiments = %d, want 50", c.Budget.MaxExperimentsOrDefault())
	}
	if c.Budget.MaxConsecutiveFailuresOrDefault() != 5 {
		t.Errorf("default MaxConsecutiveFailures = %d, want 5", c.Budget.MaxConsecutiveFailuresOrDefault())
	}
	if !c.Git.UseWorktree() {
		t.Error("default UseWorktree should be true")
	}
}

func TestLoadCampaign_MissingName(t *testing.T) {
	yaml := `
metric:
  name: speed
  direction: maximize
  baseline: 1.0
benchmark:
  command: "echo test"
  metric_pattern: "speed=([0-9.]+)"
`
	path := writeTemp(t, "no-name.yaml", yaml)
	_, err := LoadCampaign(path)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoadCampaign_InvalidDirection(t *testing.T) {
	yaml := `
name: bad-dir
metric:
  name: speed
  direction: sideways
  baseline: 1.0
benchmark:
  command: "echo test"
  metric_pattern: "speed=([0-9.]+)"
`
	path := writeTemp(t, "bad-dir.yaml", yaml)
	_, err := LoadCampaign(path)
	if err == nil {
		t.Fatal("expected error for invalid direction")
	}
}

func TestLoadCampaign_TooManySecondary(t *testing.T) {
	yaml := `
name: too-many
metric:
  name: speed
  direction: maximize
  baseline: 1.0
secondary_metrics:
  - {name: a, direction: minimize, baseline: 1.0}
  - {name: b, direction: minimize, baseline: 1.0}
  - {name: c, direction: minimize, baseline: 1.0}
  - {name: d, direction: minimize, baseline: 1.0}
benchmark:
  command: "echo test"
  metric_pattern: "speed=([0-9.]+)"
`
	path := writeTemp(t, "too-many.yaml", yaml)
	_, err := LoadCampaign(path)
	if err == nil {
		t.Fatal("expected error for too many secondary metrics")
	}
}

func TestLoadCampaign_ZeroBaseline(t *testing.T) {
	yaml := `
name: zero-base
metric:
  name: speed
  direction: maximize
  baseline: 0
benchmark:
  command: "echo test"
  metric_pattern: "speed=([0-9.]+)"
`
	path := writeTemp(t, "zero-base.yaml", yaml)
	_, err := LoadCampaign(path)
	if err == nil {
		t.Fatal("expected error for zero baseline")
	}
}

func TestFindCampaign_LocalPath(t *testing.T) {
	dir := t.TempDir()

	// Create local campaign dir
	campaignDir := filepath.Join(dir, ".skaffen", "campaigns")
	if err := os.MkdirAll(campaignDir, 0700); err != nil {
		t.Fatal(err)
	}

	yaml := `
name: local-test
metric:
  name: speed
  direction: maximize
  baseline: 1.0
benchmark:
  command: "echo test"
  metric_pattern: "speed=([0-9.]+)"
`
	if err := os.WriteFile(filepath.Join(campaignDir, "local-test.yaml"), []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	// Change to temp dir so FindCampaign finds the local path
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	c, err := FindCampaign("local-test")
	if err != nil {
		t.Fatalf("FindCampaign: %v", err)
	}
	if c.Name != "local-test" {
		t.Errorf("Name = %q, want %q", c.Name, "local-test")
	}
}

func TestFindCampaign_NotFound(t *testing.T) {
	_, err := FindCampaign("nonexistent-campaign-xyz")
	if err == nil {
		t.Fatal("expected error for campaign not found")
	}
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
