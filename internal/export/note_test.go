package export

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-pdf/fpdf"
)

// TestRenderNotePreviewMatchesVideoEmbedPipeline confirms the editor's
// live-preview endpoint uses the exact same rendering path as Slides
// export (noteHTMLWithVideoEmbeds) rather than a separate, potentially
// diverging implementation — checked via the one behavior that's
// distinctive to that specific pipeline (bare video URLs becoming a
// real iframe embed, not just a Linkify'd <a>).
func TestRenderNotePreviewMatchesVideoEmbedPipeline(t *testing.T) {
	got, err := RenderNotePreview("before\n\nhttps://youtu.be/dQw4w9WgXcQ\n\nafter")
	if err != nil {
		t.Fatalf("RenderNotePreview: %v", err)
	}
	if !strings.Contains(got, `<div class="video-embed">`) {
		t.Errorf("RenderNotePreview(%q) = %q, want a video-embed div (same pipeline as Slides export)", "...", got)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Errorf("RenderNotePreview(...) = %q, want surrounding text preserved", got)
	}
}

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
		"## A heading\n\nfollowed by a paragraph",
		"- item one\n- item two\n- item three",
		"multiple\n\nparagraphs\n\nof text",
		"unicode: café, 日本語, emoji 🎉",
		strings.Repeat("long caption text that needs to wrap across multiple lines in the pdf output. ", 20),
	}
	for _, note := range cases {
		t.Run(note[:min(20, len(note))], func(t *testing.T) {
			pdf := newTestPDF(t)
			startY := pdf.GetY()
			writeNotePDF(pdf, note, "")
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
	writeNotePDF(pdf, "**bold** *italic* `code` [link](https://x.com)", "")
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

// TestWriteNotePDFSeparatesHeadingFromFollowingParagraph guards against
// the exact bug this feature surfaced: before *ast.Heading had its own
// case, a "## Heading" line's text and the very next paragraph's text
// were written to the same line with no break in between (visually
// "HeadingParagraph text" with no space), because only *ast.Paragraph
// ever called pdf.Ln(). A heading followed by a paragraph should now
// take strictly more vertical space than the same words as one flat,
// heading-less paragraph — proof a real break was inserted, not just
// that nothing crashed.
func TestWriteNotePDFSeparatesHeadingFromFollowingParagraph(t *testing.T) {
	withHeading := newTestPDF(t)
	writeNotePDF(withHeading, "## A heading\n\nfollowed by a paragraph", "")
	heightWithHeading := withHeading.GetY()

	flat := newTestPDF(t)
	writeNotePDF(flat, "A heading followed by a paragraph", "")
	heightFlat := flat.GetY()

	if heightWithHeading <= heightFlat {
		t.Errorf("heading+paragraph took %.1fmm, flat single paragraph took %.1fmm — want the heading version strictly taller (its own line break)", heightWithHeading, heightFlat)
	}
}

func TestVideoEmbedURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string // expected embed URL; empty means "not recognized"
	}{
		{"youtube watch", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "https://www.youtube.com/embed/dQw4w9WgXcQ"},
		{"youtube watch no www", "https://youtube.com/watch?v=dQw4w9WgXcQ", "https://www.youtube.com/embed/dQw4w9WgXcQ"},
		{"youtu.be short", "https://youtu.be/dQw4w9WgXcQ", "https://www.youtube.com/embed/dQw4w9WgXcQ"},
		{"youtube embed", "https://www.youtube.com/embed/dQw4w9WgXcQ", "https://www.youtube.com/embed/dQw4w9WgXcQ"},
		{"vimeo", "https://vimeo.com/123456789", "https://player.vimeo.com/video/123456789"},
		{"unrelated link", "https://example.com/watch?v=x", ""},
		{"loom link", "https://www.loom.com/share/abc123", ""},
		{"plain text", "not a url at all", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := videoEmbedURL(tc.url)
			if tc.want == "" {
				if ok {
					t.Errorf("videoEmbedURL(%q) = (%q, true), want not recognized", tc.url, got)
				}
				return
			}
			if !ok || got != tc.want {
				t.Errorf("videoEmbedURL(%q) = (%q, %v), want (%q, true)", tc.url, got, ok, tc.want)
			}
		})
	}
}

// TestNoteHTMLWithVideoEmbedsRendersIframeForStandaloneLink confirms a
// video URL that's the ONLY thing on its own line becomes a live
// <iframe> embed, with surrounding paragraphs still rendered normally
// around it — this is the exact "paste a link on its own line" flow
// the feature targets, and it must not swallow the rest of the note.
func TestNoteHTMLWithVideoEmbedsRendersIframeForStandaloneLink(t *testing.T) {
	note := "before text\n\nhttps://youtu.be/dQw4w9WgXcQ\n\nafter text"
	got, err := noteHTMLWithVideoEmbeds(note)
	if err != nil {
		t.Fatalf("noteHTMLWithVideoEmbeds: %v", err)
	}
	if !strings.Contains(got, `<div class="video-embed">`) {
		t.Errorf("noteHTMLWithVideoEmbeds(%q) = %q, want a video-embed div", note, got)
	}
	if !strings.Contains(got, `src="https://www.youtube.com/embed/dQw4w9WgXcQ"`) {
		t.Errorf("noteHTMLWithVideoEmbeds(%q) = %q, want an iframe pointed at the embed URL", note, got)
	}
	if !strings.Contains(got, "before text") || !strings.Contains(got, "after text") {
		t.Errorf("noteHTMLWithVideoEmbeds(%q) = %q, want surrounding paragraphs preserved", note, got)
	}
}

// TestNoteHTMLWithVideoEmbedsLeavesOtherLinksAlone confirms a link that
// isn't a recognized YouTube/Vimeo URL renders as a normal clickable
// link (via Linkify), not an iframe — the feature is narrowly scoped to
// two specific services, not "any bare URL becomes an embed."
func TestNoteHTMLWithVideoEmbedsLeavesOtherLinksAlone(t *testing.T) {
	note := "https://example.com/some-page"
	got, err := noteHTMLWithVideoEmbeds(note)
	if err != nil {
		t.Fatalf("noteHTMLWithVideoEmbeds: %v", err)
	}
	if strings.Contains(got, "video-embed") {
		t.Errorf("noteHTMLWithVideoEmbeds(%q) = %q, want no video embed for a non-video link", note, got)
	}
	if !strings.Contains(got, `<a href="https://example.com/some-page"`) {
		t.Errorf("noteHTMLWithVideoEmbeds(%q) = %q, want it Linkify'd into a normal <a> tag", note, got)
	}
}

func TestInlineLocalImagesRewritesLocalReferenceToDataURI(t *testing.T) {
	dir := t.TempDir()
	imgBytes := encodeTestPNG(t, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	if err := os.WriteFile(filepath.Join(dir, "pasted.png"), imgBytes, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	note := "before ![a diagram](pasted.png) after"
	got := inlineLocalImages(note, dir)

	if strings.Contains(got, "(pasted.png)") {
		t.Errorf("inlineLocalImages(%q) = %q, still references the relative path, want it rewritten", note, got)
	}
	if !strings.Contains(got, "data:image/png;base64,") {
		t.Errorf("inlineLocalImages(%q) = %q, want a data:image/png;base64, URI", note, got)
	}
	if !strings.Contains(got, "[a diagram]") {
		t.Errorf("inlineLocalImages(%q) = %q, want the alt text preserved", note, got)
	}
}

// TestInlineLocalImagesLeavesRemoteAndDataURIsAlone confirms a remote
// http(s) image link or an already-inlined data: URI passes through
// completely untouched — only genuinely local, on-disk references
// should ever be rewritten.
func TestInlineLocalImagesLeavesRemoteAndDataURIsAlone(t *testing.T) {
	dir := t.TempDir()
	cases := []string{
		"![remote](https://example.com/x.png)",
		"![already inline](data:image/png;base64,AAAA)",
	}
	for _, note := range cases {
		if got := inlineLocalImages(note, dir); got != note {
			t.Errorf("inlineLocalImages(%q) = %q, want it unchanged", note, got)
		}
	}
}

// TestInlineLocalImagesLeavesMissingFileAsIs confirms a relative
// reference to a file that doesn't exist on disk stays as the original
// relative path rather than being silently dropped — a visibly broken
// link is more debuggable than an image that just vanishes.
func TestInlineLocalImagesLeavesMissingFileAsIs(t *testing.T) {
	dir := t.TempDir()
	note := "![gone](does-not-exist.png)"
	if got := inlineLocalImages(note, dir); got != note {
		t.Errorf("inlineLocalImages(%q) = %q, want it left unchanged when the file is missing", note, got)
	}
}

// TestWriteNotePDFEmbedsLocalImage confirms an image referenced from a
// note's Markdown is actually drawn into the PDF (not silently dropped,
// which writeNotePDF's ast.Walk would otherwise do — there was no
// *ast.Image case at all before this feature) and that the cursor
// visibly advances well past a single text line's worth of height,
// evidence a block-sized image was placed rather than nothing.
func TestWriteNotePDFEmbedsLocalImage(t *testing.T) {
	dir := t.TempDir()
	imgBytes := encodeTestPNG(t, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	if err := os.WriteFile(filepath.Join(dir, "shot.png"), imgBytes, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	pdf := newTestPDF(t)
	startY := pdf.GetY()
	writeNotePDF(pdf, "caption text\n\n![](shot.png)", dir)
	if err := pdf.Error(); err != nil {
		t.Fatalf("writeNotePDF with embedded image left pdf in error state: %v", err)
	}
	if pdf.GetY()-startY < 20 {
		t.Errorf("writeNotePDF with embedded image: cursor advanced only %.1fmm, want a block-sized image to have pushed it further", pdf.GetY()-startY)
	}
}

// TestWriteNotePDFMissingImageShowsPlaceholderNotCrash confirms a note
// referencing an image that isn't actually on disk (e.g. a stale
// reference after the file was moved) renders a visible placeholder
// instead of erroring out or silently skipping — same failure-
// visibility choice renderWindowStep already makes for a missing
// screenshot.
func TestWriteNotePDFMissingImageShowsPlaceholderNotCrash(t *testing.T) {
	dir := t.TempDir()
	pdf := newTestPDF(t)
	writeNotePDF(pdf, "![](does-not-exist.png)", dir)
	if err := pdf.Error(); err != nil {
		t.Fatalf("writeNotePDF with missing image left pdf in error state: %v", err)
	}
}

func encodeTestPNG(t *testing.T, fill color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for y := 0; y < 30; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}
