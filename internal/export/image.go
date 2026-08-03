package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/noorshikalgar/story/internal/capture"
	"github.com/noorshikalgar/story/internal/session"
)

// stepHeading returns a step's display title — the user-editable
// override (session.Step.Title) if one is set, otherwise the default
// "Step <Index>" every exporter used before titles were editable.
func stepHeading(step session.Step) string {
	if step.Title != "" {
		return step.Title
	}
	return fmt.Sprintf("Step %d", step.Index)
}

// toCaptureAnnotations adapts session.Annotation to capture.Annotation —
// two structurally-identical but distinct types, kept separate rather
// than shared to avoid a capture<->session import cycle (session already
// imports capture for Redact/RenderAnnotations).
func toCaptureAnnotations(anns []session.Annotation) []capture.Annotation {
	out := make([]capture.Annotation, len(anns))
	for i, a := range anns {
		out[i] = capture.Annotation{Number: a.Number, X: a.X, Y: a.Y, W: a.W, H: a.H}
	}
	return out
}

// stepImageBytes returns a step's screenshot, with any annotations baked
// in as a numbered ring + badge — used by exporters (PDF, Slides) that
// embed image bytes directly rather than linking to a file. The
// on-disk original is never modified; annotations are presentation-time
// data (decision #18) flattened fresh on every export.
func stepImageBytes(dir string, step session.Step) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(dir, step.ImageFile))
	if err != nil {
		return nil, err
	}
	if len(step.Annotations) == 0 {
		return data, nil
	}
	return capture.RenderAnnotations(data, toCaptureAnnotations(step.Annotations))
}

// stepImagePath returns a relative path (from dir) to a version of a
// step's screenshot with any annotations baked in — used by Markdown,
// which links to an image file rather than embedding bytes. If the step
// has no annotations, this is just ImageFile itself, unchanged; otherwise
// a flattened copy is written alongside it (regenerated on every export,
// not part of the session's own data) and that path is returned instead.
func stepImagePath(dir string, step session.Step) (string, error) {
	if len(step.Annotations) == 0 {
		return step.ImageFile, nil
	}
	flattened, err := stepImageBytes(dir, step)
	if err != nil {
		return "", err
	}
	relPath := strings.TrimSuffix(step.ImageFile, filepath.Ext(step.ImageFile)) + ".annotated.png"
	if err := os.WriteFile(filepath.Join(dir, relPath), flattened, 0o644); err != nil {
		return "", err
	}
	return relPath, nil
}
