package editor

import "html"

func indexHTML(name string) string {
	return `<!doctype html>
<html><head><meta charset="utf-8">
<title>Edit — ` + html.EscapeString(name) + `</title>
<style>` + editorCSS + `</style>
</head><body>
<h1>` + html.EscapeString(name) + `</h1>
<p class="hint">Reorder, hide, or edit captions, then export from the CLI/MCP as usual — changes are saved immediately.</p>
<div id="steps"></div>
<script>` + editorJS + `</script>
</body></html>
`
}

const editorCSS = `
  :root { color-scheme: dark; }
  * { box-sizing: border-box; }
  body { margin: 0; padding: 2rem; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #1a1a1e; color: #f0f0f0; max-width: 900px; margin-inline: auto; }
  h1 { margin-bottom: 0.25rem; }
  .hint { color: #999; margin-top: 0; margin-bottom: 2rem; }
  .step { background: #232328; border: 1px solid #333; border-radius: 8px; padding: 1rem 1.25rem; margin-bottom: 1rem; }
  .step.hidden-step { opacity: 0.45; }
  .step-header { display: flex; align-items: center; gap: 0.75rem; margin-bottom: 0.75rem; }
  .step-header h2 { font-size: 1.1rem; margin: 0; color: #8ab4ff; flex: 1; }
  .step-header button { background: #2a2a30; color: #f0f0f0; border: 1px solid #444; border-radius: 4px; padding: 0.3rem 0.6rem; cursor: pointer; font-size: 0.85rem; }
  .step-header button:hover { background: #3a3a42; }
  .step-header button:disabled { opacity: 0.3; cursor: default; }
  .window { font-weight: 600; color: #8ab4ff; font-size: 0.9rem; margin-bottom: 0.5rem; }
  textarea { width: 100%; min-height: 3rem; background: #17171a; color: #f0f0f0; border: 1px solid #444; border-radius: 4px; padding: 0.5rem; font-family: inherit; font-size: 0.95rem; resize: vertical; }
  .save-row { display: flex; justify-content: flex-end; margin-top: 0.4rem; gap: 0.5rem; align-items: center; }
  .save-row button { background: #2a5a3a; color: #f0f0f0; border: 1px solid #3a7a4a; border-radius: 4px; padding: 0.4rem 0.9rem; cursor: pointer; }
  .save-row button:hover { background: #366e46; }
  .saved-hint { color: #7ee787; font-size: 0.85rem; opacity: 0; transition: opacity 0.3s; }
  .saved-hint.show { opacity: 1; }
  img { max-width: 100%; border-radius: 4px; display: block; }
  pre { background: #17171a; padding: 0.75rem; border-radius: 4px; overflow-x: auto; font-size: 0.85rem; margin: 0.4rem 0; }
  pre.cmd { color: #7ee787; }
  .img-wrap { position: relative; margin-top: 0.5rem; }
  .img-toolbar { display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.4rem; }
  .img-toolbar button { background: #2a2a30; color: #f0f0f0; border: 1px solid #444; border-radius: 4px; padding: 0.3rem 0.7rem; cursor: pointer; font-size: 0.8rem; }
  .img-toolbar button.armed.redact-btn { background: #7a2a2a; border-color: #a03c3c; }
  .img-toolbar button.armed.highlight-btn { background: #7a5a1a; border-color: #a07c2c; }
  .mode-hint { color: #999; font-size: 0.8rem; }
  .drag-overlay { position: absolute; display: none; pointer-events: none; border: 2px dashed #fff; box-sizing: border-box; }
  .drag-overlay.redact { border-color: #ff5a5a; background: rgba(255, 0, 0, 0.15); }
  .drag-overlay.highlight { border-color: #ffb020; background: rgba(255, 176, 32, 0.15); }
`

const editorJS = `
  var sessionData = null;

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

  function load() {
    fetch('/api/session').then(function(r) { return r.json(); }).then(function(s) {
      sessionData = s;
      render();
    });
  }

  // fullOrder lists every step (hidden ones included, dimmed via CSS —
  // hiding is meant to be reversible in this UI, so a hidden step stays
  // visible-but-dimmed and reorderable rather than disappearing).
  function fullOrder() {
    if (sessionData.display_order && sessionData.display_order.length) return sessionData.display_order.slice();
    return sessionData.steps.map(function(s) { return s.index; });
  }

  function stepByIndex(idx) {
    return sessionData.steps.find(function(s) { return s.index === idx; });
  }

  function isHidden(idx) {
    return (sessionData.hidden || []).indexOf(idx) !== -1;
  }

  function render() {
    var container = document.getElementById('steps');
    container.innerHTML = '';
    var order = fullOrder();

    order.forEach(function(idx, pos) {
      var step = stepByIndex(idx);
      if (!step) return;

      var el = document.createElement('div');
      el.className = 'step' + (isHidden(idx) ? ' hidden-step' : '');

      var header = document.createElement('div');
      header.className = 'step-header';
      var h2 = document.createElement('h2');
      h2.textContent = 'Step ' + step.index;
      header.appendChild(h2);

      var upBtn = mkBtn('↑', pos === 0, function() { moveStep(pos, -1); });
      var downBtn = mkBtn('↓', pos === order.length - 1, function() { moveStep(pos, 1); });
      var hideBtn = mkBtn(isHidden(idx) ? 'Show' : 'Hide', false, function() { toggleHidden(idx); });
      header.appendChild(upBtn);
      header.appendChild(downBtn);
      header.appendChild(hideBtn);
      el.appendChild(header);

      if (step.window_app) {
        var w = document.createElement('div');
        w.className = 'window';
        w.textContent = step.window_app + (step.window_title ? ' — ' + step.window_title : '');
        el.appendChild(w);
      }

      if (step.command) {
        var cmd = document.createElement('pre');
        cmd.className = 'cmd';
        cmd.textContent = '$ ' + step.command;
        el.appendChild(cmd);
        if (step.output) {
          var out = document.createElement('pre');
          out.textContent = step.output;
          el.appendChild(out);
        }
      } else {
        var ta = document.createElement('textarea');
        ta.value = step.note || '';
        el.appendChild(ta);

        var row = document.createElement('div');
        row.className = 'save-row';
        var savedHint = document.createElement('span');
        savedHint.className = 'saved-hint';
        savedHint.textContent = 'Saved';
        var saveBtn = document.createElement('button');
        saveBtn.textContent = 'Save caption';
        saveBtn.onclick = function() {
          api('POST', '/api/caption', {index: step.index, note: ta.value}).then(function() {
            savedHint.classList.add('show');
            setTimeout(function() { savedHint.classList.remove('show'); }, 1500);
            load();
          }).catch(function(e) { alert('Save failed: ' + e.message); });
        };
        row.appendChild(savedHint);
        row.appendChild(saveBtn);
        el.appendChild(row);

        if (step.image_file) {
          var imgWrap = document.createElement('div');
          imgWrap.className = 'img-wrap';

          var toolbar = document.createElement('div');
          toolbar.className = 'img-toolbar';
          var redactBtn = document.createElement('button');
          redactBtn.className = 'redact-btn';
          redactBtn.textContent = 'Draw Redaction';
          var highlightBtn = document.createElement('button');
          highlightBtn.className = 'highlight-btn';
          highlightBtn.textContent = 'Draw Highlight';
          var modeHint = document.createElement('span');
          modeHint.className = 'mode-hint';
          toolbar.appendChild(redactBtn);
          toolbar.appendChild(highlightBtn);
          toolbar.appendChild(modeHint);
          imgWrap.appendChild(toolbar);

          var img = document.createElement('img');
          img.src = '/images/' + step.image_file.split('/').pop();
          img.alt = 'Step ' + step.index + ' screenshot';
          imgWrap.appendChild(img);

          var overlay = document.createElement('div');
          overlay.className = 'drag-overlay';
          imgWrap.appendChild(overlay);

          el.appendChild(imgWrap);
          setupImageEditing(img, overlay, redactBtn, highlightBtn, modeHint, step.index);
        }
      }

      container.appendChild(el);
    });
  }

  function mkBtn(label, disabled, onclick) {
    var b = document.createElement('button');
    b.textContent = label;
    b.disabled = disabled;
    b.addEventListener('click', onclick);
    return b;
  }

  function moveStep(pos, delta) {
    // fullOrder() already lists every step, hidden ones included (they're
    // dimmed, not excluded — see render()), so a simple adjacent swap is
    // a complete, valid order for the API with nothing left to append.
    var order = fullOrder();
    var newPos = pos + delta;
    if (newPos < 0 || newPos >= order.length) return;
    var tmp = order[pos];
    order[pos] = order[newPos];
    order[newPos] = tmp;
    api('POST', '/api/reorder', {order: order}).then(load);
  }

  function toggleHidden(idx) {
    api('POST', '/api/hide', {index: idx, hidden: !isHidden(idx)}).then(load);
  }

  // setupImageEditing wires one step's screenshot up for click-and-drag
  // redaction/highlighting. "Armed" mode: clicking Redact or Highlight
  // arms that mode (crosshair cursor, button visually marked active);
  // the next drag on the image performs it and disarms automatically —
  // deliberately single-shot per click rather than a persistent toggle,
  // so it's always unambiguous which mode (if any) is currently armed
  // when there are several steps' images on the page at once.
  function setupImageEditing(img, overlay, redactBtn, highlightBtn, modeHint, stepIndex) {
    var armedMode = null;
    var dragStart = null;

    function setArmed(mode) {
      armedMode = mode;
      img.style.cursor = mode ? 'crosshair' : 'default';
      modeHint.textContent = mode ? ('Drag on the image to draw a ' + mode + '.') : '';
      redactBtn.classList.toggle('armed', mode === 'redact');
      highlightBtn.classList.toggle('armed', mode === 'highlight');
    }

    redactBtn.addEventListener('click', function() { setArmed(armedMode === 'redact' ? null : 'redact'); });
    highlightBtn.addEventListener('click', function() { setArmed(armedMode === 'highlight' ? null : 'highlight'); });

    function pos(e) {
      var rect = img.getBoundingClientRect();
      return {x: e.clientX - rect.left, y: e.clientY - rect.top};
    }

    function updateOverlay(a, b) {
      overlay.style.left = Math.min(a.x, b.x) + 'px';
      overlay.style.top = Math.min(a.y, b.y) + 'px';
      overlay.style.width = Math.abs(a.x - b.x) + 'px';
      overlay.style.height = Math.abs(a.y - b.y) + 'px';
    }

    img.addEventListener('mousedown', function(e) {
      if (!armedMode) return;
      e.preventDefault();
      dragStart = pos(e);
      overlay.className = 'drag-overlay ' + armedMode;
      overlay.style.display = 'block';
      updateOverlay(dragStart, dragStart);
    });

    window.addEventListener('mousemove', function(e) {
      if (!dragStart) return;
      updateOverlay(dragStart, pos(e));
    });

    window.addEventListener('mouseup', function(e) {
      if (!dragStart) return;
      var cur = pos(e);
      var x0 = Math.min(dragStart.x, cur.x), y0 = Math.min(dragStart.y, cur.y);
      var w = Math.abs(dragStart.x - cur.x), h = Math.abs(dragStart.y - cur.y);
      var mode = armedMode;
      dragStart = null;
      overlay.style.display = 'none';
      setArmed(null);

      if (w < 4 || h < 4) return; // ignore accidental clicks/tiny drags

      // The <img> is often displayed smaller than its natural size (CSS
      // max-width: 100%); convert the drag's on-screen coordinates to the
      // image's real pixel coordinates, which is what the API/PNG edit
      // operates in.
      var scaleX = img.naturalWidth / img.clientWidth;
      var scaleY = img.naturalHeight / img.clientHeight;
      api('POST', '/api/edit-image', {
        index: stepIndex, action: mode,
        x: x0 * scaleX, y: y0 * scaleY, w: w * scaleX, h: h * scaleY,
      }).then(function() {
        // Same filename, new bytes — force a real re-fetch instead of
        // whatever the browser has cached.
        var base = img.src.split('?')[0];
        img.src = base + '?t=' + Date.now();
      }).catch(function(e) { alert('Edit failed: ' + e.message); });
    });
  }

  load();
`
