package export

import (
	"encoding/base64"
	"fmt"
	"html"
	"strings"

	"github.com/noorshikalgar/story/internal/session"
)

// Slides renders a session as a single self-contained HTML slide deck —
// one step per slide, step-through via arrow keys or on-screen buttons.
// "Self-contained" is deliberate: no CDN dependency (e.g. reveal.js from
// a CDN, which the technical-considerations doc mentions as an option) —
// this project is local-first (decision #3), and a slide deck that only
// renders with an internet connection undermines that. Screenshots are
// embedded as base64 data URIs so the single .html file is the whole
// deliverable — no separate images/ folder needs to travel with it,
// unlike the Markdown export's relative links.
func Slides(s *session.Session, dir string) ([]byte, error) {
	var b strings.Builder

	b.WriteString("<!doctype html>\n<html><head><meta charset=\"utf-8\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(s.Name))
	b.WriteString(slidesCSS)
	b.WriteString("</head><body>\n<div class=\"deck\">\n")

	b.WriteString("<section class=\"slide active\">\n")
	fmt.Fprintf(&b, "<h1>%s</h1>\n", html.EscapeString(s.Name))
	subtitle := "Captured " + s.StartedAt.Format("2006-01-02 15:04")
	if s.StoppedAt != nil {
		subtitle += " – " + s.StoppedAt.Format("15:04")
	}
	fmt.Fprintf(&b, "<p class=\"subtitle\">%s</p>\n", html.EscapeString(subtitle))
	steps := s.VisibleSteps()
	fmt.Fprintf(&b, "<p class=\"hint\">%d step(s) — use arrow keys or the buttons below</p>\n", len(steps))
	b.WriteString("</section>\n")

	for _, step := range steps {
		b.WriteString("<section class=\"slide\">\n")
		fmt.Fprintf(&b, "<h2>%s</h2>\n", html.EscapeString(stepHeading(step)))

		if step.Command != "" {
			fmt.Fprintf(&b, "<pre class=\"cmd\">$ %s</pre>\n", html.EscapeString(step.Command))
			if step.Output != "" {
				fmt.Fprintf(&b, "<pre class=\"output\">%s</pre>\n", html.EscapeString(step.Output))
			}
			if step.ExitCode != nil && *step.ExitCode != 0 {
				fmt.Fprintf(&b, "<p class=\"exit\">Exited %d.</p>\n", *step.ExitCode)
			}
		} else {
			if step.WindowApp != "" {
				windowLine := step.WindowApp
				if step.WindowTitle != "" {
					windowLine += " — " + step.WindowTitle
				}
				fmt.Fprintf(&b, "<p class=\"window\">%s</p>\n", html.EscapeString(windowLine))
			}
			if step.Note != "" {
				fmt.Fprintf(&b, "<p class=\"note\">%s</p>\n", html.EscapeString(step.Note))
			}
			if step.ImageFile != "" {
				if dataURI, err := imageDataURI(dir, step); err == nil {
					fmt.Fprintf(&b, "<img src=\"%s\" alt=\"Step %d screenshot\">\n", dataURI, step.Index)
				} else {
					fmt.Fprintf(&b, "<p class=\"exit\">[screenshot missing: %s]</p>\n", html.EscapeString(step.ImageFile))
				}
			}
		}
		b.WriteString("</section>\n")
	}

	b.WriteString("</div>\n")
	b.WriteString(slidesJS)
	b.WriteString("</body></html>\n")

	return []byte(b.String()), nil
}

func imageDataURI(dir string, step session.Step) (string, error) {
	data, err := stepImageBytes(dir, step)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data), nil
}

const slidesCSS = `<style>
  :root { color-scheme: dark; }
  * { box-sizing: border-box; }
  body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #1a1a1e; color: #f0f0f0; }
  .deck { position: relative; width: 100vw; height: 100vh; overflow: hidden; }
  .slide { position: absolute; inset: 0; display: none; flex-direction: column; align-items: center; justify-content: center; padding: 6vh 8vw; text-align: center; overflow-y: auto; }
  .slide.active { display: flex; }
  h1 { font-size: 2.5rem; margin-bottom: 0.5rem; }
  h2 { font-size: 1.8rem; margin-bottom: 1rem; color: #8ab4ff; }
  .subtitle { color: #aaa; font-style: italic; }
  .hint { color: #777; margin-top: 2rem; font-size: 0.9rem; }
  .window { font-weight: 600; color: #8ab4ff; margin-bottom: 0.5rem; }
  .note { font-size: 1.1rem; max-width: 60ch; margin-bottom: 1.5rem; }
  .exit { color: #ff8a8a; font-weight: 600; }
  img { max-width: 85vw; max-height: 60vh; border-radius: 6px; box-shadow: 0 4px 24px rgba(0,0,0,0.5); }
  pre { max-width: 85vw; overflow-x: auto; text-align: left; background: #26262b; padding: 1rem; border-radius: 6px; font-size: 0.9rem; }
  pre.cmd { color: #7ee787; }
  pre.output { color: #d0d0d0; }
  .nav { position: fixed; bottom: 2vh; left: 50%; transform: translateX(-50%); display: flex; gap: 1rem; align-items: center; z-index: 10; }
  .nav button { background: #2a2a30; color: #f0f0f0; border: 1px solid #444; border-radius: 4px; padding: 0.5rem 1rem; cursor: pointer; font-size: 1rem; }
  .nav button:hover { background: #3a3a42; }
  .nav button:disabled { opacity: 0.3; cursor: default; }
  .counter { color: #999; font-size: 0.9rem; min-width: 4rem; text-align: center; }
</style>
`

const slidesJS = `<div class="nav">
  <button id="prevBtn">&larr; Prev</button>
  <span class="counter" id="counter"></span>
  <button id="nextBtn">Next &rarr;</button>
</div>
<script>
  var slides = document.querySelectorAll('.slide');
  var idx = 0;
  function storyRender() {
    slides.forEach(function(s, i) { s.classList.toggle('active', i === idx); });
    document.getElementById('counter').textContent = (idx + 1) + ' / ' + slides.length;
    document.getElementById('prevBtn').disabled = idx === 0;
    document.getElementById('nextBtn').disabled = idx === slides.length - 1;
  }
  function storyGo(delta) {
    idx = Math.max(0, Math.min(slides.length - 1, idx + delta));
    storyRender();
  }
  document.getElementById('prevBtn').addEventListener('click', function() { storyGo(-1); });
  document.getElementById('nextBtn').addEventListener('click', function() { storyGo(1); });
  document.addEventListener('keydown', function(e) {
    if (e.key === 'ArrowRight' || e.key === ' ') storyGo(1);
    if (e.key === 'ArrowLeft') storyGo(-1);
  });
  storyRender();
</script>
`
