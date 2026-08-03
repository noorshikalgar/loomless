package export

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/go-pdf/fpdf"
)

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
	if err := goldmark.Convert([]byte(note), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
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
func writeNotePDF(pdf *fpdf.Fpdf, note string) {
	const lineHt = 5.5
	const baseSize = 11.0

	src := []byte(note)
	doc := goldmark.DefaultParser().Parse(text.NewReader(src))

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
