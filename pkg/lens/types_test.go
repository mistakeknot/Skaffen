package lens

import (
	"encoding/json"
	"testing"
)

func TestLensJSONRoundTrip(t *testing.T) {
	original := Lens{
		ID:             "lens_test_roundtrip",
		Name:           "Test Roundtrip",
		Definition:     "A lens for testing JSON serialization.",
		Scale:          ScaleMeso,
		Tier:           TierCore,
		Context:        "When testing serialization",
		Forces:         []string{"correctness", "completeness"},
		Solution:       "Marshal and unmarshal, then compare.",
		Questions:      []string{"Did all fields survive?"},
		Examples:       []string{"This test"},
		Source:         "unit_test",
		Confidence:     ConfidenceEstablished,
		EvidenceLevel:  EvidenceLevelEmpiricallyValidated,
		Discipline:     "engineering",
		BridgeScore:    0.85,
		CommunityID:    3,
		UsageCount:     42,
		EffectivenessScore: 0.91,
		Contraindications:  []string{"none applicable"},
		NearMissLenses:     []string{"lens_other"},
		DistinguishingFeatures: []DistinguishingFeature{
			{
				DiscriminatesAgainst: "lens_other",
				Feature:              "handles edge cases",
				Question:             "Does it handle nulls?",
				Holding: &Holding{
					OperativeCondition: "when nulls present",
					Rationale:          "null safety matters",
					Scope:              "all fields",
					StressTestRef:      "st_001",
				},
			},
		},
		FailureSignatures: []string{"silent data loss"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Lens
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify scalar fields.
	if decoded.ID != original.ID {
		t.Errorf("ID: got %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Name != original.Name {
		t.Errorf("Name: got %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Definition != original.Definition {
		t.Errorf("Definition: got %q, want %q", decoded.Definition, original.Definition)
	}
	if decoded.Scale != original.Scale {
		t.Errorf("Scale: got %q, want %q", decoded.Scale, original.Scale)
	}
	if decoded.Tier != original.Tier {
		t.Errorf("Tier: got %q, want %q", decoded.Tier, original.Tier)
	}
	if decoded.Context != original.Context {
		t.Errorf("Context: got %q, want %q", decoded.Context, original.Context)
	}
	if decoded.Solution != original.Solution {
		t.Errorf("Solution: got %q, want %q", decoded.Solution, original.Solution)
	}
	if decoded.Source != original.Source {
		t.Errorf("Source: got %q, want %q", decoded.Source, original.Source)
	}
	if decoded.Confidence != original.Confidence {
		t.Errorf("Confidence: got %q, want %q", decoded.Confidence, original.Confidence)
	}
	if decoded.EvidenceLevel != original.EvidenceLevel {
		t.Errorf("EvidenceLevel: got %q, want %q", decoded.EvidenceLevel, original.EvidenceLevel)
	}
	if decoded.Discipline != original.Discipline {
		t.Errorf("Discipline: got %q, want %q", decoded.Discipline, original.Discipline)
	}
	if decoded.BridgeScore != original.BridgeScore {
		t.Errorf("BridgeScore: got %v, want %v", decoded.BridgeScore, original.BridgeScore)
	}
	if decoded.CommunityID != original.CommunityID {
		t.Errorf("CommunityID: got %d, want %d", decoded.CommunityID, original.CommunityID)
	}
	if decoded.UsageCount != original.UsageCount {
		t.Errorf("UsageCount: got %d, want %d", decoded.UsageCount, original.UsageCount)
	}
	if decoded.EffectivenessScore != original.EffectivenessScore {
		t.Errorf("EffectivenessScore: got %v, want %v", decoded.EffectivenessScore, original.EffectivenessScore)
	}

	// Verify slices.
	if len(decoded.Forces) != len(original.Forces) {
		t.Errorf("Forces length: got %d, want %d", len(decoded.Forces), len(original.Forces))
	}
	if len(decoded.Questions) != len(original.Questions) {
		t.Errorf("Questions length: got %d, want %d", len(decoded.Questions), len(original.Questions))
	}
	if len(decoded.Examples) != len(original.Examples) {
		t.Errorf("Examples length: got %d, want %d", len(decoded.Examples), len(original.Examples))
	}
	if len(decoded.Contraindications) != len(original.Contraindications) {
		t.Errorf("Contraindications length: got %d, want %d", len(decoded.Contraindications), len(original.Contraindications))
	}
	if len(decoded.NearMissLenses) != len(original.NearMissLenses) {
		t.Errorf("NearMissLenses length: got %d, want %d", len(decoded.NearMissLenses), len(original.NearMissLenses))
	}
	if len(decoded.FailureSignatures) != len(original.FailureSignatures) {
		t.Errorf("FailureSignatures length: got %d, want %d", len(decoded.FailureSignatures), len(original.FailureSignatures))
	}

	// Verify distinguishing features.
	if len(decoded.DistinguishingFeatures) != 1 {
		t.Fatalf("DistinguishingFeatures length: got %d, want 1", len(decoded.DistinguishingFeatures))
	}
	df := decoded.DistinguishingFeatures[0]
	if df.DiscriminatesAgainst != "lens_other" {
		t.Errorf("DistinguishingFeature.DiscriminatesAgainst: got %q, want %q", df.DiscriminatesAgainst, "lens_other")
	}
	if df.Feature != "handles edge cases" {
		t.Errorf("DistinguishingFeature.Feature: got %q, want %q", df.Feature, "handles edge cases")
	}
	if df.Holding == nil {
		t.Fatal("DistinguishingFeature.Holding: got nil, want non-nil")
	}
	if df.Holding.OperativeCondition != "when nulls present" {
		t.Errorf("Holding.OperativeCondition: got %q, want %q", df.Holding.OperativeCondition, "when nulls present")
	}
}

func TestEdgeJSONRoundTrip(t *testing.T) {
	original := Edge{
		SourceID:   "lens_a",
		TargetID:   "lens_b",
		Type:       EdgeTypeComplements,
		Confidence: 0.82,
		Rationale:  "They work well together.",
		Provenance: "auto_generated",
		Symmetric:  true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Edge
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.SourceID != original.SourceID {
		t.Errorf("SourceID: got %q, want %q", decoded.SourceID, original.SourceID)
	}
	if decoded.TargetID != original.TargetID {
		t.Errorf("TargetID: got %q, want %q", decoded.TargetID, original.TargetID)
	}
	if decoded.Type != original.Type {
		t.Errorf("Type: got %q, want %q", decoded.Type, original.Type)
	}
	if decoded.Confidence != original.Confidence {
		t.Errorf("Confidence: got %v, want %v", decoded.Confidence, original.Confidence)
	}
	if decoded.Rationale != original.Rationale {
		t.Errorf("Rationale: got %q, want %q", decoded.Rationale, original.Rationale)
	}
	if decoded.Provenance != original.Provenance {
		t.Errorf("Provenance: got %q, want %q", decoded.Provenance, original.Provenance)
	}
	if decoded.Symmetric != original.Symmetric {
		t.Errorf("Symmetric: got %v, want %v", decoded.Symmetric, original.Symmetric)
	}
}

func TestLensRef(t *testing.T) {
	l := Lens{
		ID:   "lens_test_ref",
		Name: "Test Ref Lens",
	}

	ref := l.Ref()
	if ref.ID != l.ID {
		t.Errorf("Ref().ID: got %q, want %q", ref.ID, l.ID)
	}
	if ref.Name != l.Name {
		t.Errorf("Ref().Name: got %q, want %q", ref.Name, l.Name)
	}
}
