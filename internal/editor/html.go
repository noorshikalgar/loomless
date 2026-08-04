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
<div class="editor-modal-overlay" id="editorModalOverlay">
  <div class="editor-modal">
    <button class="editor-modal-close" id="editorModalClose">&times; Close</button>
    <div class="editor-modal-label" id="editorModalLabel"></div>
    <div class="editor-modal-body" id="editorModalBody"></div>
  </div>
</div>
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
  .img-toolbar button.armed.crop-btn { background: #1a5a2e; border-color: #2c7c44; }
  .img-toolbar button.armed.freehand-btn { background: #3a2a6a; border-color: #5c4c9c; }
  .freehand-preview { position: absolute; left: 0; top: 0; width: 100%; height: 100%; display: none; pointer-events: none; z-index: 2; overflow: visible; }
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
  .drag-overlay.crop { border-color: #7ee787; background: rgba(126, 231, 135, 0.1); }
  /* The saved crop boundary (once Applied, not the in-progress
     drag-overlay above) — a dashed line plus four opaque bands covering
     everything OUTSIDE the kept region, so it's immediately obvious what
     will and won't survive export. Four separate divs rather than one
     overflow:hidden box-shadow trick, since .img-frame can't safely clip
     overflow — a resize handle on an annotation near the image's edge
     deliberately sits a few px outside the annotation's own box. */
  .crop-mask { position: absolute; display: none; background: rgba(0, 0, 0, 0.65); pointer-events: none; z-index: 2; }
  .crop-boundary { position: absolute; display: none; border: 2px dashed #7ee787; box-sizing: border-box; pointer-events: none; z-index: 3; }
  .resize-handle { position: absolute; width: 10px; height: 10px; margin: -5px; background: #fff; border: 1px solid #333; border-radius: 2px; z-index: 2; }
  .handle-nw, .handle-se { cursor: nwse-resize; }
  .handle-ne, .handle-sw { cursor: nesw-resize; }
  .annotation { position: absolute; border: 3px solid #ff3b30; box-sizing: border-box; cursor: move; z-index: 3; }
  .annotation.freehand { border: none; }
  .annotation.freehand svg { display: block; }
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
  .annotation-toolbar input.annotation-color-input { -webkit-appearance: none; appearance: none; width: 22px; height: 22px; padding: 0; border: 1px solid #444; border-radius: 4px; cursor: pointer; background: none; }
  .annotation-toolbar input.annotation-color-input::-webkit-color-swatch-wrapper { padding: 2px; }
  .annotation-toolbar input.annotation-color-input::-webkit-color-swatch { border: none; border-radius: 2px; }
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
  .window-line input { font-size: 0.9rem; background: #17171a; border: 1px solid #444; border-radius: 4px; padding: 0.1rem 0.35rem; font-family: inherit; }
  /* App and window-title fields deliberately don't look like clones of
     each other: same visual split as the display-mode text (bold/blue
     app, plain/gray title), a dash between them so it reads as one
     "App — Title" line being edited, and different widths since they're
     conceptually different-length fields. */
  .window-line input.window-app-input { font-weight: 600; color: #8ab4ff; width: 8rem; }
  .window-line input.window-title-input { font-weight: normal; color: #ccc; flex: 1; min-width: 0; }
  .window-line-sep { color: #666; }
  /* A content step (decision #29) is visually distinguished only by a
     subtle left accent — same card shape as every other step, since
     it's still a first-class step (reorderable, hideable), just one
     that was authored instead of captured. */
  .step.content-step { border-left: 3px solid #8ab4ff; }
  .insert-step-row { display: flex; justify-content: center; margin: -0.4rem 0 0.6rem; }
  .insert-step-btn { background: transparent; border: 1px dashed #444; color: #777; border-radius: 4px; padding: 0.15rem 0.6rem; font-size: 0.75rem; cursor: pointer; }
  .insert-step-btn:hover { border-color: #8ab4ff; color: #8ab4ff; }
  /* Focused/"modal" view for an image editor — see the openFocusView
     comment in editorJS for why this reparents the existing imgWrap
     rather than building a second editor. Fixed positioning means this
     ignores the body's 900px reading-width cap and can use the full
     viewport, which is the entire point: precise crop/redact/freehand
     work is a lot easier on a screenshot rendered near its real size
     than one squeezed into a narrow reading column. */
  .editor-modal-overlay { display: none; position: fixed; inset: 0; background: rgba(0,0,0,0.85); z-index: 100; align-items: center; justify-content: center; padding: 2rem; }
  .editor-modal-overlay.open { display: flex; }
  .editor-modal { position: relative; background: #1a1a1e; border: 1px solid #333; border-radius: 8px; padding: 1rem; max-width: 96vw; max-height: 92vh; overflow: auto; }
  .editor-modal-close { position: absolute; top: 0.6rem; right: 0.6rem; background: #2a2a30; color: #f0f0f0; border: 1px solid #444; border-radius: 4px; padding: 0.35rem 0.7rem; cursor: pointer; font-size: 0.85rem; }
  .editor-modal-close:hover { background: #3a3a42; }
  .editor-modal-label { color: #8ab4ff; font-weight: 600; margin: 0 3.5rem 0.75rem 0; font-size: 1rem; }
  .editor-modal-body .img-frame img { max-height: 76vh; width: auto; max-width: 88vw; }
  .editor-modal-body .expand-btn { display: none; }
  body.modal-open { overflow: hidden; }
  .expand-btn { position: absolute; top: 8px; right: 8px; z-index: 6; background: rgba(20,20,24,0.75); color: #f0f0f0; border: 1px solid #555; border-radius: 4px; width: 26px; height: 26px; font-size: 0.9rem; line-height: 1; cursor: pointer; display: flex; align-items: center; justify-content: center; }
  .expand-btn:hover { background: rgba(40,40,48,0.9); }
`

const editorJS = `
  // Matches capture.DefaultAnnotationColor (Go) exactly — an annotation
  // with no stored color renders as this on both sides (editor overlay
  // and every flattened export), so keep them in sync if either changes.
  var DEFAULT_ANNOTATION_COLOR = '#ff3b30';

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

  // Focused/"modal" editing view. A screenshot squeezed into the page's
  // 900px reading column is fine for a quick glance, but precise work —
  // circling one line of a stack trace, drawing a tight redaction around
  // a credential, cropping right at an edge — needs the image rendered
  // close to its real size, not shrunk down. Rather than build a second,
  // parallel editor for that, this reparents the SAME imgWrap node
  // buildImageEditor already built (and already wired every listener
  // onto, as closures over its own img/imgFrame/etc.) into a large
  // centered overlay, then moves it back to its original spot on close.
  // Every position in that subtree is already computed as a percentage
  // of the image's natural size or a ratio measured fresh at drag time
  // (see renderAnnotations' and setupDrawing's own comments), so
  // rendering at a much larger on-screen size needs no code changes
  // anywhere else at all.
  var modalOverlay = document.getElementById('editorModalOverlay');
  var modalBody = document.getElementById('editorModalBody');
  var modalLabel = document.getElementById('editorModalLabel');
  var modalReturn = null; // {node, parent, next} — where imgWrap came from, to restore on close

  function openFocusView(imgWrap, label) {
    modalReturn = {node: imgWrap, parent: imgWrap.parentNode, next: imgWrap.nextSibling};
    modalLabel.textContent = label;
    modalBody.appendChild(imgWrap);
    modalOverlay.classList.add('open');
    document.body.classList.add('modal-open');
  }

  // Nothing behind the overlay is reachable while it's open (it covers
  // the full viewport), so the step list can't re-render out from under
  // modalReturn.parent between open and close — it's always still the
  // live DOM node imgWrap started in.
  function closeFocusView() {
    if (!modalReturn) return;
    if (modalReturn.next) {
      modalReturn.parent.insertBefore(modalReturn.node, modalReturn.next);
    } else {
      modalReturn.parent.appendChild(modalReturn.node);
    }
    modalReturn = null;
    modalOverlay.classList.remove('open');
    document.body.classList.remove('modal-open');
  }

  document.getElementById('editorModalClose').addEventListener('click', closeFocusView);
  modalOverlay.addEventListener('mousedown', function(e) { if (e.target === modalOverlay) closeFocusView(); });
  document.addEventListener('keydown', function(e) { if (e.key === 'Escape' && modalReturn) closeFocusView(); });

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

    var visiblePos = 0;
    order.forEach(function(idx, pos) {
      var step = stepByIndex(idx);
      if (!step) return;
      var hidden = isHidden(idx);
      if (!hidden) visiblePos++;
      // The default "Step N" heading numbers by position among VISIBLE
      // steps in the CURRENT display order (matching exactly what the
      // exporters compute over VisibleSteps()) — not step.index, which
      // is a stable but frozen capture-time reference (decision #4) that
      // doesn't move when steps are reordered. A hidden step shows the
      // position it would take if shown right now, since it isn't
      // counted in the real (exported) numbering at all.
      var displayPos = hidden ? (visiblePos + 1) : visiblePos;

      container.appendChild(buildInsertControl(pos));

      var el = document.createElement('div');
      el.className = 'step' + (hidden ? ' hidden-step' : '') + (step.is_content ? ' content-step' : '');
      el.setAttribute('data-step-index', step.index);

      var header = document.createElement('div');
      header.className = 'step-header';
      header.appendChild(buildStepTitle(step, displayPos));

      var upBtn = mkBtn('↑', pos === 0, function() { moveStep(pos, -1); });
      var downBtn = mkBtn('↓', pos === order.length - 1, function() { moveStep(pos, 1); });
      var hideBtn = mkBtn(isHidden(idx) ? 'Show' : 'Hide', false, function() { toggleHidden(idx); });
      header.appendChild(upBtn);
      header.appendChild(downBtn);
      header.appendChild(hideBtn);
      el.appendChild(header);

      // A content step (decision #29) is purely authored, with no
      // captured window to attribute — showing the "+ add app"
      // click-to-edit affordance there would invite the user to
      // attribute a paragraph of prose to an application, which makes
      // no sense. A real capture-time note step (no image, but still
      // captured live) keeps the window line, since it may genuinely
      // have — or be worth adding — real window context.
      if (!step.command && !step.is_content) {
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
        if (step.is_content) ta.placeholder = 'Write something in Markdown — headings, lists, links, images, a pasted YouTube/Vimeo link on its own line…';
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
        row.appendChild(buildInsertImageControl(ta));
        row.appendChild(savedHint);
        row.appendChild(saveBtn);
        el.appendChild(row);

        if (step.image_file) {
          el.appendChild(buildImageEditor(step, displayPos));
        }
      }

      container.appendChild(el);
    });
    container.appendChild(buildInsertControl(order.length));
  }

  // buildInsertControl renders a slim "+ Insert step" control that
  // creates a new, empty content step (decision #29) right at this
  // point in the story — between two already-captured steps, not just
  // appended to the end, which is the whole reason this needs its own
  // endpoint rather than reusing the capture-time AddStep path (see
  // InsertContentStep's own doc comment). position is this control's
  // index into fullOrder(), i.e. exactly where the new step should land.
  function buildInsertControl(position) {
    var wrap = document.createElement('div');
    wrap.className = 'insert-step-row';
    var btn = document.createElement('button');
    btn.className = 'insert-step-btn';
    btn.textContent = '+ Insert step';
    btn.addEventListener('click', function() {
      apiJSON('POST', '/api/insert-step', {position: position}).then(function(resp) {
        sessionData = resp.session;
        render();
        var card = document.querySelector('[data-step-index="' + resp.new_index + '"]');
        var ta = card && card.querySelector('textarea');
        if (ta) {
          ta.focus();
          ta.scrollIntoView({block: 'center'});
        }
      }).catch(function(e) { alert('Insert failed: ' + e.message); });
    });
    wrap.appendChild(btn);
    return wrap;
  }

  // buildInsertImageControl renders the "Insert Image" button (plus its
  // hidden file input) shared by every note/content textarea — uploads
  // via /api/upload-image, then inserts a Markdown image reference at
  // the textarea's current cursor position, exactly the way a GitHub
  // issue's comment box handles a pasted image. The upload and the note
  // save are two separate, deliberately un-linked steps (matching the
  // existing "Save caption" button's own explicit-save pattern) — the
  // image lands on disk immediately, but doesn't become part of the
  // step's actual Note text until the user clicks Save.
  function buildInsertImageControl(ta) {
    var wrap = document.createElement('span');
    var btn = document.createElement('button');
    btn.type = 'button';
    btn.textContent = 'Insert Image';
    var fileInput = document.createElement('input');
    fileInput.type = 'file';
    fileInput.accept = 'image/png,image/jpeg';
    fileInput.style.display = 'none';
    btn.addEventListener('click', function() { fileInput.click(); });
    fileInput.addEventListener('change', function() {
      var file = fileInput.files[0];
      fileInput.value = '';
      if (!file) return;
      btn.disabled = true;
      var reader = new FileReader();
      reader.onload = function() {
        var dataUrl = String(reader.result);
        var base64 = dataUrl.substring(dataUrl.indexOf(',') + 1);
        apiJSON('POST', '/api/upload-image', {data: base64}).then(function(res) {
          var snippet = '![](' + res.filename + ')';
          var start = ta.selectionStart != null ? ta.selectionStart : ta.value.length;
          var end = ta.selectionEnd != null ? ta.selectionEnd : start;
          var needsLeadingBreak = start > 0 && ta.value[start - 1] !== '\n';
          var insert = (needsLeadingBreak ? '\n\n' : '') + snippet + '\n\n';
          ta.value = ta.value.slice(0, start) + insert + ta.value.slice(end);
          ta.focus();
          var caret = start + insert.length;
          ta.setSelectionRange(caret, caret);
        }).catch(function(e) { alert('Image upload failed: ' + e.message); })
          .then(function() { btn.disabled = false; });
      };
      reader.readAsDataURL(file);
    });
    wrap.appendChild(btn);
    wrap.appendChild(fileInput);
    return wrap;
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
  function buildStepTitle(step, position) {
    var wrap = document.createElement('div');
    wrap.className = 'step-title';

    function defaultTitle() { return 'Step ' + position; }

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
      appInput.className = 'window-app-input';
      var sep = document.createElement('span');
      sep.className = 'window-line-sep';
      sep.textContent = '—';
      var titleInput = document.createElement('input');
      titleInput.type = 'text';
      titleInput.placeholder = 'Window title';
      titleInput.value = step.window_title || '';
      titleInput.className = 'window-title-input';
      wrap.appendChild(appInput);
      wrap.appendChild(sep);
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
  function buildImageEditor(step, position) {
    var imgWrap = document.createElement('div');
    imgWrap.className = 'img-wrap';

    var toolbar = document.createElement('div');
    toolbar.className = 'img-toolbar';
    var redactBtn = mkPlainBtn('Draw Redaction', 'redact-btn');
    var solidBtn = mkPlainBtn('Solid', 'style-selected');
    var blurBtn = mkPlainBtn('Blur');
    var highlightBtn = mkPlainBtn('Draw Highlight', 'highlight-btn');
    var freehandBtn = mkPlainBtn('Draw Freehand', 'freehand-btn');
    var cropBtn = mkPlainBtn('Draw Crop', 'crop-btn');
    var clearCropBtn = mkPlainBtn('Clear Crop');
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
    [redactBtn, solidBtn, blurBtn, highlightBtn, freehandBtn, cropBtn, clearCropBtn, sep1, applyBtn, cancelBtn, sep2, undoBtn, redoBtn, resetBtn, modeHint]
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
    img.alt = 'Step ' + position + ' screenshot';
    imgFrame.appendChild(img);

    var overlay = document.createElement('div');
    overlay.className = 'drag-overlay';
    imgFrame.appendChild(overlay);

    var expandBtn = document.createElement('button');
    expandBtn.className = 'expand-btn';
    expandBtn.title = 'Open focused view';
    expandBtn.textContent = '⤢';
    expandBtn.addEventListener('click', function(e) {
      e.stopPropagation();
      openFocusView(imgWrap, step.title || ('Step ' + position));
    });
    imgFrame.appendChild(expandBtn);

    // Live preview while actively drawing a NEW freehand stroke — an SVG
    // sized to the whole frame (not just a sub-region like the crop/
    // redact/highlight drag-overlay, since a stroke's own extent isn't
    // known ahead of time) with a viewBox in the image's natural pixel
    // space, so points captured during drawing can be written straight
    // into the polyline with no coordinate conversion at all.
    var freehandPreviewSvg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    freehandPreviewSvg.setAttribute('class', 'freehand-preview');
    var freehandPreviewPoly = document.createElementNS('http://www.w3.org/2000/svg', 'polyline');
    freehandPreviewPoly.setAttribute('fill', 'none');
    freehandPreviewPoly.setAttribute('stroke-width', '4');
    freehandPreviewPoly.setAttribute('stroke-linecap', 'round');
    freehandPreviewPoly.setAttribute('stroke-linejoin', 'round');
    freehandPreviewPoly.setAttribute('stroke', DEFAULT_ANNOTATION_COLOR);
    freehandPreviewSvg.appendChild(freehandPreviewPoly);
    imgFrame.appendChild(freehandPreviewSvg);

    var drawHandles = ['nw', 'ne', 'sw', 'se'].map(function(corner) {
      var h = document.createElement('div');
      h.className = 'resize-handle handle-' + corner;
      h.style.display = 'none';
      imgFrame.appendChild(h);
      return {corner: corner, el: h};
    });

    // The saved crop boundary, if any: a dashed outline plus four bands
    // covering everything outside it. Built once and just toggled/
    // repositioned on every renderCrop() call, unlike annotations (which
    // rebuild from scratch each time) — there's at most one crop, so
    // there's no list-churn to simplify by throwing the DOM away.
    var cropTop = document.createElement('div');
    cropTop.className = 'crop-mask';
    var cropBottom = document.createElement('div');
    cropBottom.className = 'crop-mask';
    var cropLeft = document.createElement('div');
    cropLeft.className = 'crop-mask';
    var cropRight = document.createElement('div');
    cropRight.className = 'crop-mask';
    var cropBoundary = document.createElement('div');
    cropBoundary.className = 'crop-boundary';
    [cropTop, cropBottom, cropLeft, cropRight, cropBoundary].forEach(function(el) { imgFrame.appendChild(el); });

    function renderCrop() {
      var crop = step.crop;
      clearCropBtn.disabled = !crop;
      if (!crop) {
        [cropTop, cropBottom, cropLeft, cropRight, cropBoundary].forEach(function(el) { el.style.display = 'none'; });
        return;
      }
      var leftPct = crop.x / img.naturalWidth * 100;
      var topPct = crop.y / img.naturalHeight * 100;
      var wPct = crop.w / img.naturalWidth * 100;
      var hPct = crop.h / img.naturalHeight * 100;
      var rightPct = leftPct + wPct;
      var bottomPct = topPct + hPct;

      cropBoundary.style.display = 'block';
      cropBoundary.style.left = leftPct + '%';
      cropBoundary.style.top = topPct + '%';
      cropBoundary.style.width = wPct + '%';
      cropBoundary.style.height = hPct + '%';

      cropTop.style.display = 'block';
      cropTop.style.left = '0'; cropTop.style.top = '0';
      cropTop.style.width = '100%'; cropTop.style.height = topPct + '%';

      cropBottom.style.display = 'block';
      cropBottom.style.left = '0'; cropBottom.style.top = bottomPct + '%';
      cropBottom.style.width = '100%'; cropBottom.style.height = (100 - bottomPct) + '%';

      cropLeft.style.display = 'block';
      cropLeft.style.left = '0'; cropLeft.style.top = topPct + '%';
      cropLeft.style.width = leftPct + '%'; cropLeft.style.height = hPct + '%';

      cropRight.style.display = 'block';
      cropRight.style.left = rightPct + '%'; cropRight.style.top = topPct + '%';
      cropRight.style.width = (100 - rightPct) + '%'; cropRight.style.height = hPct + '%';
    }

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
      renderCrop();
    } else {
      img.addEventListener('load', function() { renderAnnotations(); renderCrop(); });
    }

    imgWrap.appendChild(imgFrame);

    stepEditors[step.index] = {
      // setStepData is used after undo/redo/reset, which can change the
      // image bytes, annotation list, and crop atomically — it replaces
      // this step's local data and re-renders both overlays, and the
      // caller separately calls bustImageCache() since the image bytes
      // may also have changed.
      setStepData: function(newStep) {
        step.annotations = newStep.annotations || [];
        step.crop = newStep.crop || null;
        renderAnnotations();
        renderCrop();
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

    clearCropBtn.disabled = !step.crop;
    clearCropBtn.addEventListener('click', function() {
      apiJSON('POST', '/api/crop/clear', {index: step.index}).then(function(newStep) {
        stepEditors[step.index].setStepData(newStep);
      }).catch(function(e) { alert('Clear crop failed: ' + e.message); });
    });

    setupDrawing({
      img: img, imgFrame: imgFrame, overlay: overlay, handles: drawHandles,
      redactBtn: redactBtn, highlightBtn: highlightBtn, cropBtn: cropBtn, freehandBtn: freehandBtn,
      freehandPreviewSvg: freehandPreviewSvg, freehandPreviewPoly: freehandPreviewPoly,
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
    var isFreehand = !!(ann.points && ann.points.length);
    var el = document.createElement('div');
    el.className = 'annotation' + (isFreehand ? ' freehand' : '');

    // color() always resolves to a real, renderable value (never '' or
    // undefined) so every place a color is read — inline styles here,
    // the color-picker input's value, the API payload other commits send
    // — agrees on what "this annotation's color" means. '' on disk means
    // "use the default", not "no color set", so the fallback belongs
    // here, once, rather than sprinkled at every call site.
    function color() { return ann.color || DEFAULT_ANNOTATION_COLOR; }

    function place() {
      var topPct = ann.y / img.naturalHeight;
      el.style.left = (ann.x / img.naturalWidth * 100) + '%';
      el.style.top = (topPct * 100) + '%';
      el.style.width = (ann.w / img.naturalWidth * 100) + '%';
      el.style.height = (ann.h / img.naturalHeight * 100) + '%';
      if (!isFreehand) el.style.borderColor = color();
      // A toolbar floating above an annotation near the top of the image
      // would be clipped off the frame — flip it below instead.
      el.classList.toggle('toolbar-below', topPct < 0.08);
    }
    place();

    // A freehand stroke draws as an SVG polyline inside the same
    // percentage-positioned box every annotation already uses for its
    // bounding rectangle — the box supplies position/size (untouched,
    // same code as a rectangle annotation), the SVG's viewBox just
    // remaps the path's natural-pixel coordinates onto whatever CSS
    // pixel size that box actually renders at, including on resize.
    // renderStroke is a no-op for a rectangle annotation (never
    // reassigned below) so commitColor can call it unconditionally
    // rather than needing to know which kind of annotation this is.
    var renderStroke = function() {};
    if (isFreehand) {
      var svgNS = 'http://www.w3.org/2000/svg';
      var svg = document.createElementNS(svgNS, 'svg');
      svg.setAttribute('viewBox', '0 0 ' + ann.w + ' ' + ann.h);
      svg.setAttribute('preserveAspectRatio', 'none');
      svg.style.position = 'absolute';
      svg.style.left = '0'; svg.style.top = '0';
      svg.style.width = '100%'; svg.style.height = '100%';
      svg.style.overflow = 'visible';
      var poly = document.createElementNS(svgNS, 'polyline');
      poly.setAttribute('fill', 'none');
      poly.setAttribute('stroke-width', '4');
      poly.setAttribute('stroke-linecap', 'round');
      poly.setAttribute('stroke-linejoin', 'round');
      el.appendChild(svg);
      svg.appendChild(poly);
      renderStroke = function() {
        svg.setAttribute('viewBox', '0 0 ' + ann.w + ' ' + ann.h);
        poly.setAttribute('stroke', color());
        poly.setAttribute('points', ann.points.map(function(p) { return (p.x - ann.x) + ',' + (p.y - ann.y); }).join(' '));
      };
      renderStroke();
    }

    var badge = document.createElement('span');
    badge.className = 'annotation-badge';
    badge.textContent = ann.number;
    badge.style.background = color();
    el.appendChild(badge);

    var toolbar = document.createElement('div');
    toolbar.className = 'annotation-toolbar';
    toolbar.addEventListener('mousedown', function(e) { e.stopPropagation(); });

    var numberInput = document.createElement('input');
    numberInput.type = 'number';
    numberInput.value = ann.number;
    numberInput.title = 'Renumber';
    toolbar.appendChild(numberInput);

    var colorInput = document.createElement('input');
    colorInput.type = 'color';
    colorInput.className = 'annotation-color-input';
    colorInput.value = color();
    colorInput.title = 'Highlight color';
    toolbar.appendChild(colorInput);

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
      api('POST', '/api/annotation/update', {index: step.index, id: ann.id, x: ann.x, y: ann.y, w: ann.w, h: ann.h, number: number, color: ann.color})
        .catch(function(e) { alert('Renumber failed: ' + e.message); ann.number = prev; badge.textContent = prev; numberInput.value = prev; });
    }
    numberInput.addEventListener('change', commitNumber);

    function commitColor() {
      var prev = ann.color;
      ann.color = colorInput.value;
      badge.style.background = color();
      el.style.borderColor = color();
      renderStroke();
      api('POST', '/api/annotation/update', {index: step.index, id: ann.id, x: ann.x, y: ann.y, w: ann.w, h: ann.h, number: ann.number, color: ann.color})
        .catch(function(e) {
          alert('Color change failed: ' + e.message);
          ann.color = prev;
          badge.style.background = color();
          el.style.borderColor = color();
          renderStroke();
          colorInput.value = color();
        });
    }
    colorInput.addEventListener('change', commitColor);

    // Freehand strokes are move-only in this first pass — no corner
    // handles, so no per-vertex or bounding-box resize. Scaling a
    // hand-drawn path rarely preserves its character anyway (unlike a
    // rectangle, where "resize" is exactly as meaningful as "draw it
    // right the first time"), so redrawing is the expected way to fix
    // a freehand annotation's shape; moving it is still worth keeping.
    var handles = isFreehand ? [] : ['nw', 'ne', 'sw', 'se'].map(function(corner) {
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
      var payload = {index: step.index, id: ann.id, x: x, y: y, w: w, h: h, number: ann.number, color: ann.color};
      if (isFreehand) {
        // Translate every vertex by the same delta the bounding box
        // moved — this is the only way a freehand annotation's position
        // update can happen, since there's no independent "box" to move
        // the path along with; the path IS the annotation.
        var dx = x - ann.x, dy = y - ann.y;
        ann.points = ann.points.map(function(p) { return {x: p.x + dx, y: p.y + dy}; });
        payload.points = ann.points;
      }
      ann.x = x; ann.y = y; ann.w = w; ann.h = h;
      renderStroke();
      api('POST', '/api/annotation/update', payload)
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
    var pending = null; // {mode, x, y, w, h} in on-screen px, relative to imgFrame — redact/highlight/crop
    var pendingFreehand = null; // array of {x,y} in the image's natural pixel space, while drawing/adjusting a stroke
    var dragging = null;

    function setArmed(mode) {
      armedMode = mode;
      el.img.style.cursor = mode ? 'crosshair' : 'default';
      el.modeHint.textContent = mode ? ('Drag on the image to draw a ' + mode + '.') : '';
      el.redactBtn.classList.toggle('armed', mode === 'redact');
      el.highlightBtn.classList.toggle('armed', mode === 'highlight');
      el.cropBtn.classList.toggle('armed', mode === 'crop');
      el.freehandBtn.classList.toggle('armed', mode === 'freehand');
    }

    function updateButtonStates() {
      var busy = !!pending || !!pendingFreehand;
      el.redactBtn.disabled = busy;
      el.highlightBtn.disabled = busy;
      el.cropBtn.disabled = busy;
      el.freehandBtn.disabled = busy;
    }

    function framePos(e) {
      var rect = el.imgFrame.getBoundingClientRect();
      return {x: e.clientX - rect.left, y: e.clientY - rect.top};
    }

    // framePosNatural is framePos already converted into the image's own
    // pixel space — freehand points are captured directly in that space
    // (unlike the redact/highlight/crop rectangle, which stays in
    // on-screen px until Apply) so the live preview's SVG viewBox can
    // use them with zero further conversion.
    function framePosNatural(e) {
      var rect = el.imgFrame.getBoundingClientRect();
      var sx = el.img.naturalWidth / rect.width;
      var sy = el.img.naturalHeight / rect.height;
      return {x: (e.clientX - rect.left) * sx, y: (e.clientY - rect.top) * sy};
    }

    function renderFreehandPreview() {
      if (!pendingFreehand || pendingFreehand.length === 0) {
        el.freehandPreviewSvg.style.display = 'none';
        return;
      }
      el.freehandPreviewSvg.setAttribute('viewBox', '0 0 ' + el.img.naturalWidth + ' ' + el.img.naturalHeight);
      el.freehandPreviewSvg.style.display = 'block';
      el.freehandPreviewPoly.setAttribute('points', pendingFreehand.map(function(p) { return p.x + ',' + p.y; }).join(' '));
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
    el.cropBtn.addEventListener('click', function() {
      if (pending) return;
      setArmed(armedMode === 'crop' ? null : 'crop');
    });
    el.freehandBtn.addEventListener('click', function() {
      if (pending || pendingFreehand) return;
      setArmed(armedMode === 'freehand' ? null : 'freehand');
    });

    el.img.addEventListener('mousedown', function(e) {
      if (!armedMode || pending || pendingFreehand) return;
      e.preventDefault();
      if (armedMode === 'freehand') {
        // A stroke's extent isn't known ahead of time the way a
        // rectangle's is, so freehand skips the draw-then-adjust split
        // entirely: the whole mousedown-to-mouseup gesture *is* the
        // path, captured directly, with Apply/Cancel appearing only
        // once the gesture ends.
        dragging = {type: 'freehand'};
        pendingFreehand = [framePosNatural(e)];
        renderFreehandPreview();
        return;
      }
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
      if (!dragging) return;
      if (dragging.type === 'freehand') {
        var np = framePosNatural(e);
        var last = pendingFreehand[pendingFreehand.length - 1];
        // Only record a point once the pointer has actually moved a few
        // natural pixels — keeps the path array a reasonable size
        // without any visible loss of smoothness (a mousemove stream at
        // typical polling rates would otherwise record a point every
        // 1-2px, most of it redundant).
        if (Math.hypot(np.x - last.x, np.y - last.y) >= 3) {
          pendingFreehand.push(np);
          renderFreehandPreview();
        }
        return;
      }
      if (!pending) return;
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
      if (dragging.type === 'freehand') {
        dragging = null;
        if (pendingFreehand && pendingFreehand.length >= 2) {
          showApplyBar(true);
          updateButtonStates();
        } else {
          // A stray click while armed, same "too small to be
          // intentional" treatment the rectangle modes give a tiny drag.
          pendingFreehand = null;
          renderFreehandPreview();
          setArmed(null);
        }
        return;
      }
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
      if (pendingFreehand) {
        var points = pendingFreehand;
        apiJSON('POST', '/api/annotation', {index: el.stepIndex, points: points}).then(function(data) {
          pendingFreehand = null;
          renderFreehandPreview();
          setArmed(null);
          showApplyBar(false);
          updateButtonStates();
          stepEditors[el.stepIndex].addAnnotationLocal(data);
        }).catch(function(e) { alert('Freehand highlight failed: ' + e.message); });
        return;
      }
      if (!pending) return;
      var mode = pending.mode;
      var scaleX = el.img.naturalWidth / el.img.clientWidth;
      var scaleY = el.img.naturalHeight / el.img.clientHeight;
      var rect = {
        x: pending.x * scaleX, y: pending.y * scaleY,
        w: pending.w * scaleX, h: pending.h * scaleY,
      };
      var request;
      if (mode === 'redact') {
        request = apiJSON('POST', '/api/edit-image', Object.assign({index: el.stepIndex, style: el.getRedactStyle()}, rect));
      } else if (mode === 'crop') {
        request = apiJSON('POST', '/api/crop', Object.assign({index: el.stepIndex}, rect));
      } else {
        request = apiJSON('POST', '/api/annotation', Object.assign({index: el.stepIndex}, rect));
      }
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
        } else if (mode === 'crop') {
          editor.setStepData(data);
        } else {
          editor.addAnnotationLocal(data);
        }
      }).catch(function(e) { alert('Edit failed: ' + e.message); });
    });

    el.cancelBtn.addEventListener('click', function() {
      pending = null;
      pendingFreehand = null;
      renderFreehandPreview();
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
