// Package export renders a session into output formats. Phase 0 supports
// Markdown only (per docs/product/09-roadmap.md); PDF/slide-deck exports
// are Phase 2.
package export

import (
	"fmt"
	"strings"

	"github.com/noorshikalgar/story/internal/session"
)

// Markdown renders a session as an ordered list of steps, each with its
// screenshot (linked by relative path) and caption.
func Markdown(s *session.Session) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", s.Name)
	fmt.Fprintf(&b, "_Captured %s", s.StartedAt.Format("2006-01-02 15:04"))
	if s.StoppedAt != nil {
		fmt.Fprintf(&b, " – %s", s.StoppedAt.Format("15:04"))
	}
	b.WriteString("_\n\n")

	if len(s.Steps) == 0 {
		b.WriteString("_No steps captured._\n")
		return b.String()
	}

	for _, step := range s.Steps {
		fmt.Fprintf(&b, "## Step %d\n\n", step.Index)
		if step.WindowApp != "" {
			fmt.Fprintf(&b, "**Window:** %s", step.WindowApp)
			if step.WindowTitle != "" {
				fmt.Fprintf(&b, " — %s", step.WindowTitle)
			}
			b.WriteString("\n\n")
		}
		if step.Note != "" {
			fmt.Fprintf(&b, "%s\n\n", step.Note)
		}
		if step.ImageFile != "" {
			fmt.Fprintf(&b, "![Step %d](%s)\n\n", step.Index, step.ImageFile)
		}
	}

	return b.String()
}
