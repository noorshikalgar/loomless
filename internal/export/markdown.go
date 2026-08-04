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
// screenshot (linked by relative path) and caption. dir is the session's
// folder — needed to flatten any annotations onto a copy of a step's
// image (see stepImagePath in image.go); the original on disk is never
// touched.
func Markdown(s *session.Session, dir string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", s.Name)
	fmt.Fprintf(&b, "_Captured %s", s.StartedAt.Format("2006-01-02 15:04"))
	if s.StoppedAt != nil {
		fmt.Fprintf(&b, " – %s", s.StoppedAt.Format("15:04"))
	}
	b.WriteString("_\n\n")

	steps := s.VisibleSteps()
	if len(steps) == 0 {
		b.WriteString("_No steps captured._\n")
		return b.String()
	}

	for i, step := range steps {
		position := i + 1
		fmt.Fprintf(&b, "## %s\n\n", stepHeading(step, position))

		if step.Command != "" {
			fmt.Fprintf(&b, "```bash\n$ %s\n```\n\n", step.Command)
			if step.Output != "" {
				fmt.Fprintf(&b, "```\n%s\n```\n\n", step.Output)
			}
			if step.ExitCode != nil && *step.ExitCode != 0 {
				fmt.Fprintf(&b, "_Exited %d._", *step.ExitCode)
				if step.OutputFile != "" {
					fmt.Fprintf(&b, " Full output: [%s](%s).", step.OutputFile, step.OutputFile)
				}
				b.WriteString("\n\n")
			} else if step.OutputFile != "" && strings.Contains(step.Output, "lines omitted") {
				fmt.Fprintf(&b, "_Full output: [%s](%s)._\n\n", step.OutputFile, step.OutputFile)
			}
			continue
		}

		if segs := windowLineSegments(step); len(segs) > 0 {
			fmt.Fprintf(&b, "**Window:** %s\n\n", strings.Join(segs, " — "))
		}
		if step.Note != "" {
			fmt.Fprintf(&b, "%s\n\n", step.Note)
		}
		if step.ImageFile != "" {
			imgPath, err := stepImagePath(dir, step)
			if err != nil {
				imgPath = step.ImageFile // fall back to the unannotated original rather than dropping the image
			}
			fmt.Fprintf(&b, "![Step %d](%s)\n\n", position, imgPath)
		}
	}

	return b.String()
}
