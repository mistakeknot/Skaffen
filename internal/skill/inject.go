package skill

import (
	"fmt"
	"strings"
)

// MaxBodyChars is the per-skill body size cap (~5K tokens ≈ 15K chars).
const MaxBodyChars = 15000

// FormatInjection formats a skill's content for injection as a user-role message.
// If args is non-empty, it is appended after the skill body.
func FormatInjection(d *Def, args string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<skill name=%q>\n", d.Name))
	if d.Body != "" {
		b.WriteString(d.Body)
		if !strings.HasSuffix(d.Body, "\n") {
			b.WriteString("\n")
		}
	}
	if args != "" {
		b.WriteString("\nARGUMENTS: ")
		b.WriteString(args)
		b.WriteString("\n")
	}
	b.WriteString("</skill>\n")
	return b.String()
}

// FormatInjectionSafe is like FormatInjection but returns an error if the
// skill body exceeds MaxBodyChars.
func FormatInjectionSafe(d *Def, args string) (string, error) {
	if len(d.Body) > MaxBodyChars {
		return "", fmt.Errorf("skill %q body is %d chars (max %d)", d.Name, len(d.Body), MaxBodyChars)
	}
	return FormatInjection(d, args), nil
}
