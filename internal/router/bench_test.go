package router

import (
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

func BenchmarkBudgetRecord(b *testing.B) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000000,
		Mode:      "graceful",
		DegradeAt: 0.8,
	})
	usage := provider.Usage{
		InputTokens:  500,
		OutputTokens: 200,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bt.Record(usage)
	}
}

func BenchmarkBudgetRecordContext(b *testing.B) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000000,
		Mode:      "graceful",
		DegradeAt: 0.8,
		Tracking:  "context",
	})
	usage := provider.Usage{
		InputTokens:              500,
		OutputTokens:             200,
		CacheCreationInputTokens: 100,
		CacheReadInputTokens:     300,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bt.Record(usage)
	}
}

func BenchmarkBudgetMaybeDegrade(b *testing.B) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000,
		Mode:      "graceful",
		DegradeAt: 0.8,
	})
	// Pre-fill to 85% — triggers degrade path
	bt.spent = 850
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bt.MaybeDegrade(ModelSonnet, "base")
	}
}

func BenchmarkBudgetState(b *testing.B) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000000,
		Mode:      "graceful",
		DegradeAt: 0.8,
	})
	bt.spent = 500000
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bt.State()
	}
}
