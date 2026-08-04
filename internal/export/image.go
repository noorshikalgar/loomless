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
// override (session.Step.Title) if one is set, otherwise "Step
// <position>". position is the step's 1-indexed place in the *current
// display order* (VisibleSteps()'s order), not step.Index — Index is a
// stable capture-time reference (decision #4: image/output filenames,
// EditLog targeting, and the annotation API all address a step by it,
// and it must never change once assigned) but that means it doesn't
// track reordering. A story reordered to (B, A, C) should read "Step 1,
// Step 2, Step 3" in that order, not the frozen indices B/A/C happened
// to capture in.
func stepHeading(step session.Step, position int) string {
	if step.Title != "" {
		return step.Title
	}
	return fmt.Sprintf("Step %d", position)
}

// toCaptureAnnotations adapts session.Annotation to capture.Annotation —
// two structurally-identical but distinct types, kept separate rather
// than shared to avoid a capture<->session import cycle (session already
// imports capture for Redact/RenderAnnotations).
func toCaptureAnnotations(anns []session.Annotation) []capture.Annotation {
	out := make([]capture.Annotation, len(anns))
	for i, a := range anns {
		out[i] = capture.Annotation{Number: a.Number, X: a.X, Y: a.Y, W: a.W, H: a.H, Color: a.Color, Points: toCapturePoints(a.Points)}
	}
	return out
}

func toCapturePoints(points []session.Point) []capture.Point {
	if points == nil {
		return nil
	}
	out := make([]capture.Point, len(points))
	for i, p := range points {
		out[i] = capture.Point{X: p.X, Y: p.Y}
	}
	return out
}

// stepImageBytes returns a step's screenshot, cropped (if step.Crop is
// set) and with any annotations baked in as a numbered ring + badge —
// used by exporters (PDF, Slides) that embed image bytes directly rather
// than linking to a file. The on-disk original is never modified; crop
// and annotations are both presentation-time data (decision #18 for
// annotations; see session.Step.Crop's doc comment for why crop works
// the same way) flattened fresh on every export.
//
// Crop happens first, and annotation coordinates shift to match (an
// annotation was drawn and is still stored in the *original* image's
// pixel space) — an annotation that ends up entirely outside the cropped
// canvas isn't specially filtered out here, it just doesn't draw
// anything, because RenderAnnotations' drawing primitives already clip
// to the target image's bounds on their own (same as Redact's fill
// does), so there's nothing extra to get wrong by handling it here too.
func stepImageBytes(dir string, step session.Step) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(dir, step.ImageFile))
	if err != nil {
		return nil, err
	}

	anns := step.Annotations
	if step.Crop != nil {
		cropped, err := capture.Crop(data, step.Crop.X, step.Crop.Y, step.Crop.W, step.Crop.H)
		if err != nil {
			return nil, err
		}
		data = cropped
		anns = shiftAnnotationsForCrop(anns, *step.Crop)
	}

	if len(anns) == 0 {
		return data, nil
	}
	return capture.RenderAnnotations(data, toCaptureAnnotations(anns))
}

// shiftAnnotationsForCrop rebases annotation positions from the original
// image's coordinate space to the cropped image's (subtracting the
// crop's own top-left corner) — copies rather than mutates, since the
// caller's step.Annotations is the session's actual in-memory data and
// must stay in original-image coordinates for the editor's own overlay
// (which always shows the full, uncropped image) to keep working.
func shiftAnnotationsForCrop(anns []session.Annotation, crop session.CropRect) []session.Annotation {
	shifted := make([]session.Annotation, len(anns))
	for i, a := range anns {
		a.X -= crop.X
		a.Y -= crop.Y
		if len(a.Points) > 0 {
			pts := make([]session.Point, len(a.Points))
			for j, p := range a.Points {
				pts[j] = session.Point{X: p.X - crop.X, Y: p.Y - crop.Y}
			}
			a.Points = pts
		}
		shifted[i] = a
	}
	return shifted
}

// stepImagePath returns a relative path (from dir) to a version of a
// step's screenshot with any crop/annotations baked in — used by
// Markdown, which links to an image file rather than embedding bytes. If
// the step has neither, this is just ImageFile itself, unchanged;
// otherwise a flattened copy is written alongside it (regenerated on
// every export, not part of the session's own data) and that path is
// returned instead.
func stepImagePath(dir string, step session.Step) (string, error) {
	if len(step.Annotations) == 0 && step.Crop == nil {
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
