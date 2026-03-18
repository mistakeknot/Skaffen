package trust_test

import (
	"fmt"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/trust"
)

// buildEvaluator creates an evaluator with n overrides: ~70% exact paths, ~30% glob patterns.
func buildEvaluator(n int) *trust.Evaluator {
	overrides := make([]trust.Override, 0, n)
	for i := range n {
		var pattern string
		if i%10 < 7 {
			// Exact path (~70%)
			pattern = fmt.Sprintf("bash:/home/user/project/src/pkg%d/file%d.go", i/10, i)
		} else {
			// Glob pattern (~30%)
			pattern = fmt.Sprintf("bash:/home/user/project/src/pkg%d/*.go", i)
		}
		overrides = append(overrides, trust.Override{
			Pattern:  pattern,
			Decision: trust.Allow,
			Scope:    trust.ScopeProject,
			Count:    1,
		})
	}
	return trust.NewEvaluator(&trust.Config{Overrides: overrides})
}

// evalKey is a path that matches nothing — worst case forces full scan.
const evalTool = "bash"
const evalParams = `{"command": "/home/user/other/repo/cmd/server/main.go build"}`

func BenchmarkEvaluate5Overrides(b *testing.B) {
	e := buildEvaluator(5)
	for b.Loop() {
		e.Evaluate(evalTool, evalParams)
	}
}

func BenchmarkEvaluate50Overrides(b *testing.B) {
	e := buildEvaluator(50)
	for b.Loop() {
		e.Evaluate(evalTool, evalParams)
	}
}

func BenchmarkEvaluate500Overrides(b *testing.B) {
	e := buildEvaluator(500)
	for b.Loop() {
		e.Evaluate(evalTool, evalParams)
	}
}
