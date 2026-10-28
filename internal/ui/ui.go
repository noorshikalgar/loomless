// Package ui is Loomless's design system: color/type/spacing tokens,
// shared components (buttons, inputs, cards, badges, toasts, dialogs),
// self-hosted fonts, the brand icon, and theme persistence — embedded in
// the binary and served under /ui/ by every Loomless web server (the
// dashboard and each per-session editor), so every page looks like the
// same product and a theme picked on one page carries to the next.
//
// Fonts are self-hosted rather than loaded from a CDN on purpose: this
// is a local-first tool that must work offline and shouldn't ping a
// third party every time a page opens.
package ui

import (
	"embed"
	"encoding/json"
	"html"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/noorshikalgar/loomless/internal/session"
)

//go:embed static
var staticFS embed.FS

// Theme is one selectable theme. ID matches an html[data-theme="…"]
// block in static/tokens.css ("loomless" is the :root default).
type Theme struct {
	ID    string
	Label string
}

// SystemTheme is a preference, not a theme: it follows the OS light/dark
// setting, resolving to "loomless" or "loomless-light" in the browser.
const SystemTheme = "system"

// Themes lists every theme the picker offers, in display order.
var Themes = []Theme{
	{SystemTheme, "System (auto)"},
	{"loomless", "Loomless Dark"},
	{"loomless-light", "Loomless Light"},
	{"tokyo-night", "Tokyo Night"},
	{"catppuccin", "Catppuccin Mocha"},
	{"gruvbox", "Gruvbox"},
	{"nord", "Nord"},
	{"github-dark", "GitHub Dark"},
}

func validTheme(id string) bool {
	for _, t := range Themes {
		if t.ID == id {
			return true
		}
	}
	return false
}

// settings is ~/.loomless/settings.json. Theme lives here, server-side,
// instead of in the browser's localStorage: every Loomless server binds
// a random port, so each launch is a new browser origin with empty
// storage — a localStorage theme would be forgotten every single time,
// and the dashboard and an editor (different ports) could never agree.
type settings struct {
	Theme string `json:"theme,omitempty"`
}

var settingsMu sync.Mutex

func settingsPath() (string, error) {
	root, err := session.Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "settings.json"), nil
}

func loadSettings() settings {
	path, err := settingsPath()
	if err != nil {
		return settings{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return settings{}
	}
	var s settings
	if json.Unmarshal(data, &s) != nil {
		return settings{}
	}
	return s
}

// saveSettings writes via temp file + rename so a crash mid-write can
// never leave a truncated settings.json behind.
func saveSettings(s settings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// CurrentTheme returns the saved theme preference, defaulting to
// following the OS.
func CurrentTheme() string {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	if t := loadSettings().Theme; validTheme(t) {
		return t
	}
	return SystemTheme
}

// SetTheme validates and persists a theme preference.
func SetTheme(id string) error {
	if !validTheme(id) {
		return &invalidThemeError{id}
	}
	settingsMu.Lock()
	defer settingsMu.Unlock()
	s := loadSettings()
	s.Theme = id
	return saveSettings(s)
}

type invalidThemeError struct{ id string }

func (e *invalidThemeError) Error() string { return "unknown theme " + `"` + e.id + `"` }

// HTMLAttrs returns the attributes for a page's <html> tag so the right
// theme is applied by the server before first paint — no flash of the
// default theme, no script needed (except to resolve "system", which
// only the browser can, done by the inline script in Head).
func HTMLAttrs() string {
	pref := CurrentTheme()
	resolved := pref
	if pref == SystemTheme {
		resolved = "loomless"
	}
	return ` data-theme-pref="` + html.EscapeString(pref) + `" data-theme="` + html.EscapeString(resolved) + `"`
}

// Head returns the shared <head> contents every Loomless page needs:
// font preloads, the token + component stylesheets, the favicon, the
// pre-paint "system" theme resolver, and the deferred UI runtime.
func Head() string {
	return `<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="icon" type="image/svg+xml" href="/ui/icon.svg">
<link rel="preload" href="/ui/fonts/Geist-Variable.woff2" as="font" type="font/woff2" crossorigin>
<link rel="preload" href="/ui/fonts/GeistMono-Variable.woff2" as="font" type="font/woff2" crossorigin>
<script>(function(){var r=document.documentElement;if(r.dataset.themePref==='system'&&window.matchMedia&&!window.matchMedia('(prefers-color-scheme: dark)').matches){r.dataset.theme='loomless-light';}})();</script>
<link rel="stylesheet" href="/ui/tokens.css">
<link rel="stylesheet" href="/ui/components.css">
<script src="/ui/ui.js" defer></script>`
}

// ThemePicker renders the icon-only theme switcher (a native <select>,
// for free keyboard and screen-reader support, with a palette icon drawn
// over it). static/ui.js wires every select[data-theme-select].
func ThemePicker() string {
	current := CurrentTheme()
	var b strings.Builder
	b.WriteString(`<div class="theme-picker"><select data-theme-select title="Theme" aria-label="Theme">`)
	for _, t := range Themes {
		b.WriteString(`<option value="` + html.EscapeString(t.ID) + `"`)
		if t.ID == current {
			b.WriteString(` selected`)
		}
		b.WriteString(`>` + html.EscapeString(t.Label) + `</option>`)
	}
	b.WriteString(`</select><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3a9 9 0 100 18c1.1 0 2-.9 2-2 0-.5-.2-1-.5-1.4-.3-.4-.5-.8-.5-1.3 0-.9.7-1.6 1.6-1.6H16a5 5 0 005-5c0-3.9-4-6.7-9-6.7z" fill="none" stroke="currentColor" stroke-width="1.6"/><circle cx="7.5" cy="10.5" r="1.1" fill="currentColor"/><circle cx="10.5" cy="7" r="1.1" fill="currentColor"/><circle cx="15" cy="7.5" r="1.1" fill="currentColor"/></svg></div>`)
	return b.String()
}

// Handler serves the design system's static assets under /ui/ and the
// theme preference API at /ui/api/theme. Mount it with
// mux.Handle("/ui/", ui.Handler()).
func Handler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err) // embedded at build time; can only fail if the directive is broken
	}
	files := http.StripPrefix("/ui/", http.FileServer(http.FS(sub)))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ui/api/theme", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Theme string `json:"theme"`
		}{CurrentTheme()})
	})
	mux.HandleFunc("POST /ui/api/theme", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Theme string `json:"theme"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := SetTheme(body.Theme); err != nil {
			if _, ok := err.(*invalidThemeError); ok {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /ui/", func(w http.ResponseWriter, r *http.Request) {
		// No directory listings — only real files are part of the API.
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/ui/fonts/") {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
	return mux
}
