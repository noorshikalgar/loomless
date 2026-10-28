// Loomless shared UI runtime: theme switching, toasts, and dialogs.
// Exposed as window.Loomless so page scripts can call
// Loomless.toast(...), await Loomless.confirm(...), await
// Loomless.prompt(...) instead of the browser's native alert/confirm/
// prompt — which block the whole page, can't be themed, and look
// nothing like the rest of the app.
(function () {
  'use strict';

  var root = document.documentElement;
  var darkQuery = window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null;

  // "system" is a preference, not a theme: it resolves to one of the
  // two Loomless themes based on the OS setting, live.
  function resolveTheme(pref) {
    if (pref !== 'system') return pref;
    return darkQuery && !darkQuery.matches ? 'loomless-light' : 'loomless';
  }

  function applyTheme(pref) {
    root.dataset.themePref = pref;
    root.dataset.theme = resolveTheme(pref);
  }

  if (darkQuery) {
    var onSchemeChange = function () {
      if (root.dataset.themePref === 'system') applyTheme('system');
    };
    if (darkQuery.addEventListener) darkQuery.addEventListener('change', onSchemeChange);
    else if (darkQuery.addListener) darkQuery.addListener(onSchemeChange);
  }

  // The preference is persisted server-side (~/.loomless/settings.json)
  // rather than localStorage: every Loomless server binds a random port,
  // so every launch is a brand-new browser origin with empty storage.
  function saveTheme(pref) {
    return fetch('/ui/api/theme', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({theme: pref}),
    }).then(function (r) {
      if (!r.ok) throw new Error('could not save theme');
    });
  }

  function wireThemePickers() {
    var pickers = document.querySelectorAll('select[data-theme-select]');
    Array.prototype.forEach.call(pickers, function (sel) {
      sel.value = root.dataset.themePref || 'system';
      sel.addEventListener('change', function () {
        applyTheme(sel.value);
        saveTheme(sel.value).catch(function () {
          toast('Theme applied, but could not be saved for next time.', 'error');
        });
      });
    });
  }

  // ---------- toasts ----------
  var stack = null;
  // toast(message, kind?, action?) — kind is 'success' | 'error' |
  // undefined; action is an optional {label, onClick} rendered as a
  // button inside the toast (e.g. "Show in Finder" after an export).
  function toast(message, kind, action) {
    if (!stack) {
      stack = document.createElement('div');
      stack.className = 'toast-stack';
      stack.setAttribute('role', 'status');
      stack.setAttribute('aria-live', 'polite');
      document.body.appendChild(stack);
    }
    var el = document.createElement('div');
    el.className = 'toast' + (kind ? ' toast-' + kind : '');
    var text = document.createElement('span');
    text.className = 'toast-text';
    text.textContent = message;
    el.appendChild(text);
    if (action && action.label) {
      var btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'btn btn-sm btn-ghost toast-action';
      btn.textContent = action.label;
      btn.addEventListener('click', function () { action.onClick(); dismiss(); });
      el.appendChild(btn);
    }
    stack.appendChild(el);
    var dismissed = false;
    function dismiss() {
      if (dismissed) return;
      dismissed = true;
      el.classList.add('leaving');
      setTimeout(function () { el.remove(); }, 250);
    }
    setTimeout(dismiss, kind === 'error' ? 6000 : action ? 6000 : 3500);
    return el;
  }

  // Close any open .menu (<details>) when clicking outside it or
  // pressing Escape — <details> handles open/close on its own summary,
  // but not dismissal from elsewhere on the page.
  document.addEventListener('click', function (e) {
    Array.prototype.forEach.call(document.querySelectorAll('details.menu[open]'), function (m) {
      if (!m.contains(e.target) || e.target.closest('.menu-item')) m.removeAttribute('open');
    });
  });
  document.addEventListener('keydown', function (e) {
    if (e.key !== 'Escape') return;
    Array.prototype.forEach.call(document.querySelectorAll('details.menu[open]'), function (m) {
      m.removeAttribute('open');
      var s = m.querySelector('summary');
      if (s) s.focus();
    });
  });

  // ---------- dialogs ----------
  // dialog() builds one modal and resolves with the chosen result.
  // Focus moves into the dialog and returns to whatever had it before;
  // Escape and backdrop clicks cancel; Enter confirms.
  function dialog(opts) {
    return new Promise(function (resolve) {
      var previouslyFocused = document.activeElement;
      var backdrop = document.createElement('div');
      backdrop.className = 'dialog-backdrop';
      var box = document.createElement('div');
      box.className = 'dialog';
      box.setAttribute('role', 'dialog');
      box.setAttribute('aria-modal', 'true');

      var title = document.createElement('h3');
      title.textContent = opts.title || '';
      title.id = 'lm-dialog-title-' + Date.now();
      box.setAttribute('aria-labelledby', title.id);
      box.appendChild(title);

      if (opts.body) {
        var body = document.createElement('p');
        body.textContent = opts.body;
        box.appendChild(body);
      }

      var input = null;
      if (opts.input) {
        input = document.createElement('input');
        input.className = 'input';
        input.type = 'text';
        input.value = opts.value || '';
        input.setAttribute('aria-label', opts.title || 'Value');
        box.appendChild(input);
      }

      var actions = document.createElement('div');
      actions.className = 'dialog-actions';
      var cancelBtn = document.createElement('button');
      cancelBtn.type = 'button';
      cancelBtn.className = 'btn btn-ghost';
      cancelBtn.textContent = opts.cancelLabel || 'Cancel';
      var okBtn = document.createElement('button');
      okBtn.type = 'button';
      okBtn.className = 'btn ' + (opts.danger ? 'btn-danger' : 'btn-primary');
      okBtn.textContent = opts.confirmLabel || 'OK';
      actions.appendChild(cancelBtn);
      actions.appendChild(okBtn);
      box.appendChild(actions);
      backdrop.appendChild(box);
      document.body.appendChild(backdrop);

      function close(result) {
        document.removeEventListener('keydown', onKey, true);
        backdrop.remove();
        if (previouslyFocused && previouslyFocused.focus) previouslyFocused.focus();
        resolve(result);
      }
      function confirmResult() {
        if (input) {
          var v = input.value.trim();
          close(v === '' ? null : v);
        } else {
          close(true);
        }
      }
      function cancelResult() { close(input ? null : false); }
      function onKey(e) {
        if (e.key === 'Escape') { e.preventDefault(); cancelResult(); }
        else if (e.key === 'Enter' && document.activeElement !== cancelBtn) { e.preventDefault(); confirmResult(); }
      }

      okBtn.addEventListener('click', confirmResult);
      cancelBtn.addEventListener('click', cancelResult);
      backdrop.addEventListener('mousedown', function (e) { if (e.target === backdrop) cancelResult(); });
      document.addEventListener('keydown', onKey, true);

      if (input) { input.focus(); input.select(); } else { okBtn.focus(); }
    });
  }

  function confirmDialog(opts) {
    if (typeof opts === 'string') opts = {title: opts};
    return dialog(opts);
  }
  function promptDialog(opts) {
    if (typeof opts === 'string') opts = {title: opts};
    opts.input = true;
    return dialog(opts);
  }

  window.Loomless = {
    toast: toast,
    confirm: confirmDialog,
    prompt: promptDialog,
    applyTheme: applyTheme,
  };

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', wireThemePickers);
  } else {
    wireThemePickers();
  }
})();
