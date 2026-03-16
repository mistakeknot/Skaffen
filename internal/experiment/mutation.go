package experiment

import (
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

// MutationType identifies the kind of structured mutation.
type MutationType string

const (
	MutationParameterSweep MutationType = "parameter_sweep"
	MutationSwap           MutationType = "swap"
	MutationToggle         MutationType = "toggle"
	MutationScale          MutationType = "scale"
	MutationRemove         MutationType = "remove"
	MutationReorder        MutationType = "reorder"
	MutationEnumSweep      MutationType = "enum_sweep"
)

const (
	defaultMaxPermutations = 24
	warnExpandedThreshold  = 50
)

// Mutation defines a structured code transformation in a campaign YAML.
type Mutation struct {
	Type            MutationType `yaml:"type"`
	Param           string       `yaml:"param,omitempty"`
	File            string       `yaml:"file,omitempty"`
	Files           []string     `yaml:"files,omitempty"`
	Range           [2]float64   `yaml:"range,omitempty"`
	Step            float64      `yaml:"step,omitempty"`
	Values          []string     `yaml:"values,omitempty"`
	Target          string       `yaml:"target,omitempty"`
	Replacement     string       `yaml:"replacement,omitempty"`
	Flag            string       `yaml:"flag,omitempty"`
	Factors         []float64    `yaml:"factors,omitempty"`
	Items           []string     `yaml:"items,omitempty"`
	Lines           string       `yaml:"lines,omitempty"`
	MaxPermutations int          `yaml:"max_permutations,omitempty"`
	Description     string       `yaml:"description,omitempty"`
}

// ExpandedMutation is a single concrete experiment derived from a Mutation.
type ExpandedMutation struct {
	ID          string         `json:"id"`
	Type        MutationType   `json:"type"`
	Description string         `json:"description"`
	Params      map[string]any `json:"params"`
}

// ExpandMutations expands a list of mutations into individual experiments.
// Returns nil for nil/empty input (backward compatible).
func ExpandMutations(mutations []Mutation) ([]ExpandedMutation, error) {
	if len(mutations) == 0 {
		return nil, nil
	}

	var expanded []ExpandedMutation
	for i, m := range mutations {
		exps, err := expandOne(m)
		if err != nil {
			return nil, fmt.Errorf("mutations[%d] (%s): %w", i, m.Type, err)
		}
		expanded = append(expanded, exps...)
	}

	if len(expanded) > warnExpandedThreshold {
		fmt.Fprintf(os.Stderr, "autoresearch: warning: %d mutations expanded (threshold %d)\n",
			len(expanded), warnExpandedThreshold)
	}

	return expanded, nil
}

func expandOne(m Mutation) ([]ExpandedMutation, error) {
	switch m.Type {
	case MutationParameterSweep:
		return expandParameterSweep(m)
	case MutationSwap:
		return expandSwap(m)
	case MutationToggle:
		return expandToggle(m)
	case MutationScale:
		return expandScale(m)
	case MutationRemove:
		return expandRemove(m)
	case MutationReorder:
		return expandReorder(m)
	case MutationEnumSweep:
		return expandEnumSweep(m)
	default:
		return nil, fmt.Errorf("unknown mutation type %q", m.Type)
	}
}

func expandParameterSweep(m Mutation) ([]ExpandedMutation, error) {
	if m.Param == "" {
		return nil, fmt.Errorf("param is required for parameter_sweep")
	}
	if m.Step <= 0 {
		return nil, fmt.Errorf("step must be > 0 for parameter_sweep")
	}
	if m.Range[0] >= m.Range[1] {
		return nil, fmt.Errorf("range[0] must be < range[1] for parameter_sweep")
	}

	var result []ExpandedMutation
	for v := m.Range[0]; v <= m.Range[1]+m.Step/2; v += m.Step {
		// Round to avoid floating point drift
		v = math.Round(v*10000) / 10000
		if v > m.Range[1] {
			break
		}
		result = append(result, ExpandedMutation{
			ID:          fmt.Sprintf("mutation:parameter_sweep:%s:%.4g", m.Param, v),
			Type:        MutationParameterSweep,
			Description: descOrDefault(m.Description, fmt.Sprintf("Set %s to %.4g in %s", m.Param, v, fileTarget(m))),
			Params: map[string]any{
				"param": m.Param,
				"value": v,
				"file":  fileTarget(m),
			},
		})
	}
	return result, nil
}

func expandSwap(m Mutation) ([]ExpandedMutation, error) {
	if m.Target == "" {
		return nil, fmt.Errorf("target is required for swap")
	}
	if m.Replacement == "" {
		return nil, fmt.Errorf("replacement is required for swap")
	}
	return []ExpandedMutation{{
		ID:          fmt.Sprintf("mutation:swap:%s:%s", m.Target, m.Replacement),
		Type:        MutationSwap,
		Description: descOrDefault(m.Description, fmt.Sprintf("Replace %s with %s in %s", m.Target, m.Replacement, filesTarget(m))),
		Params: map[string]any{
			"target":      m.Target,
			"replacement": m.Replacement,
			"files":       filesTarget(m),
		},
	}}, nil
}

func expandToggle(m Mutation) ([]ExpandedMutation, error) {
	if m.Flag == "" {
		return nil, fmt.Errorf("flag is required for toggle")
	}
	return []ExpandedMutation{{
		ID:          fmt.Sprintf("mutation:toggle:%s", m.Flag),
		Type:        MutationToggle,
		Description: descOrDefault(m.Description, fmt.Sprintf("Toggle %s in %s", m.Flag, fileTarget(m))),
		Params: map[string]any{
			"flag": m.Flag,
			"file": fileTarget(m),
		},
	}}, nil
}

func expandScale(m Mutation) ([]ExpandedMutation, error) {
	if m.Param == "" {
		return nil, fmt.Errorf("param is required for scale")
	}
	if len(m.Factors) == 0 {
		return nil, fmt.Errorf("factors is required for scale")
	}
	var result []ExpandedMutation
	for _, f := range m.Factors {
		result = append(result, ExpandedMutation{
			ID:          fmt.Sprintf("mutation:scale:%s:%.4g", m.Param, f),
			Type:        MutationScale,
			Description: descOrDefault(m.Description, fmt.Sprintf("Scale %s by %.4gx in %s", m.Param, f, fileTarget(m))),
			Params: map[string]any{
				"param":  m.Param,
				"factor": f,
				"file":   fileTarget(m),
			},
		})
	}
	return result, nil
}

func expandRemove(m Mutation) ([]ExpandedMutation, error) {
	if m.Target == "" {
		return nil, fmt.Errorf("target is required for remove")
	}
	return []ExpandedMutation{{
		ID:          fmt.Sprintf("mutation:remove:%s", m.Target),
		Type:        MutationRemove,
		Description: descOrDefault(m.Description, fmt.Sprintf("Remove %s from %s", m.Target, fileTarget(m))),
		Params: map[string]any{
			"target": m.Target,
			"file":   fileTarget(m),
			"lines":  m.Lines,
		},
	}}, nil
}

func expandReorder(m Mutation) ([]ExpandedMutation, error) {
	if len(m.Items) < 2 {
		return nil, fmt.Errorf("at least 2 items required for reorder")
	}
	maxPerm := m.MaxPermutations
	if maxPerm <= 0 {
		maxPerm = defaultMaxPermutations
	}

	perms := permutations(m.Items)
	if len(perms) > maxPerm {
		perms = perms[:maxPerm]
	}

	// Skip the identity permutation (original order)
	original := strings.Join(m.Items, ",")
	var result []ExpandedMutation
	for _, perm := range perms {
		ordered := strings.Join(perm, ",")
		if ordered == original {
			continue
		}
		h := sha256.Sum256([]byte(ordered))
		result = append(result, ExpandedMutation{
			ID:          fmt.Sprintf("mutation:reorder:%x", h[:4]),
			Type:        MutationReorder,
			Description: descOrDefault(m.Description, fmt.Sprintf("Reorder to [%s] in %s", ordered, fileTarget(m))),
			Params: map[string]any{
				"items": perm,
				"file":  fileTarget(m),
			},
		})
	}
	return result, nil
}

func expandEnumSweep(m Mutation) ([]ExpandedMutation, error) {
	if m.Param == "" {
		return nil, fmt.Errorf("param is required for enum_sweep")
	}
	if len(m.Values) == 0 {
		return nil, fmt.Errorf("values is required for enum_sweep")
	}
	var result []ExpandedMutation
	for _, v := range m.Values {
		result = append(result, ExpandedMutation{
			ID:          fmt.Sprintf("mutation:enum_sweep:%s:%s", m.Param, v),
			Type:        MutationEnumSweep,
			Description: descOrDefault(m.Description, fmt.Sprintf("Set %s to %q in %s", m.Param, v, fileTarget(m))),
			Params: map[string]any{
				"param": m.Param,
				"value": v,
				"file":  fileTarget(m),
			},
		})
	}
	return result, nil
}

// ValidateMutation checks that a mutation has the required fields for its type.
func ValidateMutation(m Mutation) error {
	_, err := expandOne(m)
	return err
}

// helpers

func fileTarget(m Mutation) string {
	if m.File != "" {
		return m.File
	}
	if len(m.Files) > 0 {
		return strings.Join(m.Files, ", ")
	}
	return "(unspecified)"
}

func filesTarget(m Mutation) string {
	if len(m.Files) > 0 {
		return strings.Join(m.Files, ", ")
	}
	if m.File != "" {
		return m.File
	}
	return "(unspecified)"
}

func descOrDefault(desc, fallback string) string {
	if desc != "" {
		return desc
	}
	return fallback
}

// permutations generates all permutations of a string slice.
func permutations(items []string) [][]string {
	if len(items) <= 1 {
		return [][]string{append([]string{}, items...)}
	}

	var result [][]string
	for i, item := range items {
		rest := make([]string, 0, len(items)-1)
		rest = append(rest, items[:i]...)
		rest = append(rest, items[i+1:]...)
		for _, perm := range permutations(rest) {
			result = append(result, append([]string{item}, perm...))
		}
	}

	// Stable sort for deterministic output
	sort.Slice(result, func(i, j int) bool {
		return strings.Join(result[i], ",") < strings.Join(result[j], ",")
	})

	return result
}
