package dashboard

import (
	"fmt"
	"html"
	"strings"
)

// pageShell wraps body content in the same dark, icon-branded chrome on
// every dashboard page — one place to change if the look needs to
// change, rather than three near-identical <head> blocks.
func pageShell(title, body string) string {
	return `<!doctype html>
<html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + html.EscapeString(title) + ` — Loomless</title>
<style>` + dashboardCSS + `</style>
</head><body>
<div class="wrap">
<header class="topbar">
  <a class="brand" href="/">Loomless</a>
  <nav><a href="/">Home</a><a href="/sessions">All sessions</a></nav>
</header>
` + body + `
</div>
<script>` + dashboardJS + `</script>
</body></html>`
}

func homeHTML(recent []summary, total int) string {
	var b strings.Builder
	b.WriteString(`<section class="hero">
  <h1>Loomless</h1>
  <p class="hint">Record a session, then edit and export it — no command line needed.</p>
  <form id="newForm" class="new-form">
    <input type="text" id="newName" placeholder="Session name, e.g. login-bug" required>
    <button type="submit" class="btn-primary">+ New Recording</button>
  </form>
  <p class="form-hint" id="newHint"></p>
</section>`)

	b.WriteString(`<section><div class="section-head"><h2>Recent</h2>`)
	if total > len(recent) {
		b.WriteString(`<a href="/sessions">View all ` + fmt.Sprint(total) + ` &rarr;</a>`)
	}
	b.WriteString(`</div>`)
	if len(recent) == 0 {
		b.WriteString(`<p class="empty">No sessions yet — start one above.</p>`)
	} else {
		b.WriteString(`<div class="card-grid">`)
		for _, s := range recent {
			b.WriteString(sessionCard(s))
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</section>`)
	return pageShell("Home", b.String())
}

func sessionsHTML(sums []summary) string {
	var b strings.Builder
	b.WriteString(`<section><div class="section-head"><h2>All sessions</h2></div>`)
	if len(sums) == 0 {
		b.WriteString(`<p class="empty">No sessions yet. <a href="/">Start one</a>.</p>`)
	} else {
		b.WriteString(`<div class="card-grid">`)
		for _, s := range sums {
			b.WriteString(sessionCard(s))
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</section>`)
	return pageShell("All sessions", b.String())
}

// sessionCard renders one session's metadata plus its actions — the
// same facts and the same real-file-on-disk export check
// Handler.summaries computes, just as a card instead of a table row, so
// home and /sessions can share this exact markup.
func sessionCard(s summary) string {
	name := html.EscapeString(s.Name)
	status := `<span class="badge stopped">stopped</span>`
	if s.Recording {
		status = `<span class="badge recording">&#9679; recording</span>`
	}
	stopBtn := ""
	if s.Recording {
		stopBtn = `<button class="act" data-action="stop" data-name="` + name + `">Stop</button>`
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="card" data-name="%s">
  <div class="card-head">
    <span class="card-title" data-rename-label>%s</span>
    %s
  </div>
  <p class="meta">%d step(s), %d screenshot(s) &middot; started %s</p>
  <p class="meta exports">%s &middot; %s &middot; %s</p>
  <div class="card-actions">
    <button class="act primary" data-action="edit" data-name="%s">Edit</button>
    <button class="act" data-action="export" data-format="md" data-name="%s">Export MD</button>
    <button class="act" data-action="export" data-format="pdf" data-name="%s">Export PDF</button>
    <button class="act" data-action="export" data-format="slides" data-name="%s">Export Slides</button>
    <button class="act" data-action="rename" data-name="%s">Rename</button>
    %s
  </div>
</div>`,
		name, name, status,
		s.Steps, s.Screenshots, s.Started,
		exportMark("MD", s.ExportedMD), exportMark("PDF", s.ExportedPDF), exportMark("Slides", s.ExportedSl),
		name, name, name, name, name, stopBtn)
	return b.String()
}

func exportMark(label string, exported bool) string {
	if exported {
		return `<span class="exp-yes">&check; ` + label + `</span>`
	}
	return `<span class="exp-no">` + label + `</span>`
}

const dashboardCSS = `
  :root { color-scheme: dark; }
  * { box-sizing: border-box; }
  body { margin: 0; padding: 0 1.5rem 3rem; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #1a1a1e; color: #f0f0f0; }
  .wrap { max-width: 1000px; margin: 0 auto; }
  a { color: #8ab4ff; text-decoration: none; }
  a:hover { text-decoration: underline; }
  .topbar { display: flex; align-items: center; justify-content: space-between; padding: 1.2rem 0; border-bottom: 1px solid #333; margin-bottom: 2rem; }
  .brand { font-weight: 700; font-size: 1.1rem; color: #f0f0f0; }
  .topbar nav { display: flex; gap: 1.2rem; }
  .hero { margin-bottom: 2.5rem; }
  .hero h1 { margin: 0 0 0.3rem; }
  .hint { color: #999; margin: 0 0 1.2rem; }
  .new-form { display: flex; gap: 0.6rem; flex-wrap: wrap; }
  .new-form input { flex: 1; min-width: 220px; background: #17171a; color: #f0f0f0; border: 1px solid #444; border-radius: 6px; padding: 0.6rem 0.8rem; font-size: 0.95rem; }
  .btn-primary { background: #2a5a3a; color: #f0f0f0; border: 1px solid #3a7a4a; border-radius: 6px; padding: 0.6rem 1.2rem; cursor: pointer; font-weight: 600; }
  .btn-primary:hover { background: #366e46; }
  .form-hint { min-height: 1.2rem; font-size: 0.85rem; color: #7ee787; margin: 0.6rem 0 0; }
  .form-hint.error { color: #ff6b6b; }
  .section-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1rem; }
  .section-head h2 { margin: 0; font-size: 1.15rem; }
  .empty { color: #999; }
  .card-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1rem; }
  .card { background: #232328; border: 1px solid #333; border-radius: 8px; padding: 1rem 1.15rem; }
  .card-head { display: flex; align-items: center; justify-content: space-between; gap: 0.5rem; margin-bottom: 0.4rem; }
  .card-title { font-weight: 600; word-break: break-word; }
  .badge { font-size: 0.7rem; padding: 0.15rem 0.5rem; border-radius: 999px; white-space: nowrap; }
  .badge.stopped { background: #333338; color: #999; }
  .badge.recording { background: #5a2a2a; color: #ff8a8a; }
  .meta { margin: 0.15rem 0; font-size: 0.82rem; color: #999; }
  .exports { display: flex; gap: 0.6rem; }
  .exp-yes { color: #7ee787; }
  .exp-no { color: #666; }
  .card-actions { display: flex; flex-wrap: wrap; gap: 0.4rem; margin-top: 0.8rem; }
  .act { background: #2a2a30; color: #f0f0f0; border: 1px solid #444; border-radius: 4px; padding: 0.35rem 0.65rem; cursor: pointer; font-size: 0.8rem; }
  .act:hover { background: #3a3a42; }
  .act.primary { background: #2a4a6a; border-color: #3c6ca0; }
  .act.primary:hover { background: #34578a; }
  @media (max-width: 480px) {
    body { padding: 0 1rem 2rem; }
    .new-form { flex-direction: column; }
    .new-form input { width: 100%; }
  }
`

const dashboardJS = `
  function api(method, path, body) {
    return fetch(path, {
      method: method,
      headers: body ? {'Content-Type': 'application/json'} : undefined,
      body: body ? JSON.stringify(body) : undefined,
    }).then(function(r) {
      if (!r.ok) return r.text().then(function(t) { throw new Error(t || r.statusText); });
      return r;
    });
  }
  function apiJSON(method, path, body) {
    return api(method, path, body).then(function(r) { return r.json(); });
  }

  var newForm = document.getElementById('newForm');
  if (newForm) {
    newForm.addEventListener('submit', function(e) {
      e.preventDefault();
      var input = document.getElementById('newName');
      var hint = document.getElementById('newHint');
      var name = input.value.trim();
      if (!name) return;
      apiJSON('POST', '/api/sessions', {name: name}).then(function(res) {
        hint.classList.remove('error');
        hint.textContent = res.watching ? 'Recording started — click around, then come back here.' : ('Created (manual capture only: ' + (res.warning || 'auto-capture unavailable') + ')');
        setTimeout(function() { window.location.reload(); }, 900);
      }).catch(function(e) {
        hint.classList.add('error');
        hint.textContent = e.message || 'Could not start recording.';
      });
    });
  }

  document.addEventListener('click', function(e) {
    var btn = e.target.closest('.act');
    if (!btn) return;
    var name = btn.dataset.name;
    var action = btn.dataset.action;

    if (action === 'edit') {
      apiJSON('POST', '/api/sessions/' + encodeURIComponent(name) + '/edit').then(function(res) {
        window.open(res.url, '_blank');
      }).catch(function(e) { alert(e.message || 'Could not open editor.'); });
      return;
    }
    if (action === 'export') {
      btn.disabled = true;
      var original = btn.textContent;
      apiJSON('POST', '/api/sessions/' + encodeURIComponent(name) + '/export', {format: btn.dataset.format}).then(function(res) {
        btn.textContent = 'Exported!';
        setTimeout(function() { btn.textContent = original; btn.disabled = false; }, 1500);
      }).catch(function(e) {
        alert(e.message || 'Export failed.');
        btn.disabled = false;
      });
      return;
    }
    if (action === 'stop') {
      if (!confirm('Stop recording "' + name + '"?')) return;
      api('POST', '/api/sessions/' + encodeURIComponent(name) + '/stop').then(function() {
        window.location.reload();
      }).catch(function(e) { alert(e.message || 'Could not stop recording.'); });
      return;
    }
    if (action === 'rename') {
      var next = prompt('Rename "' + name + '" to:', name);
      if (!next || next === name) return;
      api('POST', '/api/sessions/' + encodeURIComponent(name) + '/rename', {name: next}).then(function() {
        window.location.reload();
      }).catch(function(e) { alert(e.message || 'Could not rename.'); });
      return;
    }
  });
`
