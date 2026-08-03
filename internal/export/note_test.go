package export

import (
	"strings"
	"testing"

	"github.com/go-pdf/fpdf"
)

func TestNoteHTMLRendersBasicMarkdown(t *testing.T) {
	cases := []struct {
		name string
		note string
		want []string // substrings that must appear in the rendered HTML
	}{
		{"bold", "this is **bold** text", []string{"<strong>bold</strong>"}},
		{"italic", "this is *italic* text", []string{"<em>italic</em>"}},
		{"code", "run `go test`", []string{"<code>go test</code>"}},
		{"link", "see [the docs](https://example.com)", []string{`<a href="https://example.com">the docs</a>`}},
		{"list", "- first\n- second", []string{"<li>first</li>", "<li>second</li>"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := noteHTML(tc.note)
			if err != nil {
				t.Fatalf("noteHTML: %v", err)
			}
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("noteHTML(%q) = %q, want it to contain %q", tc.note, got, want)
				}
			}
		})
	}
}

// TestNoteHTMLEscapesRawHTML confirms a caption containing literal HTML
// (a user pasting something that looks like a tag, not necessarily
// malicious) never becomes live markup in the exported slide deck —
// goldmark's default (no WithUnsafe()) escapes it, and this test is what
// actually pins that behavior rather than trusting the library default
// silently.
func TestNoteHTMLEscapesRawHTML(t *testing.T) {
	note := "click <script>alert(1)</script> here"
	got, err := noteHTML(note)
	if err != nil {
		t.Fatalf("noteHTML: %v", err)
	}
	if strings.Contains(got, "<script>") {
		t.Errorf("noteHTML(%q) rendered a live <script> tag: %q", note, got)
	}
}

func TestNoteHTMLEmptyNote(t *testing.T) {
	got, err := noteHTML("")
	if err != nil {
		t.Fatalf("noteHTML: %v", err)
	}
	if strings.TrimSpace(got) != "" {
		t.Errorf("noteHTML(\"\") = %q, want empty", got)
	}
}

// newTestPDF builds a minimal, ready-to-write Fpdf instance the same way
// PDF() does (margins, a page, a starting font) — writeNotePDF assumes a
// page already exists and a font is already selected, same as its real
// caller (renderWindowStep) guarantees.
func newTestPDF(t *testing.T) *fpdf.Fpdf {
	t.Helper()
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pageMarginMM, pageMarginMM, pageMarginMM)
	pdf.SetAutoPageBreak(true, pageMarginMM)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 11)
	return pdf
}

// TestWriteNotePDFDoesNotErrorOnEdgeCases is a robustness sweep: every
// case here is plausible caption content (including the exact "literal
// angle brackets" scenario the writeNotePDF doc comment calls out —
// e.g. "if x < 5" — which is exactly the input that would break a
// naive HTML-string-round-trip approach) and none of them should ever
// produce an fpdf internal error or panic.
func TestWriteNotePDFDoesNotErrorOnEdgeCases(t *testing.T) {
	cases := []string{
		"",
		"plain text, no markdown at all",
		"**bold** and *italic* and `code` together",
		"nested **bold with *italic* inside** text",
		"a lone asterisk * with no matching pair",
		"a caption with literal angle brackets: if x < 5 and y > 3",
		"a caption with a stray < and a much later > separated by lots of text in between them just to be sure",
		"[a link](https://example.com/path?query=1&other=2)",
		"- item one\n- item two\n- item three",
		"multiple\n\nparagraphs\n\nof text",
		"unicode: café, 日本語, emoji 🎉",
		strings.Repeat("long caption text that needs to wrap across multiple lines in the pdf output. ", 20),
	}
	for _, note := range cases {
		t.Run(note[:min(20, len(note))], func(t *testing.T) {
			pdf := newTestPDF(t)
			startY := pdf.GetY()
			writeNotePDF(pdf, note)
			if err := pdf.Error(); err != nil {
				t.Fatalf("writeNotePDF(%q) left pdf in error state: %v", note, err)
			}
			if note != "" && pdf.GetY() <= startY && pdf.PageNo() == 1 {
				t.Errorf("writeNotePDF(%q) didn't appear to advance the cursor", note)
			}
			var buf strings.Builder
			if err := pdf.Output(&buf); err != nil {
				t.Fatalf("writeNotePDF(%q): final PDF failed to output: %v", note, err)
			}
			if buf.Len() == 0 {
				t.Errorf("writeNotePDF(%q): output PDF is empty", note)
			}
		})
	}
}

// TestWriteNotePDFRestoresFont confirms bold/italic/code styling from
// inside a caption never leaks into whatever gets rendered after it —
// every caller renders a caption and then keeps going with plain text
// (the next step's heading, etc.), so writeNotePDF must always leave the
// font exactly as it found it (Helvetica, regular, base size). fpdf
// doesn't expose a public getter for the current family/style, so this
// compares GetStringWidth for the same string against a fresh instance
// explicitly set to Helvetica/regular/11 — family and style each affect
// glyph widths differently (bold is wider, Courier is fixed-width), so
// an exact width match is strong evidence the font actually matches,
// not just the point size.
func TestWriteNotePDFRestoresFont(t *testing.T) {
	const probe = "The quick brown fox — a representative sample string."

	pdf := newTestPDF(t)
	wantR, wantG, wantB := pdf.GetTextColor()
	writeNotePDF(pdf, "**bold** *italic* `code` [link](https://x.com)")
	if ptSize, _ := pdf.GetFontSize(); ptSize != 11.0 {
		t.Errorf("after writeNotePDF, font size = %v, want 11", ptSize)
	}
	if r, g, b := pdf.GetTextColor(); r != wantR || g != wantG || b != wantB {
		t.Errorf("after writeNotePDF, text color = (%d,%d,%d), want (%d,%d,%d) — the link color must not leak into whatever renders next", r, g, b, wantR, wantG, wantB)
	}
	gotWidth := pdf.GetStringWidth(probe)

	reference := newTestPDF(t) // already Helvetica/regular/11, untouched
	wantWidth := reference.GetStringWidth(probe)

	if gotWidth != wantWidth {
		t.Errorf("after writeNotePDF, GetStringWidth(%q) = %v, want %v (Helvetica/regular/11, matching a font that was never touched)", probe, gotWidth, wantWidth)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
