package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mistakeknot/Skaffen/internal/sandbox"
)

const maxBenchmarkOutput = 2000

// RunExperimentTool executes the campaign benchmark and extracts metrics.
type RunExperimentTool struct {
	store   ExperimentStore
	finder  CampaignFinder
	wt      Worktree
	sandbox *sandbox.Sandbox
}

// NewRunExperimentTool creates a RunExperimentTool.
// Sandbox is required for benchmark command wrapping — pass nil only in tests.
func NewRunExperimentTool(store ExperimentStore, finder CampaignFinder, wt Worktree, sb *sandbox.Sandbox) *RunExperimentTool {
	return &RunExperimentTool{
		store:   store,
		finder:  finder,
		wt:      wt,
		sandbox: sb,
	}
}

func (t *RunExperimentTool) Name() string { return "run_experiment" }

func (t *RunExperimentTool) Description() string {
	return "Run the campaign benchmark and extract metrics. Returns primary and secondary metric values."
}

func (t *RunExperimentTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"campaign": {
				"type": "string",
				"description": "Campaign name"
			}
		},
		"required": ["campaign"]
	}`)
}

type runParams struct {
	Campaign string `json:"campaign"`
}

type runResult struct {
	PrimaryMetric   float64            `json:"primary_metric"`
	SecondaryValues map[string]float64 `json:"secondary_values,omitempty"`
	DurationMs      int64              `json:"duration_ms"`
	Output          string             `json:"output"`
}

func (t *RunExperimentTool) Execute(ctx context.Context, params json.RawMessage) ToolResult {
	var p runParams
	if err := json.Unmarshal(params, &p); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid params: %v", err), IsError: true}
	}

	campaign, err := t.finder.FindCampaign(p.Campaign)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("campaign not found: %v", err), IsError: true}
	}

	// Apply timeout from campaign config
	timeout := campaign.Benchmark.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build command with sandbox wrapping (same pattern as BashTool)
	cmdName, cmdArgs := "bash", []string{"-c", campaign.Benchmark.Command}
	if t.sandbox != nil {
		cmdName, cmdArgs = t.sandbox.WrapArgs(cmdName, cmdArgs...)
	}

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)

	// Run in worktree directory
	wtDir := t.wt.WorktreeDir(p.Campaign)
	if campaign.Benchmark.WorkingDir != "" {
		cmd.Dir = campaign.Benchmark.WorkingDir
	} else {
		cmd.Dir = wtDir
	}

	start := time.Now()
	out, err := cmd.CombinedOutput()
	durationMs := time.Since(start).Milliseconds()

	output := string(out)

	// Handle timeout
	if ctx.Err() == context.DeadlineExceeded {
		truncated := truncateOutput(output, maxBenchmarkOutput)
		return ToolResult{
			Content: fmt.Sprintf("benchmark timed out after %v\n\nOutput:\n%s", timeout, truncated),
			IsError: true,
		}
	}

	// Command failure — return stderr for debugging
	if err != nil {
		truncated := truncateOutput(output, maxBenchmarkOutput)
		return ToolResult{
			Content: fmt.Sprintf("benchmark failed: %v\n\nOutput:\n%s", err, truncated),
			IsError: true,
		}
	}

	// Extract primary metric
	primaryValue, err := extractMetric(output, campaign.Benchmark.MetricPattern)
	if err != nil {
		truncated := truncateOutput(output, maxBenchmarkOutput)
		return ToolResult{
			Content: fmt.Sprintf("metric extraction failed for pattern %q: %v\n\nFull output:\n%s",
				campaign.Benchmark.MetricPattern, err, truncated),
			IsError: true,
		}
	}

	// Extract secondary metrics
	secondaryValues := make(map[string]float64)
	for name, pattern := range campaign.Benchmark.SecondaryPatterns {
		val, err := extractMetric(output, pattern)
		if err != nil {
			// Secondary metric extraction failure is non-fatal — log and continue
			secondaryValues[name] = 0
			continue
		}
		secondaryValues[name] = val
	}

	result := runResult{
		PrimaryMetric:   primaryValue,
		SecondaryValues: secondaryValues,
		DurationMs:      durationMs,
		Output:          truncateOutput(output, maxBenchmarkOutput),
	}

	data, _ := json.Marshal(result)

	summary := fmt.Sprintf("Benchmark completed in %dms.\nPrimary metric (%s): %.4f",
		durationMs, campaign.Metric.Name, primaryValue)
	for name, val := range secondaryValues {
		summary += fmt.Sprintf("\nSecondary metric (%s): %.4f", name, val)
	}

	return ToolResult{Content: summary + "\n\n```json\n" + string(data) + "\n```"}
}

// extractMetric applies a regex pattern with one capture group to extract a float.
func extractMetric(output, pattern string) (float64, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0, fmt.Errorf("compile pattern: %w", err)
	}

	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return 0, fmt.Errorf("pattern %q did not match in output", pattern)
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(matches[1]), 64)
	if err != nil {
		return 0, fmt.Errorf("parse metric value %q: %w", matches[1], err)
	}

	return val, nil
}

// truncateOutput truncates output to maxLen bytes.
func truncateOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + fmt.Sprintf("\n... (truncated, %d bytes total)", len(output))
}
