package experiment

import (
	"testing"
)

func TestMutationExpandParameterSweep(t *testing.T) {
	m := Mutation{
		Type:  MutationParameterSweep,
		Param: "threshold",
		File:  "router.go",
		Range: [2]float64{0.1, 0.3},
		Step:  0.1,
	}
	expanded, err := ExpandMutations([]Mutation{m})
	if err != nil {
		t.Fatalf("ExpandMutations: %v", err)
	}
	if len(expanded) != 3 {
		t.Fatalf("expanded = %d, want 3 (0.1, 0.2, 0.3)", len(expanded))
	}

	// Verify IDs are deterministic
	if expanded[0].ID != "mutation:parameter_sweep:threshold:0.1" {
		t.Errorf("ID[0] = %q, want mutation:parameter_sweep:threshold:0.1", expanded[0].ID)
	}
	if expanded[2].ID != "mutation:parameter_sweep:threshold:0.3" {
		t.Errorf("ID[2] = %q, want mutation:parameter_sweep:threshold:0.3", expanded[2].ID)
	}

	// Verify params
	if expanded[1].Params["value"] != 0.2 {
		t.Errorf("Params[value] = %v, want 0.2", expanded[1].Params["value"])
	}
}

func TestMutationExpandParameterSweep_Deterministic(t *testing.T) {
	m := Mutation{
		Type:  MutationParameterSweep,
		Param: "lr",
		Range: [2]float64{0.001, 0.005},
		Step:  0.001,
	}
	exp1, _ := ExpandMutations([]Mutation{m})
	exp2, _ := ExpandMutations([]Mutation{m})

	if len(exp1) != len(exp2) {
		t.Fatalf("non-deterministic: len %d vs %d", len(exp1), len(exp2))
	}
	for i := range exp1 {
		if exp1[i].ID != exp2[i].ID {
			t.Errorf("non-deterministic ID at %d: %q vs %q", i, exp1[i].ID, exp2[i].ID)
		}
	}
}

func TestMutationExpandSwap(t *testing.T) {
	m := Mutation{
		Type:        MutationSwap,
		Target:      "json.Marshal",
		Replacement: "jsoniter.Marshal",
		Files:       []string{"*.go"},
	}
	expanded, err := ExpandMutations([]Mutation{m})
	if err != nil {
		t.Fatalf("ExpandMutations: %v", err)
	}
	if len(expanded) != 1 {
		t.Fatalf("expanded = %d, want 1", len(expanded))
	}
	if expanded[0].Type != MutationSwap {
		t.Errorf("Type = %q, want swap", expanded[0].Type)
	}
}

func TestMutationExpandToggle(t *testing.T) {
	m := Mutation{Type: MutationToggle, Flag: "cache_enabled", File: "config.go"}
	expanded, err := ExpandMutations([]Mutation{m})
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded) != 1 {
		t.Fatalf("expanded = %d, want 1", len(expanded))
	}
	if expanded[0].ID != "mutation:toggle:cache_enabled" {
		t.Errorf("ID = %q", expanded[0].ID)
	}
}

func TestMutationExpandScale(t *testing.T) {
	m := Mutation{Type: MutationScale, Param: "batch_size", Factors: []float64{0.5, 2.0, 4.0}}
	expanded, err := ExpandMutations([]Mutation{m})
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded) != 3 {
		t.Fatalf("expanded = %d, want 3", len(expanded))
	}
}

func TestMutationExpandRemove(t *testing.T) {
	m := Mutation{Type: MutationRemove, Target: "debug_logging", File: "agent.go", Lines: "45-52"}
	expanded, err := ExpandMutations([]Mutation{m})
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded) != 1 {
		t.Fatalf("expanded = %d, want 1", len(expanded))
	}
	if expanded[0].Params["lines"] != "45-52" {
		t.Errorf("Params[lines] = %v, want 45-52", expanded[0].Params["lines"])
	}
}

func TestMutationExpandReorder(t *testing.T) {
	m := Mutation{Type: MutationReorder, Items: []string{"a", "b", "c"}, File: "pipeline.go"}
	expanded, err := ExpandMutations([]Mutation{m})
	if err != nil {
		t.Fatal(err)
	}
	// 3! = 6 permutations minus identity = 5
	if len(expanded) != 5 {
		t.Fatalf("expanded = %d, want 5 (3! - identity)", len(expanded))
	}
}

func TestMutationExpandReorder_MaxPermutations(t *testing.T) {
	m := Mutation{
		Type:            MutationReorder,
		Items:           []string{"a", "b", "c", "d", "e"},
		MaxPermutations: 10,
	}
	expanded, err := ExpandMutations([]Mutation{m})
	if err != nil {
		t.Fatal(err)
	}
	// 5! = 120, capped to 10, minus identity (if in first 10)
	if len(expanded) > 10 {
		t.Errorf("expanded = %d, should be <= 10 (max_permutations)", len(expanded))
	}
}

func TestMutationExpandEnumSweep(t *testing.T) {
	m := Mutation{Type: MutationEnumSweep, Param: "model", Values: []string{"haiku", "sonnet", "opus"}}
	expanded, err := ExpandMutations([]Mutation{m})
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded) != 3 {
		t.Fatalf("expanded = %d, want 3", len(expanded))
	}
	if expanded[0].ID != "mutation:enum_sweep:model:haiku" {
		t.Errorf("ID[0] = %q", expanded[0].ID)
	}
}

func TestMutationExpandNil(t *testing.T) {
	expanded, err := ExpandMutations(nil)
	if err != nil {
		t.Fatal(err)
	}
	if expanded != nil {
		t.Errorf("expected nil for nil input, got %v", expanded)
	}
}

func TestMutationExpandEmpty(t *testing.T) {
	expanded, err := ExpandMutations([]Mutation{})
	if err != nil {
		t.Fatal(err)
	}
	if expanded != nil {
		t.Errorf("expected nil for empty input, got %v", expanded)
	}
}

func TestMutationExpandInvalidType(t *testing.T) {
	m := Mutation{Type: "nonexistent"}
	_, err := ExpandMutations([]Mutation{m})
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestMutationExpandMissingFields(t *testing.T) {
	cases := []struct {
		name string
		m    Mutation
	}{
		{"sweep_no_param", Mutation{Type: MutationParameterSweep, Step: 0.1, Range: [2]float64{0, 1}}},
		{"sweep_no_step", Mutation{Type: MutationParameterSweep, Param: "x", Range: [2]float64{0, 1}}},
		{"sweep_bad_range", Mutation{Type: MutationParameterSweep, Param: "x", Step: 0.1, Range: [2]float64{1, 0}}},
		{"swap_no_target", Mutation{Type: MutationSwap, Replacement: "b"}},
		{"swap_no_replacement", Mutation{Type: MutationSwap, Target: "a"}},
		{"toggle_no_flag", Mutation{Type: MutationToggle}},
		{"scale_no_param", Mutation{Type: MutationScale, Factors: []float64{2}}},
		{"scale_no_factors", Mutation{Type: MutationScale, Param: "x"}},
		{"remove_no_target", Mutation{Type: MutationRemove}},
		{"reorder_one_item", Mutation{Type: MutationReorder, Items: []string{"a"}}},
		{"enum_no_param", Mutation{Type: MutationEnumSweep, Values: []string{"a"}}},
		{"enum_no_values", Mutation{Type: MutationEnumSweep, Param: "x"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ExpandMutations([]Mutation{tc.m})
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestMutationMultipleMixed(t *testing.T) {
	mutations := []Mutation{
		{Type: MutationParameterSweep, Param: "lr", Range: [2]float64{0.1, 0.3}, Step: 0.1},
		{Type: MutationSwap, Target: "A", Replacement: "B"},
		{Type: MutationToggle, Flag: "debug"},
	}
	expanded, err := ExpandMutations(mutations)
	if err != nil {
		t.Fatal(err)
	}
	// 3 sweep + 1 swap + 1 toggle = 5
	if len(expanded) != 5 {
		t.Errorf("expanded = %d, want 5", len(expanded))
	}
}
