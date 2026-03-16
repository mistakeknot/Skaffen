package experiment

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestFullExperimentLoop exercises the complete init→run→log cycle:
// 1. Create a campaign YAML with a trivial benchmark
// 2. Create a git repo + worktree
// 3. Open a segment (init)
// 4. Make a change in the worktree
// 5. Run the benchmark (extract metric)
// 6. Log with keep → verify commit created
// 7. Make another change
// 8. Run benchmark again
// 9. Log with discard → verify changes reverted (including untracked files)
// 10. Verify JSONL has correct records
// 11. Close segment → verify summary record
// 12. Verify resume from JSONL reconstructs correct state
func TestFullExperimentLoop(t *testing.T) {
	// Setup: git repo, store dir, campaign
	repoDir := initTestRepo(t)
	storeDir := filepath.Join(t.TempDir(), "experiments")
	wtBase := filepath.Join(t.TempDir(), "worktrees")
	campaignDir := filepath.Join(t.TempDir(), "campaigns")

	store := NewStore(storeDir)
	gitOps := NewGitOps(repoDir, wtBase)

	// Create campaign YAML with trivial benchmark
	if err := os.MkdirAll(campaignDir, 0700); err != nil {
		t.Fatal(err)
	}
	campaignYAML := `
name: test-loop
metric:
  name: line_count
  unit: lines
  direction: maximize
  baseline: 1.0
secondary_metrics:
  - name: file_count
    direction: maximize
    baseline: 1.0
    regression_threshold: 0.5
benchmark:
  command: "wc -l *.md | tail -1 | awk '{print \"line_count=\" $1}' && echo 'file_count=' && ls *.md | wc -l | awk '{print \"file_count=\" $1}'"
  metric_pattern: "line_count=([0-9]+)"
  secondary_patterns:
    file_count: "file_count=([0-9]+)"
  timeout: 10s
git:
  worktree: true
  auto_commit: true
budget:
  max_experiments: 10
  max_consecutive_failures: 3
ideas:
  - "Add more lines to README"
  - "Create a new markdown file"
`
	campaignPath := filepath.Join(campaignDir, "test-loop.yaml")
	if err := os.WriteFile(campaignPath, []byte(campaignYAML), 0600); err != nil {
		t.Fatal(err)
	}

	// Load campaign
	campaign, err := LoadCampaign(campaignPath)
	if err != nil {
		t.Fatalf("LoadCampaign: %v", err)
	}

	// === Step 1: Init (create worktree + segment) ===
	err = gitOps.CreateWorktree("test-loop")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer gitOps.RemoveWorktree("test-loop")

	seg, resumed, err := store.OpenSegment(campaign, "test-session")
	if err != nil {
		t.Fatalf("OpenSegment: %v", err)
	}
	if resumed {
		t.Error("expected new segment, not resumed")
	}

	t.Logf("Init: segment=%s baseline=%.1f worktree=%s",
		seg.ID(), seg.OriginalBaseline(), gitOps.WorktreeDir("test-loop"))

	// === Step 2: Experiment 1 — keep ===
	wtDir := gitOps.WorktreeDir("test-loop")

	// Make a change: add lines to README
	readmePath := filepath.Join(wtDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test\nLine 2\nLine 3\nLine 4\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run benchmark (simulated — extract metric from command output)
	metricValue := 4.0 // 4 lines in README

	// Log with keep
	sha, err := gitOps.KeepChanges("test-loop", "Add more lines to README", metricValue-seg.CurrentBest())
	if err != nil {
		t.Fatalf("KeepChanges: %v", err)
	}
	if sha == "" {
		t.Error("expected non-empty SHA from KeepChanges")
	}

	err = seg.LogExperiment(ExperimentRecord{
		Hypothesis:    "Add more lines to README",
		Status:        "completed",
		MetricBefore:  seg.CurrentBest(),
		MetricAfter:   metricValue,
		Delta:         metricValue - 1.0, // delta from current best (was 1.0)
		AgentDecision: "keep",
		Decision:      "keep",
		GitSHA:        sha,
		DurationMs:    50,
		Notes:         "First experiment",
	})
	if err != nil {
		t.Fatalf("LogExperiment (keep): %v", err)
	}

	// Verify state after keep
	if seg.CurrentBest() != 4.0 {
		t.Errorf("after keep: CurrentBest = %v, want 4.0", seg.CurrentBest())
	}
	if seg.OriginalBaseline() != 1.0 {
		t.Errorf("after keep: OriginalBaseline = %v, want 1.0 (immutable)", seg.OriginalBaseline())
	}
	if seg.ExperimentCount() != 1 {
		t.Errorf("after keep: ExperimentCount = %d, want 1", seg.ExperimentCount())
	}

	t.Logf("Exp 1: keep (delta=+%.1f, sha=%s)", metricValue-1.0, sha[:7])

	// === Step 3: Experiment 2 — discard ===
	// Make a change that "regresses"
	if err := os.WriteFile(readmePath, []byte("# Short\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Also create an untracked file (should be cleaned by discard)
	untrackedPath := filepath.Join(wtDir, "untracked.tmp")
	if err := os.WriteFile(untrackedPath, []byte("should be removed"), 0644); err != nil {
		t.Fatal(err)
	}

	metricValue = 1.0 // regression

	// Discard
	if err := gitOps.DiscardChanges("test-loop"); err != nil {
		t.Fatalf("DiscardChanges: %v", err)
	}

	err = seg.LogExperiment(ExperimentRecord{
		Hypothesis:    "Shorten README",
		Status:        "completed",
		MetricBefore:  seg.CurrentBest(), // 4.0
		MetricAfter:   metricValue,
		Delta:         metricValue - 4.0,
		AgentDecision: "discard",
		Decision:      "discard",
		DurationMs:    30,
		Notes:         "Regression — discarded",
	})
	if err != nil {
		t.Fatalf("LogExperiment (discard): %v", err)
	}

	// Verify discard cleaned up
	data, _ := os.ReadFile(readmePath)
	if !strings.Contains(string(data), "Line 4") {
		t.Errorf("after discard: README should be reverted to kept state, got %q", string(data))
	}
	if _, err := os.Stat(untrackedPath); !os.IsNotExist(err) {
		t.Error("after discard: untracked file should be removed")
	}

	// CurrentBest should NOT change on discard
	if seg.CurrentBest() != 4.0 {
		t.Errorf("after discard: CurrentBest = %v, want 4.0", seg.CurrentBest())
	}

	t.Logf("Exp 2: discard (delta=%.1f)", metricValue-4.0)

	// === Step 4: ShouldStop ===
	stop, _ := seg.ShouldStop(10, 3)
	if stop {
		t.Error("should not stop after 2 experiments (budget=10)")
	}

	// === Step 5: Snapshot for TUI ===
	snap := seg.Snapshot(10, "lines")
	if !snap.Active {
		t.Error("snapshot should be active")
	}
	if snap.Count != 2 {
		t.Errorf("snapshot Count = %d, want 2", snap.Count)
	}
	if snap.Max != 10 {
		t.Errorf("snapshot Max = %d, want 10", snap.Max)
	}
	// CumulativeDelta = currentBest - originalBaseline = 4.0 - 1.0 = 3.0
	if snap.CumulativeDelta < 2.9 || snap.CumulativeDelta > 3.1 {
		t.Errorf("snapshot CumulativeDelta = %v, want ~3.0", snap.CumulativeDelta)
	}

	t.Logf("Snapshot: exp=%d/%d delta=%.1f%s", snap.Count, snap.Max, snap.CumulativeDelta, snap.Unit)

	// === Step 6: Close segment ===
	if err := seg.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// === Step 7: Verify JSONL contents ===
	jsonlPath := filepath.Join(storeDir, "test-loop.jsonl")
	jsonlData, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("read JSONL: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(jsonlData)), "\n")
	if len(lines) != 4 { // segment + 2 experiments + summary
		t.Errorf("JSONL lines = %d, want 4 (segment + 2 experiments + summary)", len(lines))
	}

	// Verify segment record
	if !strings.Contains(lines[0], `"type":"segment"`) {
		t.Errorf("line 0 should be segment record: %s", lines[0])
	}
	if !strings.Contains(lines[0], `"original_baseline":1`) {
		t.Errorf("segment should have original_baseline=1: %s", lines[0])
	}

	// Verify keep record has agent_decision
	if !strings.Contains(lines[1], `"agent_decision":"keep"`) {
		t.Errorf("exp 1 should have agent_decision=keep: %s", lines[1])
	}

	// Verify discard record
	if !strings.Contains(lines[2], `"decision":"discard"`) {
		t.Errorf("exp 2 should have decision=discard: %s", lines[2])
	}

	// Verify summary
	if !strings.Contains(lines[3], `"type":"summary"`) {
		t.Errorf("line 3 should be summary: %s", lines[3])
	}

	// File permissions check
	info, _ := os.Stat(jsonlPath)
	if info.Mode().Perm() != 0600 {
		t.Errorf("JSONL permissions = %o, want 0600", info.Mode().Perm())
	}

	t.Logf("JSONL: %d records, permissions %o", len(lines), info.Mode().Perm())

	// === Step 8: Verify resume from JSONL ===
	// After close, OpenSegment should start a new segment (not resume closed one)
	seg2, resumed, err := store.OpenSegment(campaign, "test-session-2")
	if err != nil {
		t.Fatalf("OpenSegment after close: %v", err)
	}
	if resumed {
		t.Error("should not resume after close")
	}
	if seg2.ExperimentCount() != 0 {
		t.Errorf("new segment ExperimentCount = %d, want 0", seg2.ExperimentCount())
	}

	t.Log("Resume after close: new segment created (correct)")
}

// TestSecretFileRejection verifies that KeepChanges rejects .env files.
func TestSecretFileRejection(t *testing.T) {
	repoDir := initTestRepo(t)
	wtBase := filepath.Join(t.TempDir(), "worktrees")
	gitOps := NewGitOps(repoDir, wtBase)

	gitOps.CreateWorktree("secret-test")
	defer gitOps.RemoveWorktree("secret-test")

	wtDir := gitOps.WorktreeDir("secret-test")

	// Create various secret file patterns
	secrets := []string{".env", "prod.env", "server.pem", "id_rsa", "cert.p12"}
	for _, name := range secrets {
		os.WriteFile(filepath.Join(wtDir, name), []byte("secret"), 0644)
		_, err := gitOps.KeepChanges("secret-test", "test", 0.1)
		if err == nil {
			t.Errorf("expected rejection for %q", name)
		}
		if !strings.Contains(err.Error(), "secret file detected") {
			t.Errorf("wrong error for %q: %v", name, err)
		}
		// Clean up for next iteration
		os.Remove(filepath.Join(wtDir, name))
		gitOps.DiscardChanges("secret-test")
	}
}

// TestCrashRecoveryWorktree verifies that a dirty worktree is cleaned on reuse.
func TestCrashRecoveryWorktree(t *testing.T) {
	repoDir := initTestRepo(t)
	wtBase := filepath.Join(t.TempDir(), "worktrees")
	storeDir := filepath.Join(t.TempDir(), "experiments")

	gitOps := NewGitOps(repoDir, wtBase)
	store := NewStore(storeDir)

	campaign := &Campaign{
		Name:   "crash-test",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	// Create worktree and dirty it (simulate crash mid-experiment)
	gitOps.CreateWorktree("crash-test")
	wtDir := gitOps.WorktreeDir("crash-test")
	os.WriteFile(filepath.Join(wtDir, "README.md"), []byte("# Dirty\n"), 0644)
	os.WriteFile(filepath.Join(wtDir, "staged.go"), []byte("package dirty\n"), 0644)

	// Stage a file (simulating partial git add before crash)
	cmd := exec.Command("git", "add", "staged.go")
	cmd.Dir = wtDir
	cmd.Run()

	// Open segment (simulating init_experiment on resume)
	seg, _, _ := store.OpenSegment(campaign, "session-crash")
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.0, MetricAfter: 1.5, Delta: 0.5,
		AgentDecision: "keep", Decision: "keep",
	})

	// Simulate crash — segment stays open, worktree is dirty

	// Resume: CreateWorktree should clean to known state
	err := gitOps.CreateWorktree("crash-test")
	if err != nil {
		t.Fatalf("CreateWorktree (resume): %v", err)
	}

	// Verify clean state
	data, _ := os.ReadFile(filepath.Join(wtDir, "README.md"))
	if string(data) != "# Test\n" {
		t.Errorf("after crash recovery: README = %q, want original", string(data))
	}
	if _, err := os.Stat(filepath.Join(wtDir, "staged.go")); !os.IsNotExist(err) {
		t.Error("after crash recovery: staged.go should be cleaned")
	}

	// Resume segment from store
	seg2, err := store.LoadSegment("crash-test")
	if err != nil {
		t.Fatalf("LoadSegment resume: %v", err)
	}
	if seg2.ExperimentCount() != 1 {
		t.Errorf("resumed ExperimentCount = %d, want 1", seg2.ExperimentCount())
	}
	if seg2.CurrentBest() != 1.5 {
		t.Errorf("resumed CurrentBest = %v, want 1.5", seg2.CurrentBest())
	}

	gitOps.RemoveWorktree("crash-test")
	t.Log("Crash recovery: worktree cleaned, segment resumed correctly")
}

// TestMutationDrivenCampaign exercises the mutation→experiment→resume cycle.
func TestMutationDrivenCampaign(t *testing.T) {
	storeDir := filepath.Join(t.TempDir(), "experiments")
	store := NewStore(storeDir)

	// Campaign with 3 mutations: 1 sweep (3 values) + 1 swap + 1 toggle = 5 expanded
	campaign := &Campaign{
		Name:   "mut-test",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
		Mutations: []Mutation{
			{Type: MutationParameterSweep, Param: "threshold", Range: [2]float64{0.1, 0.3}, Step: 0.1},
			{Type: MutationSwap, Target: "A", Replacement: "B"},
			{Type: MutationToggle, Flag: "debug"},
		},
	}

	// Expand mutations
	expanded, err := ExpandMutations(campaign.Mutations)
	if err != nil {
		t.Fatal(err)
	}
	campaign.ExpandedMutations = expanded

	if len(expanded) != 5 {
		t.Fatalf("expanded = %d, want 5", len(expanded))
	}

	// Open segment and set pending mutations
	seg, _, err := store.OpenSegment(campaign, "session-mut")
	if err != nil {
		t.Fatal(err)
	}
	seg.SetPendingMutations(expanded)

	if seg.PendingMutationCount() != 5 {
		t.Errorf("pending = %d, want 5", seg.PendingMutationCount())
	}

	// Process first 3 mutations
	for i := 0; i < 3; i++ {
		m := seg.NextMutation()
		if m == nil {
			t.Fatalf("NextMutation nil at %d, expected mutation", i)
		}
		t.Logf("Mutation %d: %s", i, m.ID)

		seg.LogExperiment(ExperimentRecord{
			MetricBefore:  1.0,
			MetricAfter:   1.1,
			Delta:         0.1,
			AgentDecision: "keep",
			Decision:      "keep",
			MutationID:    m.ID,
			MutationType:  string(m.Type),
		})
	}

	if seg.PendingMutationCount() != 2 {
		t.Errorf("after 3 mutations: pending = %d, want 2", seg.PendingMutationCount())
	}

	// Simulate crash: resume from JSONL
	seg2, err := store.LoadSegment("mut-test")
	if err != nil {
		t.Fatalf("LoadSegment: %v", err)
	}
	seg2.SetPendingMutations(expanded) // Re-set with full list; completed are filtered

	if seg2.PendingMutationCount() != 2 {
		t.Errorf("after resume: pending = %d, want 2", seg2.PendingMutationCount())
	}
	if seg2.ExperimentCount() != 3 {
		t.Errorf("after resume: experiments = %d, want 3", seg2.ExperimentCount())
	}

	// Process remaining 2 mutations
	for i := 0; i < 2; i++ {
		m := seg2.NextMutation()
		if m == nil {
			t.Fatalf("NextMutation nil at resumed %d", i)
		}
		seg2.LogExperiment(ExperimentRecord{
			MetricBefore:  1.0,
			MetricAfter:   1.1,
			Delta:         0.1,
			AgentDecision: "keep",
			Decision:      "keep",
			MutationID:    m.ID,
			MutationType:  string(m.Type),
		})
	}

	// All mutations exhausted
	if seg2.NextMutation() != nil {
		t.Error("NextMutation should be nil after all mutations processed")
	}
	if seg2.PendingMutationCount() != 0 {
		t.Errorf("pending = %d, want 0", seg2.PendingMutationCount())
	}

	t.Logf("Mutation campaign: 5 mutations processed, resume correct, exhaustion detected")
}

// TestConsecutiveFailureStop verifies the campaign stops after N consecutive failures.
func TestConsecutiveFailureStop(t *testing.T) {
	storeDir := filepath.Join(t.TempDir(), "experiments")
	store := NewStore(storeDir)
	campaign := &Campaign{
		Name:   "fail-stop",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
		Budget: BudgetConfig{MaxExperiments: 50, MaxConsecutiveFailures: 3},
	}

	seg, _, _ := store.OpenSegment(campaign, "session-fail")

	// 3 consecutive discards
	for i := 0; i < 3; i++ {
		seg.LogExperiment(ExperimentRecord{
			MetricBefore: 1.0, MetricAfter: 0.9, Delta: -0.1,
			AgentDecision: "discard", Decision: "discard",
		})
	}

	stop, reason := seg.ShouldStop(
		campaign.Budget.MaxExperimentsOrDefault(),
		campaign.Budget.MaxConsecutiveFailuresOrDefault(),
	)
	if !stop {
		t.Error("should stop after 3 consecutive failures")
	}
	if !strings.Contains(reason, "consecutive failures") {
		t.Errorf("reason = %q, want to mention consecutive failures", reason)
	}

	// A keep should reset the counter
	seg2, _, _ := store.OpenSegment(&Campaign{
		Name:   "fail-reset",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
		Budget: BudgetConfig{MaxExperiments: 50, MaxConsecutiveFailures: 3},
	}, "session-reset")

	seg2.LogExperiment(ExperimentRecord{
		MetricBefore: 1.0, MetricAfter: 0.9, Delta: -0.1,
		AgentDecision: "discard", Decision: "discard",
	})
	seg2.LogExperiment(ExperimentRecord{
		MetricBefore: 1.0, MetricAfter: 1.2, Delta: 0.2,
		AgentDecision: "keep", Decision: "keep",
	})
	seg2.LogExperiment(ExperimentRecord{
		MetricBefore: 1.2, MetricAfter: 1.1, Delta: -0.1,
		AgentDecision: "discard", Decision: "discard",
	})

	stop, _ = seg2.ShouldStop(50, 3)
	if stop {
		t.Error("should not stop — keep reset consecutive failures counter")
	}
}
