package dashboard

import (
	"fmt"
	"html"
	"strings"

	"github.com/noorshikalgar/loomless/internal/ui"
)

// Inline icons (24×24, stroke = currentColor) — small enough to inline,
// and inlining means no extra requests and they recolor with the theme.
const (
	iconPlus    = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>`
	iconStop    = `<svg viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>`
	iconOpen    = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 013 3L7 19l-4 1 1-4z"/></svg>`
	iconExport  = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12"/><path d="M7 10l5 5 5-5"/><path d="M5 21h14"/></svg>`
	iconMore    = `<svg viewBox="0 0 24 24" fill="currentColor"><circle cx="5" cy="12" r="1.7"/><circle cx="12" cy="12" r="1.7"/><circle cx="19" cy="12" r="1.7"/></svg>`
	iconFolder  = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"><path d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z"/></svg>`
	iconRename  = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M4 7V5h16v2"/><path d="M12 5v14"/><path d="M9 19h6"/></svg>`
	iconFile    = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"><path d="M14 3H6a2 2 0 00-2 2v14a2 2 0 002 2h12a2 2 0 002-2V9z"/><path d="M14 3v6h6"/></svg>`
	iconSlides  = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"><rect x="3" y="4" width="18" height="13" rx="2"/><path d="M12 17v4M8 21h8"/></svg>`
	iconSearch  = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5"/></svg>`
	iconCapture = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="3.5" fill="currentColor"/></svg>`
)

// pageShell wraps body content in the shared design-system chrome —
// one top bar, one <head>, one theme — on every dashboard page.
func pageShell(title, current, body string) string {
	nav := func(href, label string) string {
		cur := ""
		if href == current {
			cur = ` aria-current="page"`
		}
		return `<a href="` + href + `"` + cur + `>` + label + `</a>`
	}
	return `<!doctype html>
<html lang="en"` + ui.HTMLAttrs() + `><head><meta charset="utf-8">
<title>` + html.EscapeString(title) + ` · Loomless</title>
` + ui.Head() + `
<style>` + dashboardCSS + `</style>
</head><body>
<div class="container">
<header class="topbar">
  <a class="brand" href="/"><img src="/ui/icon.svg" alt="">Loomless</a>
  <nav class="nav">` + nav("/", "Home") + nav("/sessions", "All sessions") + `</nav>
  <span class="spacer"></span>
  ` + ui.ThemePicker() + `
</header>
` + body + `
</div>
<script>` + dashboardJS + `</script>
</body></html>`
}

func homeHTML(recent []summary, total int, recording []summary) string {
	var b strings.Builder
	for _, s := range recording {
		name := html.EscapeString(s.Name)
		fmt.Fprintf(&b, `<section class="card recording-banner" role="status">
  <span class="badge badge-recording">Recording</span>
  <div class="recording-text"><strong>%s</strong><span class="muted">Loomless is capturing every click — work through the steps, then stop.</span></div>
  <button class="btn btn-danger" data-action="stop" data-name="%s">%s Stop recording</button>
</section>`, name, name, iconStop)
	}

	b.WriteString(`<section class="hero">
  <h1>Capture what you did.</h1>
  <p class="muted">Start a recording, click through the bug or feature, and Loomless turns it into a step-by-step story with screenshots — ready to edit and share.</p>
  <form id="newForm" class="new-form" autocomplete="off">
    <input type="text" id="newName" class="input input-lg" placeholder="Name it, e.g. checkout-bug" aria-label="Session name" required>
    <button type="submit" class="btn btn-primary btn-lg">` + iconCapture + ` Start recording</button>
  </form>
</section>`)

	b.WriteString(`<section><div class="section-head"><h2>Recent</h2>`)
	if total > len(recent) {
		fmt.Fprintf(&b, `<a href="/sessions">View all %d &rarr;</a>`, total)
	}
	b.WriteString(`</div>`)
	if len(recent) == 0 {
		b.WriteString(`<div class="empty-state"><h3>No recordings yet</h3><p>Give your first one a name above and hit Start recording.</p></div>`)
	} else {
		b.WriteString(`<div class="session-grid">`)
		for _, s := range recent {
			b.WriteString(sessionCard(s))
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</section>`)
	return pageShell("Home", "/", b.String())
}

func sessionsHTML(sums []summary) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<section>
<div class="section-head">
  <h2>All sessions <span class="count">%d</span></h2>
  <label class="search">%s<input type="search" class="input" data-search placeholder="Search sessions" aria-label="Search sessions"></label>
</div>`, len(sums), iconSearch)
	if len(sums) == 0 {
		b.WriteString(`<div class="empty-state"><h3>No sessions yet</h3><p><a href="/">Start your first recording</a>.</p></div>`)
	} else {
		b.WriteString(`<div class="session-grid">`)
		for _, s := range sums {
			b.WriteString(sessionCard(s))
		}
		b.WriteString(`</div><div class="empty-state no-match" hidden><h3>No matches</h3><p>No session name contains that text.</p></div>`)
	}
	b.WriteString(`</section>`)
	return pageShell("All sessions", "/sessions", b.String())
}

// sessionCard renders one session — shared by Home and All sessions so
// both always show the same facts the same way. Export chips double as
// shortcuts: a chip for a format that's already been exported reveals
// that file in Finder.
func sessionCard(s summary) string {
	name := html.EscapeString(s.Name)
	badge := `<span class="badge">Stopped</span>`
	stop := ""
	if s.Recording {
		badge = `<span class="badge badge-recording">Recording</span>`
		stop = `<button class="btn btn-danger btn-sm" data-action="stop" data-name="` + name + `">` + iconStop + ` Stop</button>`
	}
	chip := func(format, label string, exported bool) string {
		if exported {
			return `<button class="chip chip-done" data-chip="` + format + `" data-action="reveal" data-format="` + format + `" data-name="` + name + `" title="Show ` + label + ` in Finder">&check; ` + label + `</button>`
		}
		return `<span class="chip" data-chip="` + format + `" title="Not exported yet">` + label + `</span>`
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<article class="card card-hover session-card" data-search="%s">
  <div class="session-head">
    <button class="session-title" data-action="open" data-name="%s" title="Open in editor">%s</button>
    %s
  </div>
  <div class="session-meta muted">%d step%s · %d screenshot%s · <time datetime="%s" title="%s">%s</time></div>
  <div class="session-chips">%s%s%s</div>
  <div class="session-actions">
    <button class="btn btn-primary btn-sm" data-action="open" data-name="%s">%s Open</button>
    %s
    <span class="spacer"></span>
    <details class="menu">
      <summary class="btn btn-sm" aria-label="Export">%s Export</summary>
      <div class="menu-list">
        <button class="menu-item" data-action="export" data-format="md" data-name="%s">%s Markdown</button>
        <button class="menu-item" data-action="export" data-format="pdf" data-name="%s">%s PDF</button>
        <button class="menu-item" data-action="export" data-format="slides" data-name="%s">%s Slide deck</button>
      </div>
    </details>
    <details class="menu">
      <summary class="btn btn-sm btn-icon btn-ghost" aria-label="More actions" title="More actions">%s</summary>
      <div class="menu-list">
        <button class="menu-item" data-action="rename" data-name="%s">%s Rename</button>
        <button class="menu-item" data-action="reveal" data-name="%s">%s Show in Finder</button>
      </div>
    </details>
  </div>
</article>`,
		html.EscapeString(strings.ToLower(s.Name)),
		name, name, badge,
		s.Steps, plural(s.Steps), s.Screenshots, plural(s.Screenshots), s.StartedISO, html.EscapeString(s.Started), html.EscapeString(s.Started),
		chip("md", "Markdown", s.ExportedMD), chip("pdf", "PDF", s.ExportedPDF), chip("slides", "Slides", s.ExportedSl),
		name, iconOpen,
		stop,
		iconExport,
		name, iconFile, name, iconFile, name, iconSlides,
		iconMore,
		name, iconRename, name, iconFolder)
	return b.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

const dashboardCSS = `
  .hero { margin-bottom: var(--space-10); }
  .hero h1 { margin: 0 0 var(--space-2); font-size: 2rem; }
  .hero p { margin: 0 0 var(--space-5); max-width: 60ch; }
  .new-form { display: flex; gap: var(--space-2); flex-wrap: wrap; max-width: 640px; }
  .new-form .input { flex: 1; min-width: 220px; }

  .recording-banner {
    display: flex; align-items: center; gap: var(--space-4); flex-wrap: wrap;
    margin-bottom: var(--space-6); border-color: var(--recording-bg);
  }
  .recording-text { display: flex; flex-direction: column; flex: 1; min-width: 200px; }
  .recording-text .muted { font-size: var(--text-sm); }

  .section-head { display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); margin-bottom: var(--space-4); flex-wrap: wrap; }
  .section-head h2 { margin: 0; display: flex; align-items: center; gap: var(--space-2); }
  .section-head a { font-size: var(--text-sm); }
  .count { font-size: var(--text-xs); font-weight: var(--weight-medium); color: var(--text-muted); background: var(--surface-2); border-radius: var(--radius-full); padding: 1px var(--space-2); }
  .search { position: relative; display: flex; align-items: center; }
  .search svg { position: absolute; left: var(--space-3); width: 15px; height: 15px; color: var(--text-muted); pointer-events: none; }
  .search .input { padding-left: 2.1rem; width: 260px; }

  .session-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: var(--space-4); }
  .session-card { display: flex; flex-direction: column; gap: var(--space-2); }
  .session-head { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-2); }
  .session-title {
    font: inherit; font-weight: var(--weight-semibold); font-size: var(--text-base); color: var(--text);
    background: none; border: 0; padding: 0; text-align: left; cursor: pointer; word-break: break-word;
  }
  .session-title:hover { color: var(--accent); }
  .session-meta { font-size: var(--text-sm); }
  .session-chips { display: flex; gap: var(--space-1); flex-wrap: wrap; }
  .chip {
    font: inherit; font-size: var(--text-xs); padding: 1px var(--space-2);
    border-radius: var(--radius-full); border: 1px dashed var(--border); color: var(--text-muted); background: none;
  }
  .chip-done { border-style: solid; border-color: var(--success-border); color: var(--success); cursor: pointer; }
  .chip-done:hover { background: var(--surface-2); }
  .session-actions { display: flex; align-items: center; gap: var(--space-2); margin-top: var(--space-2); }

  @media (max-width: 640px) {
    .hero h1 { font-size: var(--text-2xl); }
    .new-form { flex-direction: column; }
    .new-form .btn { width: 100%; }
    .search, .search .input { width: 100%; }
    .session-grid { grid-template-columns: 1fr; }
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

  function reveal(name, format) {
    return api('POST', sessionPath(name, 'reveal'), format ? {format: format} : {})
      .catch(function(e) { Loomless.toast(e.message || 'Could not open Finder.', 'error'); });
  }

  // Relative timestamps ("5 min ago"); the exact time stays in title.
  function relTime(iso) {
    var then = new Date(iso), secs = (Date.now() - then) / 1000;
    if (isNaN(secs)) return null;
    if (secs < 60) return 'just now';
    if (secs < 3600) return Math.floor(secs / 60) + ' min ago';
    if (secs < 86400) return Math.floor(secs / 3600) + ' h ago';
    if (secs < 172800) return 'yesterday';
    if (secs < 604800) return Math.floor(secs / 86400) + ' days ago';
    return null;
  }
  document.querySelectorAll('time[datetime]').forEach(function(t) {
    var r = relTime(t.getAttribute('datetime'));
    if (r) t.textContent = r;
  });

  var newForm = document.getElementById('newForm');
  if (newForm) {
    newForm.addEventListener('submit', function(e) {
      e.preventDefault();
      var input = document.getElementById('newName');
      var name = input.value.trim();
      if (!name) return;
      var btn = newForm.querySelector('button');
      btn.disabled = true;
      apiJSON('POST', '/api/sessions', {name: name}).then(function(res) {
        if (res.watching) {
          Loomless.toast('Recording "' + name + '" — go click through the steps.', 'success');
        } else {
          Loomless.toast('Created "' + name + '", but automatic capture is off: ' + (res.warning || 'unavailable') + '.', 'error');
        }
        setTimeout(function() { window.location.reload(); }, 900);
      }).catch(function(e) {
        btn.disabled = false;
        Loomless.toast(e.message || 'Could not start recording.', 'error');
      });
    });
  }

  var search = document.querySelector('[data-search]');
  if (search) {
    search.addEventListener('input', function() {
      var q = search.value.trim().toLowerCase(), shown = 0;
      document.querySelectorAll('.session-card').forEach(function(card) {
        var match = !q || card.dataset.search.indexOf(q) !== -1;
        card.hidden = !match;
        if (match) shown++;
      });
      var none = document.querySelector('.no-match');
      if (none) none.hidden = shown !== 0;
    });
  }

  document.addEventListener('click', function(e) {
    var el = e.target.closest('[data-action]');
    if (!el) return;
    var name = el.dataset.name, action = el.dataset.action, format = el.dataset.format;

    if (action === 'open') {
      apiJSON('POST', sessionPath(name, 'edit')).then(function(res) {
        var w = window.open(res.url, '_blank');
        if (!w) window.location.href = res.url;
      }).catch(function(e) { Loomless.toast(e.message || 'Could not open the editor.', 'error'); });
    } else if (action === 'export') {
      Loomless.toast('Exporting ' + FORMAT_LABEL[format] + '…');
      apiJSON('POST', sessionPath(name, 'export'), {format: format}).then(function() {
        Loomless.toast(FORMAT_LABEL[format] + ' exported.', 'success', {
          label: 'Show in Finder',
          onClick: function() { reveal(name, format); },
        });
        var card = el.closest('.session-card');
        var chip = card && card.querySelector('span.chip[data-chip="' + format + '"]');
        if (chip) {
          var done = document.createElement('button');
          done.className = 'chip chip-done';
          done.dataset.chip = format;
          done.dataset.action = 'reveal';
          done.dataset.format = format;
          done.dataset.name = name;
          done.title = 'Show in Finder';
          done.textContent = '\u2713 ' + chip.textContent;
          chip.replaceWith(done);
        }
      }).catch(function(e) { Loomless.toast(e.message || 'Export failed.', 'error'); });
    } else if (action === 'reveal') {
      reveal(name, format);
    } else if (action === 'stop') {
      Loomless.confirm({title: 'Stop recording?', body: '"' + name + '" will stop capturing clicks. You can edit and export it afterwards.', confirmLabel: 'Stop recording', danger: true}).then(function(ok) {
        if (!ok) return;
        api('POST', sessionPath(name, 'stop')).then(function() { window.location.reload(); })
          .catch(function(e) { Loomless.toast(e.message || 'Could not stop recording.', 'error'); });
      });
    } else if (action === 'rename') {
      Loomless.prompt({title: 'Rename session', value: name, confirmLabel: 'Rename'}).then(function(next) {
        if (!next || next === name) return;
        api('POST', sessionPath(name, 'rename'), {name: next}).then(function() { window.location.reload(); })
          .catch(function(e) { Loomless.toast(e.message || 'Could not rename.', 'error'); });
      });
    }
  });
`
