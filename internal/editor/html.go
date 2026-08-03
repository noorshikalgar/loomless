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
  .annotation-badge { position: absolute; top: -12px; left: -12px; width: 24px; height: 24px; border-radius: 50%; background: #ff3b30; color: #fff; border: 1px solid #a03c3c; text-align: center; font-size: 0.7rem; padding: 0; font-family: inherit; z-index: 4; }
  .annotation-badge::-webkit-inner-spin-button, .annotation-badge::-webkit-outer-spin-button { -webkit-appearance: none; margin: 0; }
  .annotation-delete { position: absolute; top: -10px; right: -10px; width: 18px; height: 18px; border-radius: 50%; background: #444; color: #fff; border: 1px solid #666; font-size: 0.7rem; line-height: 1; cursor: pointer; padding: 0; z-index: 4; }
  .annotation-delete:hover { background: #666; }
  .annotation-stack { position: absolute; top: -10px; right: 14px; display: flex; gap: 2px; z-index: 4; }
  .annotation-stack button { width: 16px; height: 16px; border-radius: 3px; background: #444; color: #fff; border: 1px solid #666; font-size: 0.65rem; line-height: 1; cursor: pointer; padding: 0; }
  .annotation-stack button:hover { background: #666; }
  .annotation-stack button:disabled { opacity: 0.3; cursor: default; }
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

  // buildAnnotationElement renders one EXISTING annotation as an
  // always-interactive numbered marker: drag its body to move it, drag a
  // corner handle to resize it (both committing via /api/annotation/update
  // as soon as the drag ends — no separate Apply, since editing an
  // annotation that already exists isn't destructive the way redaction
  // is), edit the number badge directly, reorder it in the stack
  // (front/back — array order is paint/z-order, so "front" moves an
  // annotation later in step.annotations), or delete it. Positions are
  // percentages of the image's natural size (see renderAnnotations for
  // why); the drag/resize/commit math still converts through the
  // rendered size, but only once a drag has actually started, by which
  // point the element is necessarily attached and laid out (the user is
  // looking at and dragging real on-screen pixels), so there's no
  // placement-timing hazard there.
  function buildAnnotationElement(img, ann, pos, step, onStackChanged) {
    var el = document.createElement('div');
    el.className = 'annotation';

    function place() {
      el.style.left = (ann.x / img.naturalWidth * 100) + '%';
      el.style.top = (ann.y / img.naturalHeight * 100) + '%';
      el.style.width = (ann.w / img.naturalWidth * 100) + '%';
      el.style.height = (ann.h / img.naturalHeight * 100) + '%';
    }
    place();

    var badge = document.createElement('input');
    badge.type = 'number';
    badge.className = 'annotation-badge';
    badge.value = ann.number;
    badge.addEventListener('click', function(e) { e.stopPropagation(); });
    badge.addEventListener('mousedown', function(e) { e.stopPropagation(); });
    el.appendChild(badge);

    var del = document.createElement('button');
    del.className = 'annotation-delete';
    del.textContent = '×';
    del.title = 'Delete this annotation';
    del.addEventListener('mousedown', function(e) { e.stopPropagation(); });
    del.addEventListener('click', function(e) {
      e.stopPropagation();
      api('POST', '/api/annotation/delete', {index: step.index, id: ann.id}).then(function() {
        step.annotations = (step.annotations || []).filter(function(a) { return a.id !== ann.id; });
        onStackChanged();
      }).catch(function(e2) { alert('Delete failed: ' + e2.message); });
    });
    el.appendChild(del);

    var stackCtl = document.createElement('div');
    stackCtl.className = 'annotation-stack';
    var backBtn = document.createElement('button');
    backBtn.textContent = '‹';
    backBtn.title = 'Send backward';
    backBtn.disabled = pos === 0;
    var frontBtn = document.createElement('button');
    frontBtn.textContent = '›';
    frontBtn.title = 'Bring forward';
    frontBtn.disabled = pos === (step.annotations || []).length - 1;
    [backBtn, frontBtn].forEach(function(b) {
      b.addEventListener('mousedown', function(e) { e.stopPropagation(); });
    });
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
    backBtn.addEventListener('click', function(e) { e.stopPropagation(); moveInStack(-1); });
    frontBtn.addEventListener('click', function(e) { e.stopPropagation(); moveInStack(1); });
    stackCtl.appendChild(backBtn);
    stackCtl.appendChild(frontBtn);
    el.appendChild(stackCtl);

    var handles = ['nw', 'ne', 'sw', 'se'].map(function(corner) {
      var h = document.createElement('div');
      h.className = 'resize-handle handle-' + corner;
      h.style.left = (corner.indexOf('w') !== -1 ? '0' : '100%');
      h.style.top = (corner.indexOf('n') !== -1 ? '0' : '100%');
      el.appendChild(h);
      return {corner: corner, el: h};
    });

    function commit() {
      var sx = img.naturalWidth / img.clientWidth;
      var sy = img.naturalHeight / img.clientHeight;
      var x = el.offsetLeft * sx, y = el.offsetTop * sy;
      var w = el.offsetWidth * sx, h = el.offsetHeight * sy;
      var number = parseInt(badge.value, 10);
      if (isNaN(number)) number = ann.number;
      ann.x = x; ann.y = y; ann.w = w; ann.h = h; ann.number = number;
      api('POST', '/api/annotation/update', {index: step.index, id: ann.id, x: x, y: y, w: w, h: h, number: number})
        .catch(function(e) { alert('Update failed: ' + e.message); onStackChanged(); });
    }

    var dragging = null;
    el.addEventListener('mousedown', function(e) {
      if (e.target !== el) return; // a child (badge/delete/stack/handle) handles its own mousedown
      e.preventDefault();
      dragging = {type: 'move', startX: e.clientX, startY: e.clientY, origLeft: el.offsetLeft, origTop: el.offsetTop};
    });
    handles.forEach(function(h) {
      h.el.addEventListener('mousedown', function(e) {
        e.preventDefault();
        e.stopPropagation();
        dragging = {
          type: 'resize', corner: h.corner, startX: e.clientX, startY: e.clientY,
          origLeft: el.offsetLeft, origTop: el.offsetTop, origW: el.offsetWidth, origH: el.offsetHeight,
        };
      });
    });
    window.addEventListener('mousemove', function(e) {
      if (!dragging) return;
      var dx = e.clientX - dragging.startX, dy = e.clientY - dragging.startY;
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
      dragging = null;
      commit();
    });

    badge.addEventListener('change', commit);

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
