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
  textarea { width: 100%; min-height: 3rem; background: #17171a; color: #f0f0f0; border: 1px solid #444; border-radius: 4px; padding: 0.5rem; font-family: inherit; font-size: 0.95rem; resize: vertical; }
  .save-row { display: flex; justify-content: flex-end; margin-top: 0.4rem; gap: 0.5rem; align-items: center; }
  .save-row button { background: #2a5a3a; color: #f0f0f0; border: 1px solid #3a7a4a; border-radius: 4px; padding: 0.4rem 0.9rem; cursor: pointer; }
  .save-row button:hover { background: #366e46; }
  .saved-hint { color: #7ee787; font-size: 0.85rem; opacity: 0; transition: opacity 0.3s; }
  .saved-hint.show { opacity: 1; }
  img { max-width: 100%; border-radius: 4px; display: block; }
  pre { background: #17171a; padding: 0.75rem; border-radius: 4px; overflow-x: auto; font-size: 0.85rem; margin: 0.4rem 0; }
  pre.cmd { color: #7ee787; }
  .img-wrap { margin-top: 0.5rem; }
  .img-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 0.5rem; margin-bottom: 0.4rem; }
  .img-toolbar button { background: #2a2a30; color: #f0f0f0; border: 1px solid #444; border-radius: 4px; padding: 0.3rem 0.7rem; cursor: pointer; font-size: 0.8rem; }
  .img-toolbar button:disabled { opacity: 0.35; cursor: default; }
  .img-toolbar button.armed.redact-btn { background: #7a2a2a; border-color: #a03c3c; }
  .img-toolbar button.armed.highlight-btn { background: #7a5a1a; border-color: #a07c2c; }
  .img-toolbar button.apply-btn { background: #2a5a3a; border-color: #3a7a4a; }
  .img-toolbar button.style-selected { background: #2a4a6a; border-color: #3c6ca0; }
  .img-toolbar .sep { width: 1px; height: 1.2rem; background: #444; margin: 0 0.2rem; }
  .mode-hint { color: #999; font-size: 0.8rem; }
  /* inline-block, not block, so this shrink-wraps to the <img>'s actual
     rendered box instead of stretching to the wrapper's full width —
     annotation positions are percentages of this frame, so the frame's
     box must exactly match the image's box or every percentage would be
     computed against the wrong denominator whenever a screenshot renders
     narrower than the page column. */
  .img-frame { position: relative; display: inline-block; max-width: 100%; }
  .drag-overlay { position: absolute; display: none; pointer-events: auto; border: 2px dashed #fff; box-sizing: border-box; cursor: move; }
  .drag-overlay.redact { border-color: #ff5a5a; background: rgba(255, 0, 0, 0.15); }
  .drag-overlay.highlight { border-color: #ffb020; background: rgba(255, 176, 32, 0.15); }
  .resize-handle { position: absolute; width: 10px; height: 10px; margin: -5px; background: #fff; border: 1px solid #333; border-radius: 2px; z-index: 2; }
  .handle-nw, .handle-se { cursor: nwse-resize; }
  .handle-ne, .handle-sw { cursor: nesw-resize; }
  .annotation { position: absolute; border: 3px solid #ff3b30; box-sizing: border-box; cursor: move; z-index: 3; }
  /* Resize handles on an EXISTING annotation are chrome, not content —
     shown only on hover/pin, same as the toolbar, so the default view of
     an annotated screenshot is just the highlight and its number, not a
     cluttered edit-mode look. */
  .annotation .resize-handle { display: none; }
  .annotation:hover .resize-handle, .annotation.pinned .resize-handle { display: block; }
  .annotation-badge { position: absolute; top: -11px; left: -11px; width: 22px; height: 22px; border-radius: 50%; background: #ff3b30; color: #fff; border: 1px solid #a03c3c; text-align: center; line-height: 20px; font-size: 0.7rem; font-family: inherit; z-index: 4; pointer-events: none; user-select: none; }
  /* The toolbar (renumber / reorder / delete) is chrome that only makes
     sense while actively editing an annotation — like a word processor's
     floating selection toolbar, it stays out of the way until you hover
     or click the annotation, rather than permanently overlapping the
     badge the way a delete "x" and stack buttons used to. */
  .annotation-toolbar {
    position: absolute; left: 50%; top: 0; transform: translate(-50%, calc(-100% - 10px));
    display: none; align-items: center; gap: 3px;
    background: #2a2a30; border: 1px solid #444; border-radius: 6px; padding: 3px;
    box-shadow: 0 2px 10px rgba(0,0,0,0.4); z-index: 5; white-space: nowrap;
  }
  .annotation:hover .annotation-toolbar, .annotation.pinned .annotation-toolbar { display: flex; }
  /* Annotations near the top of the image would have their toolbar
     clipped off the top of the frame — flipped below instead (JS adds
     this class based on the annotation's vertical position). */
  .annotation.toolbar-below .annotation-toolbar { top: 100%; transform: translate(-50%, 10px); }
  .annotation-toolbar input[type=number] { width: 2.8rem; background: #17171a; color: #f0f0f0; border: 1px solid #444; border-radius: 4px; padding: 0.15rem 0.3rem; font-size: 0.75rem; font-family: inherit; }
  .annotation-toolbar input::-webkit-inner-spin-button, .annotation-toolbar input::-webkit-outer-spin-button { -webkit-appearance: none; margin: 0; }
  .annotation-toolbar button { background: #333338; color: #f0f0f0; border: 1px solid #4a4a50; border-radius: 4px; width: 22px; height: 22px; font-size: 0.75rem; line-height: 1; cursor: pointer; padding: 0; }
  .annotation-toolbar button:hover { background: #45454c; }
  .annotation-toolbar button:disabled { opacity: 0.3; cursor: default; }
  .annotation-toolbar button.annotation-delete-btn:hover { background: #7a2a2a; border-color: #a03c3c; }
  .annotation-toolbar .toolbar-sep { width: 1px; height: 16px; background: #444; margin: 0 1px; }
  .step-title { display: flex; align-items: center; gap: 0.4rem; flex: 1; }
  .step-title h2 { cursor: text; }
  .step-title h2:hover { text-decoration: underline dotted; text-underline-offset: 3px; }
  .step-title input { font-size: 1.1rem; font-weight: 600; color: #8ab4ff; background: #17171a; border: 1px solid #444; border-radius: 4px; padding: 0.15rem 0.4rem; font-family: inherit; flex: 1; min-width: 0; }
  .window-line { display: flex; align-items: center; gap: 0.3rem; font-size: 0.9rem; margin-bottom: 0.5rem; }
  .window-line span { cursor: text; color: #8ab4ff; font-weight: 600; }
  .window-line span:hover { text-decoration: underline dotted; text-underline-offset: 2px; }
  .window-line input { font-size: 0.9rem; font-weight: 600; color: #8ab4ff; background: #17171a; border: 1px solid #444; border-radius: 4px; padding: 0.1rem 0.35rem; font-family: inherit; }
`

const editorJS = `
  var sessionData = null;
  // stepEditors maps a step index to live handles into that step's
  // already-rendered image editor DOM, so annotation and redaction edits
  // can update just that step's view directly (mutate local data, patch
  // the DOM) instead of re-fetching the whole session and rebuilding
  // every step from scratch — that full-page-reload-per-edit pattern was
  // also what caused annotations to intermittently render invisible: an
  // edit's success handler would re-render before the browser had a
  // settled layout for the freshly (re)inserted, browser-cached <img>,
  // so annotation placement briefly read a zeroed layout. Annotation
  // placement below no longer depends on layout at all (it positions
  // annotations as percentages of the image's natural size, available as
  // soon as the image decodes, whether or not it is even attached to the
  // document yet) — so that failure mode is gone by construction, not by
  // sequencing hacks — but going live-only for annotation/redaction edits
  // is also just a better user experience: no flash, no lost scroll
  // position, and it directly delivers "live annotation updates."
  var stepEditors = {};

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
    stepEditors = {};
    var order = fullOrder();

    order.forEach(function(idx, pos) {
      var step = stepByIndex(idx);
      if (!step) return;

      var el = document.createElement('div');
      el.className = 'step' + (isHidden(idx) ? ' hidden-step' : '');

      var header = document.createElement('div');
      header.className = 'step-header';
      header.appendChild(buildStepTitle(step));

      var upBtn = mkBtn('↑', pos === 0, function() { moveStep(pos, -1); });
      var downBtn = mkBtn('↓', pos === order.length - 1, function() { moveStep(pos, 1); });
      var hideBtn = mkBtn(isHidden(idx) ? 'Show' : 'Hide', false, function() { toggleHidden(idx); });
      header.appendChild(upBtn);
      header.appendChild(downBtn);
      header.appendChild(hideBtn);
      el.appendChild(header);

      if (!step.command) {
        el.appendChild(buildWindowLine(step));
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
          }).catch(function(e) { alert('Save failed: ' + e.message); });
        };
        row.appendChild(savedHint);
        row.appendChild(saveBtn);
        el.appendChild(row);

        if (step.image_file) {
          el.appendChild(buildImageEditor(step));
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

  // buildStepTitle renders the step heading as click-to-edit text: shows
  // "Step N" (or the stored override) as plain text; clicking swaps in a
  // text input, Enter or blur commits, Escape cancels. Typing the default
  // text back (or clearing it) removes the override rather than storing
  // it literally, so "Step 1" only ever means "no custom title" on disk.
  function buildStepTitle(step) {
    var wrap = document.createElement('div');
    wrap.className = 'step-title';

    function defaultTitle() { return 'Step ' + step.index; }

    function show() {
      wrap.innerHTML = '';
      var h2 = document.createElement('h2');
      h2.textContent = step.title || defaultTitle();
      h2.title = 'Click to rename';
      h2.addEventListener('click', edit);
      wrap.appendChild(h2);
    }

    function commit(value) {
      var trimmed = value.trim();
      var newTitle = (trimmed === '' || trimmed === defaultTitle()) ? '' : trimmed;
      if (newTitle === (step.title || '')) { show(); return; }
      api('POST', '/api/title', {index: step.index, title: newTitle}).then(function() {
        step.title = newTitle;
        show();
      }).catch(function(e) { alert('Rename failed: ' + e.message); show(); });
    }

    function edit() {
      wrap.innerHTML = '';
      var input = document.createElement('input');
      input.type = 'text';
      input.value = step.title || defaultTitle();
      wrap.appendChild(input);
      input.focus();
      input.select();
      input.onblur = function() { commit(input.value); };
      input.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') { input.blur(); }
        else if (e.key === 'Escape') { input.onblur = null; show(); }
      });
    }

    show();
    return wrap;
  }

  // buildWindowLine renders the captured app/window-title line the same
  // click-to-edit way — either field can be corrected or filled in by
  // hand (a generic process name, a stale title, or nothing captured at
  // all), independent of the automatic capture that originally populated
  // them.
  function buildWindowLine(step) {
    var wrap = document.createElement('div');
    wrap.className = 'window-line';

    function show() {
      wrap.innerHTML = '';
      var appSpan = document.createElement('span');
      appSpan.textContent = step.window_app || '+ add app';
      appSpan.title = 'Click to edit';
      appSpan.addEventListener('click', edit);
      wrap.appendChild(appSpan);

      var titleSpan = document.createElement('span');
      titleSpan.style.color = '#ccc';
      titleSpan.style.fontWeight = 'normal';
      titleSpan.textContent = step.window_title ? (' — ' + step.window_title) : '';
      titleSpan.title = 'Click to edit';
      titleSpan.addEventListener('click', edit);
      wrap.appendChild(titleSpan);
    }

    function edit() {
      wrap.innerHTML = '';
      var appInput = document.createElement('input');
      appInput.type = 'text';
      appInput.placeholder = 'App';
      appInput.value = step.window_app || '';
      var titleInput = document.createElement('input');
      titleInput.type = 'text';
      titleInput.placeholder = 'Window title';
      titleInput.value = step.window_title || '';
      wrap.appendChild(appInput);
      wrap.appendChild(titleInput);
      appInput.focus();
      appInput.select();

      var settled = false;
      function commit() {
        if (settled) return;
        settled = true;
        var app = appInput.value.trim(), title = titleInput.value.trim();
        if (app === (step.window_app || '') && title === (step.window_title || '')) { show(); return; }
        api('POST', '/api/window', {index: step.index, window_app: app, window_title: title}).then(function() {
          step.window_app = app;
          step.window_title = title;
          show();
        }).catch(function(e) { alert('Update failed: ' + e.message); show(); });
      }
      function cancel() {
        settled = true;
        show();
      }
      [appInput, titleInput].forEach(function(inp) {
        inp.addEventListener('blur', function() {
          // A blur caused by tabbing to the OTHER field in this pair
          // isn't "leaving" the edit — give focus a tick to land there
          // before deciding the user actually moved on.
          setTimeout(function() {
            if (!settled && document.activeElement !== appInput && document.activeElement !== titleInput) commit();
          }, 0);
        });
        inp.addEventListener('keydown', function(e) {
          if (e.key === 'Enter') { inp.blur(); }
          else if (e.key === 'Escape') { cancel(); }
        });
      });
    }

    show();
    return wrap;
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
    api('POST', '/api/reorder', {order: order}).then(function() {
      sessionData.display_order = order;
      render();
    });
  }

  function toggleHidden(idx) {
    var hidden = !isHidden(idx);
    api('POST', '/api/hide', {index: idx, hidden: hidden}).then(function() {
      sessionData.hidden = sessionData.hidden || [];
      var pos = sessionData.hidden.indexOf(idx);
      if (hidden && pos === -1) sessionData.hidden.push(idx);
      if (!hidden && pos !== -1) sessionData.hidden.splice(pos, 1);
      render();
    });
  }

  function imageSrc(step) {
    return '/images/' + step.image_file.split('/').pop();
  }

  // buildImageEditor builds one step's screenshot, its always-live
  // numbered annotation markers (buildAnnotationElement — draggable,
  // resizable, renumberable, deletable, reorderable in the stack,
  // committing immediately since they're non-destructive), and the
  // toolbar for redacting (destructive, draw -> adjust -> Apply, choice
  // of solid/blur style, undo/redo/reset) or adding a new highlight
  // (non-destructive, same draw -> adjust -> Apply shape, but Apply
  // creates an annotation instead of baking pixels — see decision #18).
  // It registers a stepEditors[step.index] entry so later API responses
  // can update this step's view directly (see the stepEditors comment).
  function buildImageEditor(step) {
    var imgWrap = document.createElement('div');
    imgWrap.className = 'img-wrap';

    var toolbar = document.createElement('div');
    toolbar.className = 'img-toolbar';
    var redactBtn = mkPlainBtn('Draw Redaction', 'redact-btn');
    var solidBtn = mkPlainBtn('Solid', 'style-selected');
    var blurBtn = mkPlainBtn('Blur');
    var highlightBtn = mkPlainBtn('Draw Highlight', 'highlight-btn');
    var sep1 = document.createElement('span');
    sep1.className = 'sep';
    var applyBtn = mkPlainBtn('Apply', 'apply-btn');
    var cancelBtn = mkPlainBtn('Cancel');
    var sep2 = document.createElement('span');
    sep2.className = 'sep';
    var undoBtn = mkPlainBtn('Undo');
    var redoBtn = mkPlainBtn('Redo');
    var resetBtn = mkPlainBtn('Reset');
    var modeHint = document.createElement('span');
    modeHint.className = 'mode-hint';
    [redactBtn, solidBtn, blurBtn, highlightBtn, sep1, applyBtn, cancelBtn, sep2, undoBtn, redoBtn, resetBtn, modeHint]
      .forEach(function(elm) { toolbar.appendChild(elm); });
    imgWrap.appendChild(toolbar);

    var redactStyle = 'solid';
    function setRedactStyle(style) {
      redactStyle = style;
      solidBtn.classList.toggle('style-selected', style === 'solid');
      blurBtn.classList.toggle('style-selected', style === 'blur');
    }
    solidBtn.addEventListener('click', function() { setRedactStyle('solid'); });
    blurBtn.addEventListener('click', function() { setRedactStyle('blur'); });

    var imgFrame = document.createElement('div');
    imgFrame.className = 'img-frame';

    var img = document.createElement('img');
    img.src = imageSrc(step);
    img.alt = 'Step ' + step.index + ' screenshot';
    imgFrame.appendChild(img);

    var overlay = document.createElement('div');
    overlay.className = 'drag-overlay';
    imgFrame.appendChild(overlay);

    var drawHandles = ['nw', 'ne', 'sw', 'se'].map(function(corner) {
      var h = document.createElement('div');
      h.className = 'resize-handle handle-' + corner;
      h.style.display = 'none';
      imgFrame.appendChild(h);
      return {corner: corner, el: h};
    });

    // renderAnnotations positions every annotation as a percentage of the
    // image's natural (decoded) size — not a pixel ratio derived from the
    // image's on-screen rendered size (img.clientWidth). naturalWidth/
    // naturalHeight are available as soon as the browser has decoded the
    // image, independent of whether it's attached to the document or has
    // been laid out yet, so this never depends on DOM-attachment timing
    // (the old bug: a cached image reports img.complete === true
    // immediately, so placement ran before imgFrame had been appended
    // and had a layout, reading clientWidth/Height as 0 and placing every
    // annotation as an invisible 0x0 box). Percentages also self-correct
    // on window resize for free, which pixel math never did.
    function renderAnnotations() {
      imgFrame.querySelectorAll('.annotation').forEach(function(a) { a.remove(); });
      (step.annotations || []).forEach(function(ann, pos) {
        imgFrame.appendChild(buildAnnotationElement(img, ann, pos, step, renderAnnotations));
      });
    }
    if (img.complete && img.naturalWidth) {
      renderAnnotations();
    } else {
      img.addEventListener('load', renderAnnotations);
    }

    imgWrap.appendChild(imgFrame);

    stepEditors[step.index] = {
      // setStepData is used after undo/redo/reset, which can change both
      // the image bytes and the annotation list atomically — it replaces
      // this step's local annotation data and re-renders the overlay, and
      // the caller separately calls bustImageCache() since the image
      // bytes may also have changed.
      setStepData: function(newStep) {
        step.annotations = newStep.annotations || [];
        renderAnnotations();
      },
      addAnnotationLocal: function(ann) {
        step.annotations = step.annotations || [];
        step.annotations.push(ann);
        renderAnnotations();
      },
      // bustImageCache forces a real re-fetch of the (possibly changed)
      // image bytes — the filename never changes on redact/undo/redo/
      // reset, only the bytes at that path, so the URL needs a
      // cache-busting query param or the browser would keep showing what
      // it already has cached for that exact URL.
      bustImageCache: function() {
        img.src = imageSrc(step) + '?t=' + Date.now();
      },
    };

    setupDrawing({
      img: img, imgFrame: imgFrame, overlay: overlay, handles: drawHandles,
      redactBtn: redactBtn, highlightBtn: highlightBtn,
      applyBtn: applyBtn, cancelBtn: cancelBtn,
      undoBtn: undoBtn, redoBtn: redoBtn, resetBtn: resetBtn,
      modeHint: modeHint, stepIndex: step.index,
      getRedactStyle: function() { return redactStyle; },
    });

    return imgWrap;
  }

  function mkPlainBtn(label, cls) {
    var b = document.createElement('button');
    b.textContent = label;
    if (cls) b.className = cls;
    return b;
  }

  // Unpins every pinned annotation's toolbar on an outside click.
  // Registered once at script scope (not per-annotation) so it never
  // needs cleanup across the full annotation-layer rebuilds
  // renderAnnotations() does on every add/delete/reorder — it just
  // queries whatever's currently in the document each time it fires.
  document.addEventListener('mousedown', function(e) {
    document.querySelectorAll('.annotation.pinned').forEach(function(p) {
      if (!p.contains(e.target)) p.classList.remove('pinned');
    });
  });

  // buildAnnotationElement renders one EXISTING annotation as a plain
  // highlighted rectangle with a small number badge by default — drag its
  // body to move it, drag a corner handle to resize it (both committing
  // via /api/annotation/update as soon as the drag ends — no separate
  // Apply, since editing an annotation that already exists isn't
  // destructive the way redaction is). All the other controls (renumber,
  // reorder in the stack, delete) live in a floating toolbar above the
  // annotation, shown only on hover or click — like a word processor's
  // selection toolbar — rather than permanently overlapping the badge.
  // Clicking the annotation body (a real click, not a drag) pins the
  // toolbar open; clicking anywhere outside it un-pins (see the
  // document-level listener above). Positions are percentages of the
  // image's natural size (see renderAnnotations for why); the
  // drag/resize/commit math still converts through the rendered size,
  // but only once a drag has actually started, by which point the
  // element is necessarily attached and laid out (the user is looking at
  // and dragging real on-screen pixels), so there's no placement-timing
  // hazard there.
  function buildAnnotationElement(img, ann, pos, step, onStackChanged) {
    var el = document.createElement('div');
    el.className = 'annotation';

    function place() {
      var topPct = ann.y / img.naturalHeight;
      el.style.left = (ann.x / img.naturalWidth * 100) + '%';
      el.style.top = (topPct * 100) + '%';
      el.style.width = (ann.w / img.naturalWidth * 100) + '%';
      el.style.height = (ann.h / img.naturalHeight * 100) + '%';
      // A toolbar floating above an annotation near the top of the image
      // would be clipped off the frame — flip it below instead.
      el.classList.toggle('toolbar-below', topPct < 0.08);
    }
    place();

    var badge = document.createElement('span');
    badge.className = 'annotation-badge';
    badge.textContent = ann.number;
    el.appendChild(badge);

    var toolbar = document.createElement('div');
    toolbar.className = 'annotation-toolbar';
    toolbar.addEventListener('mousedown', function(e) { e.stopPropagation(); });

    var numberInput = document.createElement('input');
    numberInput.type = 'number';
    numberInput.value = ann.number;
    numberInput.title = 'Renumber';
    toolbar.appendChild(numberInput);

    var sep1 = document.createElement('span');
    sep1.className = 'toolbar-sep';
    toolbar.appendChild(sep1);

    var backBtn = document.createElement('button');
    backBtn.textContent = '‹';
    backBtn.title = 'Send backward';
    backBtn.disabled = pos === 0;
    var frontBtn = document.createElement('button');
    frontBtn.textContent = '›';
    frontBtn.title = 'Bring forward';
    frontBtn.disabled = pos === (step.annotations || []).length - 1;
    toolbar.appendChild(backBtn);
    toolbar.appendChild(frontBtn);

    var sep2 = document.createElement('span');
    sep2.className = 'toolbar-sep';
    toolbar.appendChild(sep2);

    var delBtn = document.createElement('button');
    delBtn.className = 'annotation-delete-btn';
    delBtn.textContent = '×';
    delBtn.title = 'Delete this annotation';
    toolbar.appendChild(delBtn);

    el.appendChild(toolbar);

    function moveInStack(delta) {
      var anns = step.annotations || [];
      var newPos = pos + delta;
      if (newPos < 0 || newPos >= anns.length) return;
      var order = anns.map(function(a) { return a.id; });
      var tmp = order[pos];
      order[pos] = order[newPos];
      order[newPos] = tmp;
      api('POST', '/api/annotation/reorder', {index: step.index, order: order}).then(function() {
        var swapped = anns[pos];
        anns[pos] = anns[newPos];
        anns[newPos] = swapped;
        onStackChanged();
      }).catch(function(e) { alert('Reorder failed: ' + e.message); });
    }
    backBtn.addEventListener('click', function() { moveInStack(-1); });
    frontBtn.addEventListener('click', function() { moveInStack(1); });

    delBtn.addEventListener('click', function() {
      api('POST', '/api/annotation/delete', {index: step.index, id: ann.id}).then(function() {
        step.annotations = (step.annotations || []).filter(function(a) { return a.id !== ann.id; });
        onStackChanged();
      }).catch(function(e2) { alert('Delete failed: ' + e2.message); });
    });

    function commitNumber() {
      var number = parseInt(numberInput.value, 10);
      if (isNaN(number) || number === ann.number) { numberInput.value = ann.number; return; }
      var prev = ann.number;
      ann.number = number;
      badge.textContent = number;
      api('POST', '/api/annotation/update', {index: step.index, id: ann.id, x: ann.x, y: ann.y, w: ann.w, h: ann.h, number: number})
        .catch(function(e) { alert('Renumber failed: ' + e.message); ann.number = prev; badge.textContent = prev; numberInput.value = prev; });
    }
    numberInput.addEventListener('change', commitNumber);

    var handles = ['nw', 'ne', 'sw', 'se'].map(function(corner) {
      var h = document.createElement('div');
      h.className = 'resize-handle handle-' + corner;
      h.style.left = (corner.indexOf('w') !== -1 ? '0' : '100%');
      h.style.top = (corner.indexOf('n') !== -1 ? '0' : '100%');
      el.appendChild(h);
      return {corner: corner, el: h};
    });

    function commitPosition() {
      var sx = img.naturalWidth / img.clientWidth;
      var sy = img.naturalHeight / img.clientHeight;
      var x = el.offsetLeft * sx, y = el.offsetTop * sy;
      var w = el.offsetWidth * sx, h = el.offsetHeight * sy;
      ann.x = x; ann.y = y; ann.w = w; ann.h = h;
      api('POST', '/api/annotation/update', {index: step.index, id: ann.id, x: x, y: y, w: w, h: h, number: ann.number})
        .catch(function(e) { alert('Update failed: ' + e.message); onStackChanged(); });
    }

    var dragging = null;
    el.addEventListener('mousedown', function(e) {
      if (e.target !== el) return; // a child (badge/toolbar/handle) handles its own mousedown
      e.preventDefault();
      dragging = {type: 'move', moved: false, startX: e.clientX, startY: e.clientY, origLeft: el.offsetLeft, origTop: el.offsetTop};
    });
    handles.forEach(function(h) {
      h.el.addEventListener('mousedown', function(e) {
        e.preventDefault();
        e.stopPropagation();
        dragging = {
          type: 'resize', moved: true, corner: h.corner, startX: e.clientX, startY: e.clientY,
          origLeft: el.offsetLeft, origTop: el.offsetTop, origW: el.offsetWidth, origH: el.offsetHeight,
        };
      });
    });
    window.addEventListener('mousemove', function(e) {
      if (!dragging) return;
      var dx = e.clientX - dragging.startX, dy = e.clientY - dragging.startY;
      if (Math.abs(dx) > 3 || Math.abs(dy) > 3) dragging.moved = true;
      if (dragging.type === 'move') {
        el.style.left = (dragging.origLeft + dx) + 'px';
        el.style.top = (dragging.origTop + dy) + 'px';
        return;
      }
      var left = dragging.origLeft, top = dragging.origTop;
      var right = dragging.origLeft + dragging.origW, bottom = dragging.origTop + dragging.origH;
      if (dragging.corner.indexOf('n') !== -1) top = dragging.origTop + dy;
      if (dragging.corner.indexOf('s') !== -1) bottom = dragging.origTop + dragging.origH + dy;
      if (dragging.corner.indexOf('w') !== -1) left = dragging.origLeft + dx;
      if (dragging.corner.indexOf('e') !== -1) right = dragging.origLeft + dragging.origW + dx;
      el.style.left = Math.min(left, right) + 'px';
      el.style.top = Math.min(top, bottom) + 'px';
      el.style.width = Math.abs(right - left) + 'px';
      el.style.height = Math.abs(bottom - top) + 'px';
    });
    window.addEventListener('mouseup', function() {
      if (!dragging) return;
      var wasPlainClick = dragging.type === 'move' && !dragging.moved;
      dragging = null;
      if (wasPlainClick) {
        // A click with no drag toggles the toolbar open/closed (pinned),
        // same as clicking selected text in a word processor — no
        // position actually changed, so there's nothing to commit.
        el.classList.toggle('pinned');
      } else {
        commitPosition();
      }
    });

    return el;
  }

  // setupDrawing wires up the "draw a NEW rectangle" flow shared by both
  // Draw Redaction and Draw Highlight: arm a mode, drag on the image to
  // sketch a rectangle, adjust it (move the body, resize from a corner
  // handle), then Apply — which POSTs to /api/edit-image for redaction
  // (destructive, pixel-mutating, solid or blur per getRedactStyle()) or
  // /api/annotation for a highlight (creates a new, subsequently always-
  // editable annotation, applied live via addAnnotationLocal — no
  // reload). Nothing is sent to the server until Apply either way.
  //
  // All coordinates are tracked relative to imgFrame — the element that
  // directly and only wraps the <img> and its overlay/handles/existing
  // annotations, so a pixel measured relative to imgFrame's top-left
  // lands in exactly the same spot whether computing where the mouse is
  // or where to draw the overlay.
  function setupDrawing(el) {
    var armedMode = null;
    var pending = null; // {mode, x, y, w, h} in on-screen px, relative to imgFrame
    var dragging = null;

    function setArmed(mode) {
      armedMode = mode;
      el.img.style.cursor = mode ? 'crosshair' : 'default';
      el.modeHint.textContent = mode ? ('Drag on the image to draw a ' + mode + '.') : '';
      el.redactBtn.classList.toggle('armed', mode === 'redact');
      el.highlightBtn.classList.toggle('armed', mode === 'highlight');
    }

    function updateButtonStates() {
      el.redactBtn.disabled = !!pending;
      el.highlightBtn.disabled = !!pending;
    }

    function framePos(e) {
      var rect = el.imgFrame.getBoundingClientRect();
      return {x: e.clientX - rect.left, y: e.clientY - rect.top};
    }

    function renderOverlay() {
      if (!pending) {
        el.overlay.style.display = 'none';
        el.handles.forEach(function(h) { h.el.style.display = 'none'; });
        return;
      }
      el.overlay.className = 'drag-overlay ' + pending.mode;
      el.overlay.style.display = 'block';
      el.overlay.style.left = pending.x + 'px';
      el.overlay.style.top = pending.y + 'px';
      el.overlay.style.width = pending.w + 'px';
      el.overlay.style.height = pending.h + 'px';
      el.handles.forEach(function(h) {
        h.el.style.display = 'block';
        h.el.style.left = (h.corner.indexOf('w') !== -1 ? pending.x : pending.x + pending.w) + 'px';
        h.el.style.top = (h.corner.indexOf('n') !== -1 ? pending.y : pending.y + pending.h) + 'px';
      });
    }

    function showApplyBar(show) {
      el.applyBtn.style.display = show ? 'inline-block' : 'none';
      el.cancelBtn.style.display = show ? 'inline-block' : 'none';
    }
    showApplyBar(false);

    el.redactBtn.addEventListener('click', function() {
      if (pending) return;
      setArmed(armedMode === 'redact' ? null : 'redact');
    });
    el.highlightBtn.addEventListener('click', function() {
      if (pending) return;
      setArmed(armedMode === 'highlight' ? null : 'highlight');
    });

    el.img.addEventListener('mousedown', function(e) {
      if (!armedMode || pending) return;
      e.preventDefault();
      var p = framePos(e);
      dragging = {type: 'draw', startX: p.x, startY: p.y};
      pending = {mode: armedMode, x: p.x, y: p.y, w: 0, h: 0};
      renderOverlay();
    });

    el.overlay.addEventListener('mousedown', function(e) {
      if (!pending) return;
      e.preventDefault();
      var p = framePos(e);
      dragging = {type: 'move', startX: p.x, startY: p.y, orig: {x: pending.x, y: pending.y, w: pending.w, h: pending.h}};
    });

    el.handles.forEach(function(h) {
      h.el.addEventListener('mousedown', function(e) {
        if (!pending) return;
        e.preventDefault();
        e.stopPropagation(); // don't also trigger the overlay's move-drag
        dragging = {type: 'resize', corner: h.corner, orig: {x: pending.x, y: pending.y, w: pending.w, h: pending.h}};
      });
    });

    window.addEventListener('mousemove', function(e) {
      if (!dragging || !pending) return;
      var p = framePos(e);
      if (dragging.type === 'draw') {
        pending.x = Math.min(dragging.startX, p.x);
        pending.y = Math.min(dragging.startY, p.y);
        pending.w = Math.abs(p.x - dragging.startX);
        pending.h = Math.abs(p.y - dragging.startY);
      } else if (dragging.type === 'move') {
        pending.x = dragging.orig.x + (p.x - dragging.startX);
        pending.y = dragging.orig.y + (p.y - dragging.startY);
      } else if (dragging.type === 'resize') {
        var o = dragging.orig;
        var left = o.x, top = o.y, right = o.x + o.w, bottom = o.y + o.h;
        if (dragging.corner.indexOf('n') !== -1) top = p.y;
        if (dragging.corner.indexOf('s') !== -1) bottom = p.y;
        if (dragging.corner.indexOf('w') !== -1) left = p.x;
        if (dragging.corner.indexOf('e') !== -1) right = p.x;
        pending.x = Math.min(left, right);
        pending.y = Math.min(top, bottom);
        pending.w = Math.abs(right - left);
        pending.h = Math.abs(bottom - top);
      }
      renderOverlay();
    });

    window.addEventListener('mouseup', function() {
      if (!dragging) return;
      var wasTinyDraw = dragging.type === 'draw' && pending && (pending.w < 4 || pending.h < 4);
      dragging = null;
      if (wasTinyDraw) {
        // Too small to be an intentional rectangle -- a stray click while
        // armed, not a shape to adjust.
        pending = null;
        setArmed(null);
        renderOverlay();
        showApplyBar(false);
        updateButtonStates();
        return;
      }
      if (pending) {
        showApplyBar(true);
        updateButtonStates();
      }
    });

    el.applyBtn.addEventListener('click', function() {
      if (!pending) return;
      var mode = pending.mode;
      var scaleX = el.img.naturalWidth / el.img.clientWidth;
      var scaleY = el.img.naturalHeight / el.img.clientHeight;
      var rect = {
        x: pending.x * scaleX, y: pending.y * scaleY,
        w: pending.w * scaleX, h: pending.h * scaleY,
      };
      var request = mode === 'redact'
        ? apiJSON('POST', '/api/edit-image', Object.assign({index: el.stepIndex, style: el.getRedactStyle()}, rect))
        : apiJSON('POST', '/api/annotation', Object.assign({index: el.stepIndex}, rect));
      request.then(function(data) {
        pending = null;
        setArmed(null);
        renderOverlay();
        showApplyBar(false);
        updateButtonStates();
        var editor = stepEditors[el.stepIndex];
        if (mode === 'redact') {
          editor.setStepData(data);
          editor.bustImageCache();
        } else {
          editor.addAnnotationLocal(data);
        }
      }).catch(function(e) { alert('Edit failed: ' + e.message); });
    });

    el.cancelBtn.addEventListener('click', function() {
      pending = null;
      setArmed(null);
      renderOverlay();
      showApplyBar(false);
      updateButtonStates();
    });

    el.undoBtn.addEventListener('click', function() {
      apiJSON('POST', '/api/undo', {index: el.stepIndex}).then(function(step) {
        var editor = stepEditors[el.stepIndex];
        editor.setStepData(step);
        editor.bustImageCache();
      }).catch(function(e) { alert(e.message || 'Nothing to undo.'); });
    });
    el.redoBtn.addEventListener('click', function() {
      apiJSON('POST', '/api/redo', {index: el.stepIndex}).then(function(step) {
        var editor = stepEditors[el.stepIndex];
        editor.setStepData(step);
        editor.bustImageCache();
      }).catch(function(e) { alert(e.message || 'Nothing to redo.'); });
    });
    el.resetBtn.addEventListener('click', function() {
      if (!confirm('Discard all edits made to this screenshot in this editing session?')) return;
      apiJSON('POST', '/api/reset', {index: el.stepIndex}).then(function(step) {
        var editor = stepEditors[el.stepIndex];
        editor.setStepData(step);
        editor.bustImageCache();
      }).catch(function(e) { alert(e.message || 'Nothing to reset.'); });
    });
  }

  load();
`
