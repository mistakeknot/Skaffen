package auraken

import (
	"strings"

	"github.com/mistakeknot/Skaffen/pkg/lens"
	"github.com/mistakeknot/Skaffen/pkg/prompts"
)

// PromptLens converts a pkg/lens Lens into the prompt-injection shape,
// mirroring Auraken's lens_to_prompt_dict (lenses.py): forces joined with
// "; ", questions rendered as indented bullets, and how-to-apply falling
// back from solution to first example to a generic instruction.
func PromptLens(l lens.Lens) prompts.Lens {
	howTo := l.Solution
	if howTo == "" {
		if len(l.Examples) > 0 {
			howTo = "Apply by considering: " + l.Examples[0]
		} else {
			howTo = "Apply this framework to reframe the user's situation."
		}
	}

	var questions []string
	for _, q := range l.Questions {
		questions = append(questions, "  - "+q)
	}

	return prompts.Lens{
		Name:        l.Name,
		Description: l.Definition,
		WhenToApply: l.Context,
		HowToApply:  howTo,
		Forces:      strings.Join(l.Forces, "; "),
		Questions:   strings.Join(questions, "\n"),
	}
}

// PromptLenses converts a slice of lenses.
func PromptLenses(ls []lens.Lens) []prompts.Lens {
	if len(ls) == 0 {
		return nil
	}
	out := make([]prompts.Lens, len(ls))
	for i, l := range ls {
		out[i] = PromptLens(l)
	}
	return out
}
