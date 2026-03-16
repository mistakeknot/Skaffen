package experiment

import (
	"fmt"
	"strings"
	"time"
)

// GenerateResultsMarkdown produces a human-readable RESULTS.md from campaign analysis.
// This is the permanent record of what a campaign discovered.
func GenerateResultsMarkdown(analysis *CampaignAnalysis) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Autoresearch Results: %s\n\n", analysis.Campaign))
	b.WriteString(fmt.Sprintf("**Generated:** %s\n\n", time.Now().UTC().Format("2006-01-02 15:04 UTC")))

	// Summary
	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	b.WriteString(fmt.Sprintf("|--------|-------|\n"))
	b.WriteString(fmt.Sprintf("| Total experiments | %d |\n", analysis.TotalExperiments))
	b.WriteString(fmt.Sprintf("| Kept | %d |\n", analysis.Kept))
	b.WriteString(fmt.Sprintf("| Discarded | %d |\n", analysis.Discarded))
	b.WriteString(fmt.Sprintf("| Keep rate | %.0f%% |\n", keepRate(analysis.Kept, analysis.TotalExperiments)*100))
	b.WriteString(fmt.Sprintf("| Original baseline | %.4f |\n", analysis.OriginalBaseline))
	b.WriteString(fmt.Sprintf("| Final best | %.4f |\n", analysis.FinalBest))
	b.WriteString(fmt.Sprintf("| Cumulative improvement | %+.4f (%.1f%%) |\n\n", analysis.CumulativeDelta, analysis.ImprovementPct))

	// Convergence
	if len(analysis.Convergence) > 0 {
		b.WriteString("## Convergence\n\n")
		b.WriteString("| # | Best | Delta | Decision |\n")
		b.WriteString("|---|------|-------|----------|\n")
		for _, c := range analysis.Convergence {
			b.WriteString(fmt.Sprintf("| %d | %.4f | %+.4f | %s |\n", c.Experiment, c.Best, c.Delta, c.Decision))
		}
		b.WriteString("\n")
	}

	// Mutation effectiveness
	if len(analysis.MutationStats) > 0 {
		b.WriteString("## Mutation Type Effectiveness\n\n")
		b.WriteString("| Type | Total | Kept | Keep Rate | Avg Delta | Best Delta |\n")
		b.WriteString("|------|-------|------|-----------|-----------|------------|\n")
		for typ, ms := range analysis.MutationStats {
			b.WriteString(fmt.Sprintf("| %s | %d | %d | %.0f%% | %+.4f | %+.4f |\n",
				typ, ms.Total, ms.Kept, ms.KeepRate*100, ms.AvgDelta, ms.BestDelta))
		}
		b.WriteString("\n")
	}

	// Secondary metrics
	if len(analysis.SecondaryCorr) > 0 {
		b.WriteString("## Secondary Metrics\n\n")
		b.WriteString("| Metric | Baseline | Final | Correlation | Overrides |\n")
		b.WriteString("|--------|----------|-------|-------------|----------|\n")
		for _, ss := range analysis.SecondaryCorr {
			b.WriteString(fmt.Sprintf("| %s | %.4f | %.4f | %s | %d |\n",
				ss.Name, ss.BaselineValue, ss.FinalValue, ss.CorrelationDir, ss.OverrideCount))
		}
		b.WriteString("\n")
	}

	// Overrides
	if len(analysis.Overrides) > 0 {
		b.WriteString("## Decision Overrides\n\n")
		b.WriteString("| Experiment | Agent Said | System Did | Reason |\n")
		b.WriteString("|-----------|-----------|-----------|--------|\n")
		for _, o := range analysis.Overrides {
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				o.ExperimentID, o.AgentDecision, o.EffectiveDecn, o.OverrideReason))
		}
		b.WriteString("\n")
	}

	// Diminishing returns
	if analysis.DiminishingReturns != nil {
		b.WriteString("## Diminishing Returns\n\n")
		if analysis.DiminishingReturns.Detected {
			b.WriteString(fmt.Sprintf("**Detected.** %s\n\n", analysis.DiminishingReturns.Message))
			b.WriteString(fmt.Sprintf("- First-N keep rate: %.0f%% → Last-N: %.0f%%\n",
				analysis.DiminishingReturns.FirstNKeepRate*100, analysis.DiminishingReturns.LastNKeepRate*100))
			b.WriteString(fmt.Sprintf("- First-N avg delta: %.4f → Last-N: %.4f\n\n",
				analysis.DiminishingReturns.FirstNAvgDelta, analysis.DiminishingReturns.LastNAvgDelta))
		} else {
			b.WriteString(fmt.Sprintf("Not detected. %s\n\n", analysis.DiminishingReturns.Message))
		}
	}

	return b.String()
}

func keepRate(kept, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(kept) / float64(total)
}
