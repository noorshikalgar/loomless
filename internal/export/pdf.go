package export

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"

	"github.com/noorshikalgar/loomless/internal/session"
)

const (
	pageMarginMM  = 15.0
	usableWidthMM = 210 - 2*pageMarginMM // A4 width minus left/right margins
	pageBottomMM  = 297 - pageMarginMM   // A4 height minus bottom margin
)

// PDF renders a session as a PDF: one section per step, in order, with
// embedded screenshots for window/click steps and monospace command+output
// blocks for terminal steps — the direct-to-PDF equivalent of Markdown's
// layout, built from the same session.Session data (not by converting the
// rendered Markdown, which would need a Markdown-to-PDF renderer and its
// own dependency). dir is the session's folder, needed to read screenshot
// bytes for embedding (Markdown only needs to *link* to them, PDF needs
// the actual pixels).
func PDF(s *session.Session, dir string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pageMarginMM, pageMarginMM, pageMarginMM)
	pdf.SetAutoPageBreak(true, pageMarginMM)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 20)
	pdf.MultiCell(0, 10, safe(s.Name), "", "L", false)

	pdf.SetFont("Helvetica", "I", 10)
	subtitle := "Captured " + s.StartedAt.Format("2006-01-02 15:04")
	if s.StoppedAt != nil {
		subtitle += " - " + s.StoppedAt.Format("15:04")
	}
	pdf.MultiCell(0, 6, safe(subtitle), "", "L", false)
	pdf.Ln(4)

	steps := s.VisibleSteps()
	if len(steps) == 0 {
		pdf.SetFont("Helvetica", "I", 11)
		pdf.MultiCell(0, 6, "No steps captured.", "", "L", false)
	}

	for i, step := range steps {
		pdf.SetFont("Helvetica", "B", 14)
		pdf.MultiCell(0, 9, safe(stepHeading(step, i+1)), "", "L", false)

		if step.Command != "" {
			renderTerminalStep(pdf, step)
		} else {
			renderWindowStep(pdf, step, dir)
		}
		pdf.Ln(5)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("render pdf: %w", err)
	}
	return buf.Bytes(), nil
}

func renderTerminalStep(pdf *fpdf.Fpdf, step session.Step) {
	pdf.SetFont("Courier", "", 9)
	pdf.SetFillColor(240, 240, 240)
	pdf.MultiCell(0, 5, safe("$ "+step.Command), "1", "L", true)
	if step.Output != "" {
		pdf.MultiCell(0, 5, safe(step.Output), "1", "L", true)
	}
	if step.ExitCode != nil && *step.ExitCode != 0 {
		pdf.SetFont("Helvetica", "I", 9)
		pdf.MultiCell(0, 6, safe(fmt.Sprintf("Exited %d.", *step.ExitCode)), "", "L", false)
	}
}

func renderWindowStep(pdf *fpdf.Fpdf, step session.Step, dir string) {
	if segs := windowLineSegments(step); len(segs) > 0 {
		pdf.SetFont("Helvetica", "B", 10)
		pdf.MultiCell(0, 6, safe("Window: "+strings.Join(segs, " - ")), "", "L", false)
	}
	if step.Note != "" {
		writeNotePDF(pdf, step.Note, dir)
	}
	if step.ImageFile == "" {
		return
	}
	imgData, err := stepImageBytes(dir, step)
	if err != nil {
		pdf.SetFont("Helvetica", "I", 9)
		pdf.MultiCell(0, 6, safe(fmt.Sprintf("[screenshot missing: %s]", step.ImageFile)), "", "L", false)
		return
	}

	info := pdf.RegisterImageOptionsReader(step.ImageFile, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(imgData))
	if info == nil {
		return
	}
	imgW, imgH := info.Extent()
	displayH := usableWidthMM * imgH / imgW

	// A screenshot taller than a full page would still only ever get one
	// page's worth of auto-page-break room; scale it down to fit within a
	// single page's usable height rather than letting it render truncated.
	maxH := pageBottomMM - pageMarginMM
	displayW := usableWidthMM
	if displayH > maxH {
		displayW = usableWidthMM * maxH / displayH
		displayH = maxH
	}

	if pdf.GetY()+displayH > pageBottomMM {
		pdf.AddPage()
	}
	pdf.ImageOptions(step.ImageFile, pdf.GetX(), pdf.GetY(), displayW, displayH, true, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
	pdf.Ln(2)
}

// safe replaces common non-ASCII punctuation with ASCII equivalents and
// strips anything else outside ASCII. fpdf's core fonts (Helvetica,
// Courier) aren't UTF-8 aware without a bundled codepage descriptor or an
// embedded TTF — sanitizing keeps this exporter dependency-free rather
// than shipping a font asset for what's otherwise a handful of characters
// (em dashes, smart quotes, the masking placeholder's guillemets).
func safe(s string) string {
	r := strings.NewReplacer(
		"—", "--",
		"–", "-",
		"«", "<<",
		"»", ">>",
		"’", "'",
		"‘", "'",
		"“", `"`,
		"”", `"`,
		"…", "...",
	)
	s = r.Replace(s)
	var b strings.Builder
	for _, ch := range s {
		if ch > 127 {
			b.WriteByte('?')
			continue
		}
		b.WriteRune(ch)
	}
	return b.String()
}
