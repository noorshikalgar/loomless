package export

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"

	"github.com/go-pdf/fpdf"
)

// mdParser is shared by noteHTML and writeNotePDF's own goldmark
// parsing. Linkify (a GFM extension) is the one addition over
// goldmark's CommonMark default: a caption is free-typed, and someone
// pasting a bare "https://…" URL on its own line — the natural way to
// share a link or a video — expects it to end up clickable without
// also having to know CommonMark's `<...>` autolink bracket syntax.
var mdParser = goldmark.New(goldmark.WithExtensions(extension.Linkify))

// A step's Note (caption) is free-typed by a human and, per direct user
// request, supports Markdown — bold/italic/inline code/links/simple
// bullet lists at minimum. Markdown export needs no extra work: Note is
// already written into the .md file as raw text, so any Markdown syntax
// in it already renders correctly in any Markdown viewer. PDF and Slides
// (HTML) previously just escaped Note as plain text, so this file adds
// real rendering for both, parsed once via goldmark (CommonMark-
// compliant, pure Go, no cgo — consistent with every other dependency
// this project uses) rather than a hand-rolled parser that would
// inevitably get escaping/nesting edge cases wrong.

// noteHTML renders a caption's Markdown to HTML for the Slides exporter.
// goldmark escapes raw HTML in the source by default (no WithUnsafe()),
// so a caption containing literal "<script>" or similar renders as inert
// text, not live markup — the same safety property html.EscapeString had
// before, just no longer at the cost of ignoring Markdown syntax too.
func noteHTML(note string) (string, error) {
	var buf bytes.Buffer
	if err := mdParser.Convert([]byte(note), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// videoLinePattern matches a line that is NOTHING BUT a URL, optionally
// wrapped in CommonMark's `<...>` autolink brackets — the same
// "paste a link on its own line and get a live preview" convention
// tools like Slack/Notion use, chosen over trying to also recognize
// video links inside `[text](url)` markdown syntax or mid-sentence,
// which would make "is this line just a link" a much fuzzier question
// than it needs to be for a first version of this feature.
var videoLinePattern = regexp.MustCompile(`^<?(\S+)>?$`)

// noteHTMLWithVideoEmbeds is noteHTML, upgraded: any line that's
// nothing but a recognized YouTube/Vimeo URL (see videoEmbedURL)
// renders as a live embedded player instead of a plain link — used
// only by the Slides exporter, the one output format actually capable
// of hosting a real <iframe>. Markdown and PDF export deliberately
// don't get this treatment: a static PDF or a plain-text .md file
// structurally can't play a video, so the same line there just stays a
// normal (Linkify-clickable, for PDF) link — degrading gracefully
// rather than silently doing nothing with it.
// RenderNotePreview renders a caption/content-step body's Markdown to
// HTML for the editor's own live preview panel — exported specifically
// for internal/editor to call (everything else in this file is
// package-private). Reuses noteHTMLWithVideoEmbeds as-is: a preview
// shown inside the still-running local editor server can reference a
// content step's uploaded image by its ordinary relative path (the
// editor already serves it at that exact URL via GET /images/{file}),
// unlike the Slides exporter's noteHTMLWithVideoEmbeds call site, which
// first runs inlineLocalImages because a self-contained EXPORTED .html
// file won't have that server around to ask.
func RenderNotePreview(note string) (string, error) {
	return noteHTMLWithVideoEmbeds(note)
}

func noteHTMLWithVideoEmbeds(note string) (string, error) {
	lines := strings.Split(note, "\n")
	var out bytes.Buffer
	var block []string

	flush := func() error {
		if len(block) == 0 {
			return nil
		}
		var buf bytes.Buffer
		if err := mdParser.Convert([]byte(strings.Join(block, "\n")), &buf); err != nil {
			return err
		}
		out.Write(buf.Bytes())
		block = block[:0]
		return nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := videoLinePattern.FindStringSubmatch(trimmed); m != nil {
			if embedURL, ok := videoEmbedURL(m[1]); ok {
				if err := flush(); err != nil {
					return "", err
				}
				fmt.Fprintf(&out, `<div class="video-embed"><iframe src="%s" loading="lazy" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe></div>`, htmlAttrEscape(embedURL))
				continue
			}
		}
		block = append(block, line)
	}
	if err := flush(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// localImagePattern matches Markdown image syntax, capturing the alt
// text and the path/URL separately so inlineLocalImages can leave a
// remote or already-data: URI alone and only rewrite genuinely local
// references.
var localImagePattern = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)\)`)

// inlineLocalImages rewrites Markdown image references that point at a
// file in the session directory (as opposed to an http(s) URL or an
// already-inlined data: URI) into a base64 data: URI, before the note
// is handed to goldmark for the Slides exporter. This exists because
// Slides is deliberately self-contained (see Slides' own doc comment:
// screenshots are embedded, not linked, so a single .html file is the
// whole deliverable) — an image a user pasted into a content step's
// Markdown body via a relative filename would otherwise be a broken
// link the moment that .html file is opened from anywhere other than
// right next to the session folder. Markdown export needs no
// equivalent treatment: relative image links are exactly how it
// already references every captured screenshot (see stepImagePath).
// A reference that can't be read from disk is left as-is rather than
// dropped — a visibly broken relative link in the output is more
// debuggable than an image that silently vanished.
func inlineLocalImages(note, dir string) string {
	return localImagePattern.ReplaceAllStringFunc(note, func(match string) string {
		sub := localImagePattern.FindStringSubmatch(match)
		alt, path := sub[1], sub[2]
		if strings.Contains(path, "://") || strings.HasPrefix(path, "data:") {
			return match
		}
		data, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			return match
		}
		mimeType := http.DetectContentType(data)
		return "![" + alt + "](data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data) + ")"
	})
}

// htmlAttrEscape escapes a value for safe use inside a double-quoted
// HTML attribute — narrower than html.EscapeString's full text-node
// escaping (no need to escape '<'/'>' here), but embedURL is always
// server-constructed from a regex-matched ID (see videoEmbedURL), never
// echoed from arbitrary user text, so this is a defense-in-depth
// measure rather than the only thing standing between user input and
// the page.
func htmlAttrEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// writeNotePDF renders a caption's Markdown directly into pdf using
// fpdf's core text primitives (Write/WriteLinkString/SetFont), walking
// goldmark's parsed AST rather than round-tripping through an HTML
// string — fpdf's own HTMLBasicType.Write re-tokenizes its input with a
// regex that has no entity-decoding step, so a caption containing a
// literal "<" or ">" (very plausible in a dev tool, e.g. "if x < 5")
// would either misrender as literal "&lt;" or, worse, get misread as an
// unclosed tag and silently swallow everything up to the next ">".
// Walking the AST and calling fpdf primitives directly sidesteps that
// whole class of problem by construction: no HTML string is ever
// constructed or re-parsed, so there is nothing for stray angle brackets
// to be misinterpreted as. Restores the font to Helvetica/regular/base
// size on the way out, matching what every caller already expects after
// rendering a text block.
func writeNotePDF(pdf *fpdf.Fpdf, note, dir string) {
	const lineHt = 5.5
	const baseSize = 11.0

	src := []byte(note)
	doc := mdParser.Parser().Parse(text.NewReader(src))

	boldDepth, italicDepth, codeDepth := 0, 0, 0
	var linkDest string
	textR, textG, textB := pdf.GetTextColor()

	applyFont := func() {
		style := ""
		if boldDepth > 0 {
			style += "B"
		}
		if italicDepth > 0 {
			style += "I"
		}
		family := "Helvetica"
		if codeDepth > 0 {
			// Courier is a core PDF font (no embedding needed, same as
			// the terminal-step renderer already uses) — a real
			// monospace switch for inline code, not an approximation.
			family = "Courier"
		}
		pdf.SetFont(family, style, baseSize)
	}
	applyFont()

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		switch node := n.(type) {
		case *ast.Paragraph:
			if !entering {
				pdf.Ln(lineHt * 1.6)
			}
		case *ast.Heading:
			// Rendered bold (reusing the exact same boldDepth mechanism
			// as **bold** text) with its own trailing line break, not
			// left to fall through to *ast.Text with no special casing
			// at all — before this case existed, a heading's text and
			// the paragraph right after it ran together on one line with
			// no separator, since only *ast.Paragraph ever called Ln().
			// Not scaling font size per heading level (##  vs ###) is a
			// deliberate v1 scope line: distinguishing a heading from
			// body text at all was the actual defect; a full per-level
			// type scale is a visual nicety, not a correctness gap.
			if entering {
				boldDepth++
				applyFont()
			} else {
				boldDepth--
				applyFont()
				pdf.Ln(lineHt * 1.6)
			}
		case *ast.Emphasis:
			if node.Level >= 2 {
				if entering {
					boldDepth++
				} else {
					boldDepth--
				}
			} else {
				if entering {
					italicDepth++
				} else {
					italicDepth--
				}
			}
			applyFont()
		case *ast.CodeSpan:
			if entering {
				codeDepth++
			} else {
				codeDepth--
			}
			applyFont()
		case *ast.Link:
			if entering {
				linkDest = safe(string(node.Destination))
				pdf.SetTextColor(37, 99, 235) // legible link blue
			} else {
				linkDest = ""
				pdf.SetTextColor(textR, textG, textB)
			}
		case *ast.AutoLink:
			if entering {
				url := safe(string(node.URL(src)))
				pdf.SetTextColor(37, 99, 235)
				pdf.WriteLinkString(lineHt, url, url)
				pdf.SetTextColor(textR, textG, textB)
			}
		case *ast.ListItem:
			if entering {
				pdf.Write(lineHt, "- ")
			} else {
				pdf.Ln(lineHt)
			}
		case *ast.Image:
			if entering {
				pdf.Ln(lineHt * 0.5)
				// node.Destination is a file path, not display text —
				// deliberately NOT run through safe() (which mangles
				// non-ASCII into '?', fine for prose, wrong for a
				// filename that has to still resolve on disk).
				writeNoteImagePDF(pdf, dir, string(node.Destination))
				return ast.WalkSkipChildren, nil
			}
		case *ast.Text:
			if entering {
				t := safe(string(node.Segment.Value(src)))
				if t != "" {
					if linkDest != "" {
						pdf.WriteLinkString(lineHt, t, linkDest)
					} else {
						pdf.Write(lineHt, t)
					}
				}
				if node.SoftLineBreak() || node.HardLineBreak() {
					pdf.Write(lineHt, " ")
				}
			}
		}
		return ast.WalkContinue, nil
	})

	pdf.SetFont("Helvetica", "", baseSize)
	pdf.SetTextColor(textR, textG, textB)
}

// writeNoteImagePDF embeds an image referenced by a note's Markdown
// (e.g. one inserted via a content step's "Insert Image" button) as a
// real block image in the PDF, full note-width, scaled down to fit a
// single page's remaining height the same way renderWindowStep's own
// screenshot embed already does — a silently-dropped image would be a
// half-finished rendering of a note that visibly has one in the editor
// and in every other export format. relPath is resolved relative to
// dir (the session folder), matching how a content step's uploaded
// image is stored and referenced everywhere else. A relPath that can't
// be read leaves a visible "[image missing]" note rather than silently
// skipping it — same failure-visibility choice renderWindowStep makes
// for a missing screenshot.
func writeNoteImagePDF(pdf *fpdf.Fpdf, dir, relPath string) {
	data, err := os.ReadFile(filepath.Join(dir, relPath))
	if err != nil {
		pdf.SetFont("Helvetica", "I", 9)
		pdf.MultiCell(0, 6, safe(fmt.Sprintf("[image missing: %s]", relPath)), "", "L", false)
		pdf.SetFont("Helvetica", "", 11)
		return
	}
	imgType := "PNG"
	if ext := strings.ToLower(filepath.Ext(relPath)); ext == ".jpg" || ext == ".jpeg" {
		imgType = "JPG"
	}
	info := pdf.RegisterImageOptionsReader(relPath, fpdf.ImageOptions{ImageType: imgType}, bytes.NewReader(data))
	if info == nil {
		return
	}
	imgW, imgH := info.Extent()
	displayW := usableWidthMM
	displayH := usableWidthMM * imgH / imgW
	maxH := pageBottomMM - pageMarginMM
	if displayH > maxH {
		displayW = usableWidthMM * maxH / displayH
		displayH = maxH
	}
	if pdf.GetY()+displayH > pageBottomMM {
		pdf.AddPage()
	}
	pdf.ImageOptions(relPath, pdf.GetX(), pdf.GetY(), displayW, displayH, true, fpdf.ImageOptions{ImageType: imgType}, 0, "")
	pdf.SetY(pdf.GetY() + displayH + 2)
}
