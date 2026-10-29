package dashboard

import (
	"fmt"
	"html"
	"net/url"
	"strings"

	"github.com/noorshikalgar/loomless/internal/ui"
)

// Inline icons (24×24, currentColor) — inlined so they recolor with the
// theme and cost no extra requests.
const (
	icRecord  = `<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="6" fill="currentColor"/></svg>`
	icStop    = `<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="6.5" y="6.5" width="11" height="11" rx="2" fill="currentColor"/></svg>`
	icHome    = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round" aria-hidden="true"><path d="M4 10.5L12 4l8 6.5V20a1 1 0 01-1 1h-4.5v-6h-5v6H5a1 1 0 01-1-1z"/></svg>`
	icLibrary = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round" aria-hidden="true"><rect x="3.5" y="3.5" width="7" height="7" rx="1.5"/><rect x="13.5" y="3.5" width="7" height="7" rx="1.5"/><rect x="3.5" y="13.5" width="7" height="7" rx="1.5"/><rect x="13.5" y="13.5" width="7" height="7" rx="1.5"/></svg>`
	icEdit    = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 013 3L7 19l-4 1 1-4z"/></svg>`
	icFile    = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round" aria-hidden="true"><path d="M14 3H6a2 2 0 00-2 2v14a2 2 0 002 2h12a2 2 0 002-2V9z"/><path d="M14 3v6h6"/></svg>`
	icSlides  = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="4" width="18" height="13" rx="2"/><path d="M12 17v4M8 21h8"/></svg>`
	icFolder  = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round" aria-hidden="true"><path d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z"/></svg>`
	icRename  = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" aria-hidden="true"><path d="M4 7V5h16v2"/><path d="M12 5v14"/><path d="M9 19h6"/></svg>`
	icTrash   = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 7h16"/><path d="M10 11v6M14 11v6"/><path d="M6 7l1 12a2 2 0 002 2h6a2 2 0 002-2l1-12"/><path d="M9 7V4h6v3"/></svg>`
	icMore    = `<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><circle cx="5" cy="12" r="1.6"/><circle cx="12" cy="12" r="1.6"/><circle cx="19" cy="12" r="1.6"/></svg>`
	icSearch  = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5"/></svg>`
	icArrow   = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12h14M13 6l6 6-6 6"/></svg>`
)

// thumbPlaceholder is what a tile shows before a session has any
// screenshot: the brand's dashed-square motif, not a broken image.
const thumbPlaceholder = `<div class="thumb-empty" aria-hidden="true"><svg viewBox="0 0 120 72"><rect x="8" y="8" width="30" height="24" rx="5"/><rect x="45" y="8" width="30" height="24" rx="5"/><rect x="8" y="40" width="30" height="24" rx="5"/><rect x="45" y="40" width="30" height="24" rx="5"/><rect x="82" y="40" width="30" height="24" rx="5"/></svg><span>No screenshots yet</span></div>`

// startArt illustrates what a recording produces — numbered steps with
// screenshots — in the same dashed, annotated language as the icon.
const startArt = `<svg class="start-art" viewBox="0 0 340 240" aria-hidden="true">
  <rect class="a-win" x="10" y="10" width="320" height="220" rx="14"/>
  <circle class="a-dot" cx="32" cy="32" r="4"/><circle class="a-dot" cx="46" cy="32" r="4"/><circle class="a-dot" cx="60" cy="32" r="4"/>
  <g transform="translate(30 58)">
    <rect class="a-shot" x="26" y="0" width="96" height="44" rx="6"/>
    <rect class="a-line" x="138" y="10" width="138" height="7" rx="3.5"/><rect class="a-line dim" x="138" y="26" width="92" height="7" rx="3.5"/>
    <circle class="a-b1" cx="8" cy="22" r="9"/><text x="8" y="26">1</text>
  </g>
  <g transform="translate(30 116)">
    <rect class="a-shot" x="26" y="0" width="96" height="44" rx="6"/>
    <rect class="a-line" x="138" y="10" width="118" height="7" rx="3.5"/><rect class="a-line dim" x="138" y="26" width="132" height="7" rx="3.5"/>
    <circle class="a-b2" cx="8" cy="22" r="9"/><text x="8" y="26">2</text>
  </g>
  <g transform="translate(30 174)">
    <rect class="a-shot" x="26" y="0" width="96" height="44" rx="6"/>
    <rect class="a-line" x="138" y="10" width="146" height="7" rx="3.5"/><rect class="a-line dim" x="138" y="26" width="70" height="7" rx="3.5"/>
    <circle class="a-b3" cx="8" cy="22" r="9"/><text x="8" y="26">3</text>
  </g>
  <path class="a-cursor" d="M118 82l0 22 6-5 4 9 4-2-4-9 8 0z"/>
</svg>`

type page struct {
	title     string
	current   string // "/" or "/sessions"
	total     int
	recording []summary
	body      string
}

// shell renders the app frame shared by every page: sidebar (brand, new
// recording, navigation, live-recording indicator, appearance) and the
// main content column.
func shell(p page) string {
	navItem := func(href, label, icon, extra string) string {
		cur := ""
		if href == p.current {
			cur = ` aria-current="page"`
		}
		return `<a class="side-link" href="` + href + `"` + cur + `>` + icon + `<span>` + label + `</span>` + extra + `</a>`
	}
	var live strings.Builder
	if len(p.recording) > 0 {
		live.WriteString(`<div class="side-section"><div class="side-label">Recording</div>`)
		for _, s := range p.recording {
			fmt.Fprintf(&live, `<a class="side-live" href="/"><span class="rec-dot"></span><span class="side-live-name">%s</span><span class="mono" data-timer="%s">--:--</span></a>`,
				html.EscapeString(s.Name), s.StartedISO)
		}
		live.WriteString(`</div>`)
	}
	return `<!doctype html>
<html lang="en"` + ui.HTMLAttrs() + `><head><meta charset="utf-8">
<title>` + html.EscapeString(p.title) + ` · Loomless</title>
` + ui.Head() + `
<style>` + dashboardCSS + `</style>
</head><body>
<div class="app">
<aside class="sidebar">
  <a class="side-brand" href="/"><img src="/ui/icon.svg" alt="">Loomless</a>
  <a class="btn side-new" href="/#new" data-new>` + icRecord + `<span>New recording</span><kbd>N</kbd></a>
  <nav class="side-nav" aria-label="Main">
    ` + navItem("/", "Home", icHome, "") + `
    ` + navItem("/sessions", "Library", icLibrary, fmt.Sprintf(`<span class="side-count">%d</span>`, p.total)) + `
  </nav>
  ` + live.String() + `
  <div class="side-foot">
    <span class="side-foot-label">Appearance</span>
    ` + ui.ThemePicker() + `
  </div>
</aside>
<main class="main">
` + p.body + `
</main>
</div>
<script>` + dashboardJS + `</script>
</body></html>`
}

func homeHTML(recent []summary, total int, recording []summary) string {
	var b strings.Builder
	if len(recording) > 0 {
		s := recording[0]
		name := html.EscapeString(s.Name)
		thumb := `<div class="live-wait"><span class="rec-dot"></span>Waiting for the first click…</div>`
		if s.HasThumb {
			thumb = `<img data-live-thumb src="/thumb/` + url.PathEscape(s.Name) + `?pos=last&v=` + fmt.Sprint(s.Steps) + `" alt="Latest screenshot">`
		}
		fmt.Fprintf(&b, `<section class="live-panel" data-live data-name="%s" data-steps="%d">
  <div class="live-info">
    <div class="live-status"><span class="rec-dot"></span>Recording</div>
    <h1 class="live-title">%s</h1>
    <div class="live-stats">
      <div class="stat"><span class="stat-value mono" data-timer="%s">00:00</span><span class="stat-label">Elapsed</span></div>
      <div class="stat"><span class="stat-value mono" data-live-steps>%d</span><span class="stat-label">Steps</span></div>
      <div class="stat"><span class="stat-value mono" data-live-shots>%d</span><span class="stat-label">Screenshots</span></div>
    </div>
    <p class="live-hint">Work through the flow in any app — every click becomes a step with a screenshot.</p>
    <div class="row row-wrap">
      <button class="btn btn-danger btn-lg" data-action="stop" data-name="%s">%s Stop recording</button>
      <button class="btn btn-lg" data-action="open" data-name="%s">%s Open editor</button>
    </div>
  </div>
  <figure class="live-preview">%s<figcaption>Latest capture</figcaption></figure>
</section>`, name, s.Steps, name, s.StartedISO, s.Steps, s.Screenshots, name, icStop, name, icEdit, thumb)
	} else {
		b.WriteString(`<section class="start-panel" id="new">
  <div class="start-copy">
    <span class="eyebrow">New recording</span>
    <h1>What are you capturing?</h1>
    <p>Name it, click through the flow in any app, and Loomless writes the steps — screenshots and all.</p>
    <form id="newForm" class="start-form" autocomplete="off">
      <input type="text" id="newName" class="input input-lg" placeholder="e.g. checkout-expired-card" aria-label="Recording name" required>
      <button type="submit" class="btn btn-primary btn-lg">` + icRecord + ` Record</button>
    </form>
    <ol class="how">
      <li><b>1</b>Name it</li>
      <li><b>2</b>Click through the flow</li>
      <li><b>3</b>Edit &amp; export</li>
    </ol>
  </div>
  ` + startArt + `
</section>`)
	}

	b.WriteString(`<section class="section"><div class="section-head"><h2>Recent</h2>`)
	if total > 0 {
		b.WriteString(`<a class="link-arrow" href="/sessions">View library ` + icArrow + `</a>`)
	}
	b.WriteString(`</div>`)
	if len(recent) == 0 {
		b.WriteString(`<div class="empty-state"><h3>Nothing recorded yet</h3><p>Your recordings will show up here with a preview of each one.</p></div>`)
	} else {
		b.WriteString(`<div class="tile-grid">`)
		for _, s := range recent {
			b.WriteString(tile(s))
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</section>`)
	return shell(page{title: "Home", current: "/", total: total, recording: recording, body: b.String()})
}

func sessionsHTML(sums []summary) string {
	var recording []summary
	for _, s := range sums {
		if s.Recording {
			recording = append(recording, s)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<header class="page-head">
  <div><h1>Library</h1><p class="muted" data-count-label>%d recording%s</p></div>
  <div class="toolbar">
    <label class="search">%s<input type="search" class="input" data-search placeholder="Search" aria-label="Search recordings"><kbd>/</kbd></label>
    <select class="select" data-sort aria-label="Sort by">
      <option value="new">Newest first</option>
      <option value="old">Oldest first</option>
      <option value="name">Name</option>
      <option value="steps">Most steps</option>
    </select>
  </div>
</header>`, len(sums), plural(len(sums)), icSearch)
	if len(sums) == 0 {
		b.WriteString(`<div class="empty-state"><h3>No recordings yet</h3><p><a href="/#new">Start your first one</a> — it takes a name and a few clicks.</p></div>`)
	} else {
		b.WriteString(`<div class="tile-grid" data-grid>`)
		for _, s := range sums {
			b.WriteString(tile(s))
		}
		b.WriteString(`</div><div class="empty-state no-match" hidden><h3>No matches</h3><p>No recording name contains that text.</p></div>`)
	}
	return shell(page{title: "Library", current: "/sessions", total: len(sums), recording: recording, body: b.String()})
}

// tile renders one recording card: a screenshot preview you click to
// open, then the name, a one-line summary, and an actions menu. Export
// formats show up only once they exist, as small pills — unexported
// formats aren't worth a permanent slot of visual noise on every card.
func tile(s summary) string {
	name := html.EscapeString(s.Name)
	thumb := thumbPlaceholder
	if s.HasThumb {
		thumb = `<img src="/thumb/` + url.PathEscape(s.Name) + `" loading="lazy" alt="">`
	}
	rec := ""
	if s.Recording {
		rec = `<span class="tile-rec"><span class="rec-dot"></span>REC</span>`
	}
	var pills strings.Builder
	for _, p := range []struct {
		on           bool
		format, text string
	}{{s.ExportedMD, "md", "MD"}, {s.ExportedPDF, "pdf", "PDF"}, {s.ExportedSl, "slides", "Slides"}} {
		if p.on {
			fmt.Fprintf(&pills, `<button class="pill" data-action="reveal" data-format="%s" data-name="%s" title="Show %s in Finder">%s</button>`, p.format, name, p.text, p.text)
		}
	}
	meta := `<time datetime="` + s.StartedISO + `" title="` + html.EscapeString(s.Started) + `">` + html.EscapeString(s.Started) + `</time>`
	if s.Duration != "" {
		meta += `<span class="sep">·</span><span>` + s.Duration + `</span>`
	}
	danger := `<button class="menu-item danger" data-action="delete" data-name="` + name + `">` + icTrash + ` Delete…</button>`
	if s.Recording {
		danger = `<button class="menu-item danger" data-action="stop" data-name="` + name + `">` + icStop + ` Stop recording</button>`
	}
	return fmt.Sprintf(`<article class="tile" data-search="%s" data-name="%s" data-started="%s" data-steps="%d">
  <button class="tile-thumb" data-action="open" data-name="%s" aria-label="Open %s in the editor">
    %s
    %s
    <span class="tile-count mono">%d step%s</span>
    <span class="tile-open">%s Open editor</span>
  </button>
  <div class="tile-body">
    <div class="tile-head">
      <h3 class="tile-title" title="%s">%s</h3>
      <details class="menu">
        <summary class="btn btn-icon btn-ghost btn-sm" aria-label="Actions for %s" title="Actions">%s</summary>
        <div class="menu-list">
          <button class="menu-item" data-action="open" data-name="%s">%s Open editor</button>
          <div class="menu-sep"></div>
          <button class="menu-item" data-action="export" data-format="md" data-name="%s">%s Export Markdown</button>
          <button class="menu-item" data-action="export" data-format="pdf" data-name="%s">%s Export PDF</button>
          <button class="menu-item" data-action="export" data-format="slides" data-name="%s">%s Export slide deck</button>
          <div class="menu-sep"></div>
          <button class="menu-item" data-action="rename" data-name="%s">%s Rename…</button>
          <button class="menu-item" data-action="reveal" data-name="%s">%s Show in Finder</button>
          <div class="menu-sep"></div>
          %s
        </div>
      </details>
    </div>
    <div class="tile-meta">%s<span class="pills">%s</span></div>
  </div>
</article>`,
		html.EscapeString(strings.ToLower(s.Name)), name, s.StartedISO, s.Steps,
		name, name,
		thumb, rec, s.Steps, plural(s.Steps), icEdit,
		name, name,
		name, icMore,
		name, icEdit,
		name, icFile, name, icFile, name, icSlides,
		name, icRename, name, icFolder,
		danger,
		meta, pills.String())
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

const dashboardCSS = `
  /* ---------- app shell ---------- */
  .app { display: grid; grid-template-columns: 248px minmax(0, 1fr); min-height: 100vh; }
  .sidebar {
    position: sticky; top: 0; height: 100vh;
    display: flex; flex-direction: column; gap: var(--space-1);
    padding: var(--space-4) var(--space-3);
    background: var(--surface-inset); border-right: 1px solid var(--border-soft);
  }
  .side-brand {
    display: flex; align-items: center; gap: var(--space-2);
    padding: var(--space-1) var(--space-2) var(--space-4);
    color: var(--text); font-weight: var(--weight-bold); font-size: var(--text-lg); letter-spacing: var(--tracking-tight);
  }
  .side-brand:hover { text-decoration: none; }
  .side-brand img { width: 26px; height: 26px; border-radius: 7px; box-shadow: 0 0 0 1px var(--border); }
  .side-new { justify-content: flex-start; margin-bottom: var(--space-4); height: 2.25rem; background: var(--surface); box-shadow: var(--shadow-sm); }
  .side-new:hover { text-decoration: none; }
  .side-new span { flex: 1; text-align: left; }
  .side-new svg { width: 12px; height: 12px; color: var(--recording); }
  kbd {
    font-family: var(--font-mono); font-size: 0.6875rem; line-height: 1;
    padding: 3px 5px; border-radius: 4px;
    border: 1px solid currentColor; opacity: 0.55;
  }
  .side-nav { display: flex; flex-direction: column; gap: 2px; }
  .side-link {
    display: flex; align-items: center; gap: var(--space-3);
    height: 2rem; padding: 0 var(--space-2); border-radius: var(--radius);
    color: var(--text-soft); font-size: var(--text-sm); font-weight: var(--weight-medium);
  }
  .side-link svg { width: 17px; height: 17px; color: var(--text-muted); }
  .side-link span:first-of-type { flex: 1; }
  .side-link:hover { background: var(--surface-2); color: var(--text); text-decoration: none; }
  .side-link[aria-current="page"] { background: var(--surface-2); color: var(--text); }
  .side-link[aria-current="page"] svg { color: var(--accent); }
  .side-count { font-family: var(--font-mono); font-size: var(--text-xs); color: var(--text-muted); }
  .side-section { margin-top: var(--space-5); }
  .side-label { padding: 0 var(--space-2) var(--space-1); font-size: 0.6875rem; font-weight: var(--weight-semibold); letter-spacing: 0.06em; text-transform: uppercase; color: var(--text-muted); }
  .side-live {
    display: flex; align-items: center; gap: var(--space-2);
    height: 2rem; padding: 0 var(--space-2); border-radius: var(--radius);
    color: var(--text); font-size: var(--text-sm);
  }
  .side-live:hover { background: var(--surface-2); text-decoration: none; }
  .side-live-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .side-live .mono { font-size: var(--text-xs); color: var(--recording); }
  .side-foot {
    margin-top: auto; display: flex; align-items: center; justify-content: space-between;
    padding: var(--space-3) var(--space-2) 0; border-top: 1px solid var(--border-soft);
  }
  .side-foot-label { font-size: var(--text-xs); color: var(--text-muted); }

  .main { padding: var(--space-8) var(--space-10) var(--space-12); max-width: 1320px; width: 100%; }

  .rec-dot {
    width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0;
    background: var(--recording); box-shadow: 0 0 0 0 var(--recording);
    animation: rec-pulse 1.6s var(--ease) infinite;
  }
  @keyframes rec-pulse {
    0% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--recording) 55%, transparent); }
    70% { box-shadow: 0 0 0 6px transparent; }
    100% { box-shadow: 0 0 0 0 transparent; }
  }

  /* ---------- start panel ---------- */
  .start-panel {
    display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr); align-items: center; gap: var(--space-8);
    padding: var(--space-8) var(--space-8);
    background: var(--surface); border: 1px solid var(--border-soft); border-radius: 16px;
    box-shadow: var(--shadow-sm);
    background-image: radial-gradient(circle at 100% 0%, color-mix(in srgb, var(--accent) 9%, transparent), transparent 55%);
  }
  .eyebrow { display: inline-block; font-size: var(--text-xs); font-weight: var(--weight-semibold); letter-spacing: 0.06em; text-transform: uppercase; color: var(--accent); margin-bottom: var(--space-2); }
  .start-copy h1 { margin: 0 0 var(--space-2); font-size: 1.875rem; }
  .start-copy p { margin: 0 0 var(--space-6); color: var(--text-soft); max-width: 46ch; }
  .start-form { display: flex; gap: var(--space-2); }
  .start-form .input { flex: 1; min-width: 0; }
  .start-form .btn svg { width: 12px; height: 12px; }
  .how { display: flex; gap: var(--space-6); list-style: none; padding: 0; margin: var(--space-5) 0 0; font-size: var(--text-sm); color: var(--text-muted); flex-wrap: wrap; }
  .how li { display: flex; align-items: center; gap: var(--space-2); }
  .how b {
    display: inline-flex; align-items: center; justify-content: center; width: 20px; height: 20px;
    border-radius: 50%; font-size: 0.6875rem; font-family: var(--font-mono);
    border: 1px dashed var(--border); color: var(--text-soft);
  }
  .start-art { width: 100%; max-width: 380px; justify-self: end; }
  .a-win { fill: var(--surface-inset); stroke: var(--border); stroke-width: 1; }
  .a-dot { fill: var(--border); }
  .a-shot { fill: var(--surface-2); stroke: var(--border); stroke-dasharray: 5 4; }
  .a-line { fill: var(--border); }
  .a-line.dim { fill: var(--border-soft); }
  .a-b1 { fill: #FFB020; } .a-b2 { fill: #A855F7; } .a-b3 { fill: #FF3B30; }
  .start-art text { font-family: var(--font-mono); font-size: 10px; font-weight: 700; fill: #fff; text-anchor: middle; }
  .a-cursor { fill: var(--text); stroke: var(--surface); stroke-width: 1.5; }

  /* ---------- live panel ---------- */
  .live-panel {
    display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.1fr); gap: var(--space-8); align-items: center;
    padding: var(--space-8); border-radius: 16px;
    background: var(--surface); border: 1px solid color-mix(in srgb, var(--recording) 35%, var(--border-soft));
    box-shadow: var(--shadow-md);
  }
  .live-status { display: inline-flex; align-items: center; gap: var(--space-2); font-size: var(--text-xs); font-weight: var(--weight-semibold); letter-spacing: 0.06em; text-transform: uppercase; color: var(--recording); }
  .live-title { margin: var(--space-2) 0 var(--space-5); font-size: 1.875rem; word-break: break-word; }
  .live-stats { display: flex; gap: var(--space-8); margin-bottom: var(--space-5); }
  .stat { display: flex; flex-direction: column; gap: 2px; }
  .stat-value { font-size: 1.75rem; font-weight: var(--weight-semibold); letter-spacing: -0.02em; color: var(--text); font-variant-numeric: tabular-nums; }
  .stat-label { font-size: var(--text-xs); color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; }
  .live-hint { margin: 0 0 var(--space-5); color: var(--text-soft); font-size: var(--text-sm); }
  .live-preview { margin: 0; }
  .live-preview img, .live-wait {
    display: block; width: 100%; aspect-ratio: 16 / 10; object-fit: cover; object-position: top left;
    border-radius: var(--radius-lg); border: 1px solid var(--border-soft); background: var(--surface-inset);
  }
  .live-wait { display: flex; align-items: center; justify-content: center; gap: var(--space-2); color: var(--text-muted); font-size: var(--text-sm); }
  .live-preview figcaption { margin-top: var(--space-2); font-size: var(--text-xs); color: var(--text-muted); }

  /* ---------- sections, page header ---------- */
  .section { margin-top: var(--space-10); }
  .section-head { display: flex; align-items: baseline; justify-content: space-between; margin-bottom: var(--space-4); }
  .section-head h2 { margin: 0; font-size: var(--text-lg); }
  .link-arrow { display: inline-flex; align-items: center; gap: var(--space-1); font-size: var(--text-sm); font-weight: var(--weight-medium); }
  .link-arrow svg { width: 14px; height: 14px; }
  .page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: var(--space-4); flex-wrap: wrap; margin-bottom: var(--space-6); }
  .page-head h1 { margin: 0; }
  .page-head p { margin: var(--space-1) 0 0; font-size: var(--text-sm); }
  .toolbar { display: flex; gap: var(--space-2); align-items: center; }
  .search { position: relative; display: flex; align-items: center; }
  .search > svg { position: absolute; left: 10px; width: 15px; height: 15px; color: var(--text-muted); pointer-events: none; }
  .search .input { padding-left: 32px; padding-right: 30px; width: 260px; }
  .search kbd { position: absolute; right: 8px; color: var(--text-muted); }
  .search .input:focus + kbd { display: none; }

  /* ---------- tiles ---------- */
  .tile-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: var(--space-5); }
  .tile { display: flex; flex-direction: column; min-width: 0; }
  .tile-thumb {
    position: relative; display: block; width: 100%; padding: 0; overflow: hidden; cursor: pointer;
    aspect-ratio: 16 / 10; border-radius: 12px; border: 1px solid var(--border-soft);
    background: var(--surface-inset); font: inherit; color: inherit;
    transition: border-color var(--dur) var(--ease), box-shadow var(--dur) var(--ease), transform var(--dur) var(--ease);
  }
  .tile-thumb img {
    width: 100%; height: 100%; object-fit: cover; object-position: top left; display: block;
    transition: transform 400ms var(--ease);
  }
  .tile-thumb:hover { border-color: var(--border); box-shadow: var(--shadow-md); transform: translateY(-2px); }
  .tile-thumb:hover img { transform: scale(1.03); }
  .tile-open {
    position: absolute; inset: auto 10px 10px auto;
    display: inline-flex; align-items: center; gap: 6px; padding: 6px 10px;
    font-size: var(--text-xs); font-weight: var(--weight-medium);
    background: var(--surface); color: var(--text); border: 1px solid var(--border); border-radius: var(--radius);
    box-shadow: var(--shadow-md); opacity: 0; transform: translateY(4px);
    transition: opacity var(--dur-fast) var(--ease), transform var(--dur-fast) var(--ease);
  }
  .tile-open svg { width: 13px; height: 13px; }
  .tile-thumb:hover .tile-open, .tile-thumb:focus-visible .tile-open { opacity: 1; transform: none; }
  .tile-count {
    position: absolute; left: 10px; bottom: 10px; padding: 3px 8px;
    font-size: 0.6875rem; color: #fff; background: rgba(0, 0, 0, 0.6); border-radius: var(--radius-full);
    backdrop-filter: blur(6px);
  }
  .tile-rec {
    position: absolute; left: 10px; top: 10px; display: inline-flex; align-items: center; gap: 6px;
    padding: 3px 8px; font-size: 0.6875rem; font-weight: var(--weight-bold); letter-spacing: 0.06em;
    color: #fff; background: rgba(0, 0, 0, 0.65); border-radius: var(--radius-full); backdrop-filter: blur(6px);
  }
  .thumb-empty { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: var(--space-2); color: var(--text-muted); font-size: var(--text-xs); }
  .thumb-empty svg { width: 96px; }
  .thumb-empty rect { fill: none; stroke: var(--border); stroke-width: 1.5; stroke-dasharray: 4 3; }
  .tile-body { padding: var(--space-3) 2px 0; }
  .tile-head { display: flex; align-items: center; justify-content: space-between; gap: var(--space-2); }
  .tile-title { margin: 0; font-size: var(--text-base); font-weight: var(--weight-semibold); letter-spacing: -0.01em; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .tile-head .menu { margin-right: -6px; }
  .tile-meta { display: flex; align-items: center; gap: 6px; margin-top: 2px; font-size: var(--text-sm); color: var(--text-muted); min-height: 22px; }
  .tile-meta .sep { opacity: 0.6; }
  .pills { display: inline-flex; gap: 4px; margin-left: auto; }
  .pill {
    font: inherit; font-family: var(--font-mono); font-size: 0.6875rem; line-height: 1;
    padding: 4px 6px; border-radius: 5px; cursor: pointer;
    color: var(--success); background: color-mix(in srgb, var(--success) 12%, transparent); border: 0;
  }
  .pill:hover { background: color-mix(in srgb, var(--success) 22%, transparent); }
  .menu-item.danger, .menu-item.danger svg { color: var(--danger); }
  .tile.removing { opacity: 0; transform: scale(0.97); transition: opacity var(--dur) var(--ease), transform var(--dur) var(--ease); }

  /* ---------- responsive ---------- */
  @media (max-width: 980px) {
    .start-panel, .live-panel { grid-template-columns: 1fr; }
    .start-art { display: none; }
  }
  @media (max-width: 860px) {
    .app { grid-template-columns: 1fr; }
    .sidebar {
      position: static; height: auto; flex-direction: row; align-items: center; flex-wrap: wrap;
      gap: var(--space-2); padding: var(--space-3) var(--space-4);
      border-right: 0; border-bottom: 1px solid var(--border-soft);
    }
    .side-brand { padding: 0; margin-right: auto; }
    .side-new { order: 3; width: 100%; margin: 0; justify-content: center; }
    .side-new span { flex: 0; }
    .side-new kbd, .side-section, .side-foot-label { display: none; }
    .side-nav { flex-direction: row; order: 1; }
    .side-link { padding: 0 var(--space-2); }
    .side-link span, .side-count { display: none; }
    .side-foot { margin: 0; padding: 0; border: 0; order: 2; }
    .main { padding: var(--space-5) var(--space-4) var(--space-10); }
    .start-panel, .live-panel { padding: var(--space-5); }
    .start-form { flex-direction: column; }
    .start-form .input { flex: none; }
    .live-stats { gap: var(--space-5); }
    .toolbar, .search, .search .input { width: 100%; }
    .tile-grid { grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); }
  }
`

const dashboardJS = `
  function api(method, path, body) {
    return fetch(path, {
      method: method,
      headers: body ? {'Content-Type': 'application/json'} : undefined,
      body: body ? JSON.stringify(body) : undefined,
    }).then(function(r) {
      if (!r.ok) return r.text().then(function(t) { throw new Error((t || r.statusText).trim()); });
      return r;
    });
  }
  function apiJSON(method, path, body) {
    return api(method, path, body).then(function(r) { return r.json(); });
  }
  function sessionPath(name, suffix) {
    return '/api/sessions/' + encodeURIComponent(name) + '/' + suffix;
  }
  var FORMAT_LABEL = {md: 'Markdown', pdf: 'PDF', slides: 'Slide deck'};
  var FORMAT_PILL = {md: 'MD', pdf: 'PDF', slides: 'Slides'};

  function reveal(name, format) {
    return api('POST', sessionPath(name, 'reveal'), format ? {format: format} : {})
      .catch(function(e) { Loomless.toast(e.message || 'Could not open Finder.', 'error'); });
  }

  // ---- relative timestamps ----
  function relTime(iso) {
    var secs = (Date.now() - new Date(iso)) / 1000;
    if (isNaN(secs)) return null;
    if (secs < 60) return 'Just now';
    if (secs < 3600) return Math.floor(secs / 60) + ' min ago';
    if (secs < 86400) return Math.floor(secs / 3600) + ' h ago';
    if (secs < 172800) return 'Yesterday';
    if (secs < 604800) return Math.floor(secs / 86400) + ' days ago';
    return new Date(iso).toLocaleDateString(undefined, {month: 'short', day: 'numeric', year: 'numeric'});
  }
  document.querySelectorAll('time[datetime]').forEach(function(t) {
    var r = relTime(t.getAttribute('datetime'));
    if (r) t.textContent = r;
  });

  // ---- live timers (sidebar + live panel) ----
  function fmtElapsed(ms) {
    var s = Math.max(0, Math.floor(ms / 1000)), h = Math.floor(s / 3600), m = Math.floor(s / 60) % 60, sec = s % 60;
    var pad = function(n) { return (n < 10 ? '0' : '') + n; };
    return (h ? h + ':' + pad(m) : pad(m)) + ':' + pad(sec);
  }
  var timers = document.querySelectorAll('[data-timer]');
  function tick() {
    timers.forEach(function(el) { el.textContent = fmtElapsed(Date.now() - new Date(el.dataset.timer)); });
  }
  if (timers.length) { tick(); setInterval(tick, 1000); }

  // ---- live panel polling: step count + latest screenshot ----
  var live = document.querySelector('[data-live]');
  if (live) {
    var liveName = live.dataset.name, lastSteps = +live.dataset.steps;
    setInterval(function() {
      apiJSON('GET', '/api/sessions').then(function(list) {
        var s = list.find(function(x) { return x.name === liveName; });
        if (!s || !s.recording) { window.location.reload(); return; }
        live.querySelector('[data-live-steps]').textContent = s.steps;
        live.querySelector('[data-live-shots]').textContent = s.screenshots;
        if (s.steps !== lastSteps && s.has_thumb) {
          lastSteps = s.steps;
          var fig = live.querySelector('.live-preview');
          var img = fig.querySelector('img');
          if (!img) {
            img = document.createElement('img');
            img.alt = 'Latest screenshot';
            var wait = fig.querySelector('.live-wait');
            if (wait) wait.replaceWith(img);
          }
          img.src = '/thumb/' + encodeURIComponent(liveName) + '?pos=last&v=' + s.steps;
        }
      }).catch(function() {});
    }, 2500);
  }

  // ---- new recording ----
  var newForm = document.getElementById('newForm');
  var newName = document.getElementById('newName');
  if (newForm) {
    if (location.hash === '#new') newName.focus();
    newForm.addEventListener('submit', function(e) {
      e.preventDefault();
      var name = newName.value.trim();
      if (!name) return;
      var btn = newForm.querySelector('button');
      btn.disabled = true;
      apiJSON('POST', '/api/sessions', {name: name}).then(function(res) {
        if (!res.watching) {
          Loomless.toast('Recording created, but automatic capture is off: ' + (res.warning || 'unavailable') + '.', 'error');
        }
        setTimeout(function() { window.location.href = '/'; }, res.watching ? 150 : 1800);
      }).catch(function(e) {
        btn.disabled = false;
        Loomless.toast(e.message || 'Could not start recording.', 'error');
      });
    });
  }
  document.querySelectorAll('[data-new]').forEach(function(a) {
    a.addEventListener('click', function(e) {
      if (newName) { e.preventDefault(); newName.focus(); newName.scrollIntoView({block: 'center', behavior: 'smooth'}); }
    });
  });

  // ---- keyboard shortcuts: N = new recording, / = search ----
  var search = document.querySelector('[data-search]');
  document.addEventListener('keydown', function(e) {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    var t = e.target;
    if (t.closest && t.closest('input, textarea, select, [contenteditable], .dialog')) return;
    if (e.key === 'n' || e.key === 'N') {
      e.preventDefault();
      if (newName) newName.focus(); else window.location.href = '/#new';
    } else if (e.key === '/' && search) {
      e.preventDefault();
      search.focus();
    }
  });

  // ---- library: search + sort ----
  var grid = document.querySelector('[data-grid]');
  function applyFilter() {
    if (!grid) return;
    var q = search ? search.value.trim().toLowerCase() : '', shown = 0;
    grid.querySelectorAll('.tile').forEach(function(tile) {
      var match = !q || tile.dataset.search.indexOf(q) !== -1;
      tile.hidden = !match;
      if (match) shown++;
    });
    var none = document.querySelector('.no-match');
    if (none) none.hidden = shown !== 0;
  }
  if (search) search.addEventListener('input', applyFilter);
  var sort = document.querySelector('[data-sort]');
  if (sort && grid) {
    sort.addEventListener('change', function() {
      var tiles = Array.prototype.slice.call(grid.querySelectorAll('.tile'));
      var by = {
        new: function(a, b) { return b.dataset.started.localeCompare(a.dataset.started); },
        old: function(a, b) { return a.dataset.started.localeCompare(b.dataset.started); },
        name: function(a, b) { return a.dataset.name.localeCompare(b.dataset.name); },
        steps: function(a, b) { return b.dataset.steps - a.dataset.steps; },
      }[sort.value];
      tiles.sort(by).forEach(function(t) { grid.appendChild(t); });
    });
  }

  // Keep the sidebar's Library count and the Library header in step
  // with tiles removed in place (delete), without a reload.
  function adjustCount(delta) {
    var side = document.querySelector('.side-count');
    var n = (side ? +side.textContent : 0) + delta;
    if (side) side.textContent = n;
    var label = document.querySelector('[data-count-label]');
    if (label) label.textContent = n + ' recording' + (n === 1 ? '' : 's');
  }

  // ---- actions ----
  document.addEventListener('click', function(e) {
    var el = e.target.closest('[data-action]');
    if (!el) return;
    var name = el.dataset.name, action = el.dataset.action, format = el.dataset.format;
    var tile = el.closest('.tile');

    if (action === 'open') {
      apiJSON('POST', sessionPath(name, 'edit')).then(function(res) {
        var w = window.open(res.url, '_blank');
        if (!w) window.location.href = res.url;
      }).catch(function(e) { Loomless.toast(e.message || 'Could not open the editor.', 'error'); });

    } else if (action === 'export') {
      Loomless.toast('Exporting ' + FORMAT_LABEL[format] + '…');
      apiJSON('POST', sessionPath(name, 'export'), {format: format}).then(function() {
        Loomless.toast(FORMAT_LABEL[format] + ' ready.', 'success', {
          label: 'Show in Finder',
          onClick: function() { reveal(name, format); },
        });
        var pills = tile && tile.querySelector('.pills');
        if (pills && !pills.querySelector('[data-format="' + format + '"]')) {
          var p = document.createElement('button');
          p.className = 'pill';
          p.dataset.action = 'reveal';
          p.dataset.format = format;
          p.dataset.name = name;
          p.title = 'Show ' + FORMAT_PILL[format] + ' in Finder';
          p.textContent = FORMAT_PILL[format];
          pills.appendChild(p);
        }
      }).catch(function(e) { Loomless.toast(e.message || 'Export failed.', 'error'); });

    } else if (action === 'reveal') {
      reveal(name, format);

    } else if (action === 'stop') {
      Loomless.confirm({title: 'Stop recording?', body: '"' + name + '" will stop capturing clicks. You can edit and export it right after.', confirmLabel: 'Stop recording', danger: true}).then(function(ok) {
        if (!ok) return;
        api('POST', sessionPath(name, 'stop')).then(function() { window.location.reload(); })
          .catch(function(e) { Loomless.toast(e.message || 'Could not stop recording.', 'error'); });
      });

    } else if (action === 'rename') {
      Loomless.prompt({title: 'Rename recording', value: name, confirmLabel: 'Rename'}).then(function(next) {
        if (!next || next === name) return;
        api('POST', sessionPath(name, 'rename'), {name: next}).then(function() { window.location.reload(); })
          .catch(function(e) { Loomless.toast(e.message || 'Could not rename.', 'error'); });
      });

    } else if (action === 'delete') {
      Loomless.confirm({title: 'Delete "' + name + '"?', body: 'It moves to the trash — you can undo right after.', confirmLabel: 'Delete', danger: true}).then(function(ok) {
        if (!ok) return;
        apiJSON('POST', sessionPath(name, 'delete')).then(function(res) {
          if (tile) {
            tile.classList.add('removing');
            setTimeout(function() { tile.remove(); applyFilter(); }, 220);
          }
          adjustCount(-1);
          Loomless.toast('Deleted "' + name + '".', null, {
            label: 'Undo',
            onClick: function() {
              apiJSON('POST', '/api/trash/' + encodeURIComponent(res.trash_id) + '/restore')
                .then(function() { window.location.reload(); })
                .catch(function(e) { Loomless.toast(e.message || 'Could not restore.', 'error'); });
            },
          });
        }).catch(function(e) { Loomless.toast(e.message || 'Could not delete.', 'error'); });
      });
    }
  });
`
