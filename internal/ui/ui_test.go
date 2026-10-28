package ui

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// The brand icon is embedded from internal/ui/static (go:embed can't
// reach ../../assets), so it's a copy of assets/icon.svg — this fails
// the moment the two drift apart instead of shipping a stale icon.
func TestIconMatchesAssetsMaster(t *testing.T) {
	master, err := os.ReadFile("../../assets/icon.svg")
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := staticFS.ReadFile("static/icon.svg")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(master, embedded) {
		t.Fatal("internal/ui/static/icon.svg differs from assets/icon.svg — copy the master over")
	}
}

func TestHandlerServesStaticAssets(t *testing.T) {
	h := Handler()
	cases := []struct {
		path        string
		contentType string
		cache       string
	}{
		{"/ui/tokens.css", "text/css", "no-cache"},
		{"/ui/components.css", "text/css", "no-cache"},
		{"/ui/ui.js", "javascript", "no-cache"},
		{"/ui/icon.svg", "image/svg+xml", "no-cache"},
		{"/ui/fonts/Geist-Variable.woff2", "font/woff2", "max-age=86400"},
		{"/ui/fonts/GeistMono-Variable.woff2", "font/woff2", "max-age=86400"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d, want 200", c.path, rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, c.contentType) {
			t.Errorf("%s: Content-Type %q, want it to contain %q", c.path, ct, c.contentType)
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, c.cache) {
			t.Errorf("%s: Cache-Control %q, want it to contain %q", c.path, cc, c.cache)
		}
	}
}

func TestHandlerRefusesDirectoryListings(t *testing.T) {
	h := Handler()
	for _, path := range []string{"/ui/", "/ui/fonts/"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, rec.Code)
		}
	}
}

func TestThemeDefaultsToSystem(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := CurrentTheme(); got != SystemTheme {
		t.Fatalf("CurrentTheme() = %q with no settings file, want %q", got, SystemTheme)
	}
	if attrs := HTMLAttrs(); !strings.Contains(attrs, `data-theme-pref="system"`) || !strings.Contains(attrs, `data-theme="loomless"`) {
		t.Fatalf("HTMLAttrs() = %q, want system pref resolving to loomless server-side", attrs)
	}
}

func TestThemeRoundTripsThroughAPI(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	h := Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/ui/api/theme", strings.NewReader(`{"theme":"tokyo-night"}`)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST status %d (%s), want 204", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ui/api/theme", nil))
	if !strings.Contains(rec.Body.String(), `"tokyo-night"`) {
		t.Fatalf("GET body %q, want tokyo-night", rec.Body.String())
	}
	if attrs := HTMLAttrs(); !strings.Contains(attrs, `data-theme="tokyo-night"`) {
		t.Fatalf("HTMLAttrs() = %q after saving tokyo-night", attrs)
	}
	if picker := ThemePicker(); !strings.Contains(picker, `value="tokyo-night" selected`) {
		t.Fatal("ThemePicker() doesn't mark the saved theme selected")
	}
}

func TestThemeAPIRejectsUnknownTheme(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/ui/api/theme", strings.NewReader(`{"theme":"not-a-theme"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
	if got := CurrentTheme(); got != SystemTheme {
		t.Fatalf("rejected theme still changed CurrentTheme() to %q", got)
	}
}

// Every theme the picker offers must actually exist in tokens.css —
// otherwise picking it silently falls back to the default palette.
func TestEveryThemeHasTokens(t *testing.T) {
	css, err := staticFS.ReadFile("static/tokens.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, th := range Themes {
		if th.ID == SystemTheme || th.ID == "loomless" {
			continue // system resolves in JS; loomless is the :root default
		}
		if !strings.Contains(string(css), `html[data-theme="`+th.ID+`"]`) {
			t.Errorf("theme %q has no html[data-theme] block in tokens.css", th.ID)
		}
	}
}
