package lens

// Scale indicates the analytical scope of a lens.
type Scale string

const (
	ScaleMacro Scale = "macro"
	ScaleMeso  Scale = "meso"
	ScaleMicro Scale = "micro"
)

// Tier indicates whether a lens is part of the core or extended set.
type Tier string

const (
	TierCore     Tier = "core"
	TierExtended Tier = "extended"
)

// Confidence indicates how well-established a lens is.
type Confidence string

const (
	ConfidenceEstablished Confidence = "established"
	ConfidenceEmerging    Confidence = "emerging"
	ConfidenceSpeculative Confidence = "speculative"
)

// EvidenceLevel describes the empirical backing of a lens.
type EvidenceLevel string

const (
	EvidenceLevelEmpiricallyValidated     EvidenceLevel = "empirically_validated"
	EvidenceLevelPractitionerEstablished  EvidenceLevel = "practitioner_established"
	EvidenceLevelTheoretical              EvidenceLevel = "theoretical"
)

// EdgeType describes the relationship between two lenses.
type EdgeType string

const (
	EdgeTypeComplements EdgeType = "complements"
	EdgeTypeContrasts   EdgeType = "contrasts"
	EdgeTypeRefines     EdgeType = "refines"
	EdgeTypeSequences   EdgeType = "sequences"
)

// LensRef is a lightweight reference to a lens.
type LensRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Holding is a judicial holding that makes a distinguishing question durable.
type Holding struct {
	OperativeCondition string `json:"operative_condition"`
	Rationale          string `json:"rationale"`
	Scope              string `json:"scope"`
	StressTestRef      string `json:"stress_test_ref,omitempty"`
}

// DistinguishingFeature is a contrastive feature that discriminates
// a lens from a specific near-miss.
type DistinguishingFeature struct {
	DiscriminatesAgainst string   `json:"discriminates_against"`
	Feature              string   `json:"feature"`
	Question             string   `json:"question,omitempty"`
	Holding              *Holding `json:"holding,omitempty"`
}

// Lens is a conceptual framework for cognitive augmentation.
type Lens struct {
	ID                     string                  `json:"id"`
	Name                   string                  `json:"name"`
	Definition             string                  `json:"definition"`
	Scale                  Scale                   `json:"scale"`
	Tier                   Tier                    `json:"tier"`
	Context                string                  `json:"context"`
	Forces                 []string                `json:"forces"`
	Solution               string                  `json:"solution"`
	Questions              []string                `json:"questions"`
	Examples               []string                `json:"examples"`
	Source                 string                  `json:"source"`
	Confidence             Confidence              `json:"confidence"`
	EvidenceLevel          EvidenceLevel           `json:"evidence_level"`
	Discipline             string                  `json:"discipline"`
	BridgeScore            float64                 `json:"bridge_score"`
	CommunityID            int                     `json:"community_id"`
	UsageCount             int                     `json:"usage_count"`
	EffectivenessScore     float64                 `json:"effectiveness_score"`
	Contraindications      []string                `json:"contraindications"`
	NearMissLenses         []string                `json:"near_miss_lenses"`
	DistinguishingFeatures []DistinguishingFeature  `json:"distinguishing_features"`
	FailureSignatures      []string                `json:"failure_signatures"`
}

// Ref returns a lightweight reference to this lens.
func (l *Lens) Ref() LensRef {
	return LensRef{ID: l.ID, Name: l.Name}
}

// Edge is a typed relationship between two lenses.
type Edge struct {
	SourceID   string   `json:"source_id"`
	TargetID   string   `json:"target_id"`
	Type       EdgeType `json:"type"`
	Confidence float64  `json:"confidence"`
	Rationale  string   `json:"rationale"`
	Provenance string   `json:"provenance"`
	Symmetric  bool     `json:"symmetric"`
}

// Community is a detected cluster of related lenses.
type Community struct {
	ID      int      `json:"id"`
	Members []string `json:"members"`
}
