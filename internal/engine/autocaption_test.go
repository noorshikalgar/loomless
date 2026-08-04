package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/noorshikalgar/story/internal/capture"
	"github.com/noorshikalgar/story/internal/session"
)

func TestBuildAutoCaption(t *testing.T) {
	type tc struct {
		name             string
		elem             capture.AXElement
		elemOK           bool
		windowApp        string
		focusJustChanged bool
		want             string
	}
	table := []tc{
		{
			name: "labeled element with app", elem: capture.AXElement{Role: "button", Label: "Export"},
			elemOK: true, windowApp: "Finder",
			want: `Clicked the "Export" button in Finder.`,
		},
		{
			name: "labeled element, no app", elem: capture.AXElement{Role: "text field", Label: "Search"},
			elemOK: true, windowApp: "",
			want: `Clicked the "Search" text field.`,
		},
		{
			name: "role only, no label, with app", elem: capture.AXElement{Role: "checkbox"},
			elemOK: true, windowApp: "Settings",
			want: "Clicked a checkbox in Settings.",
		},
		{
			name: "role only, no label, no app", elem: capture.AXElement{Role: "checkbox"},
			elemOK: true, windowApp: "",
			want: "Clicked a checkbox.",
		},
		{
			name: "element found but empty role and label, with app", elem: capture.AXElement{},
			elemOK: true, windowApp: "Terminal",
			want: "Clicked in Terminal.",
		},
		{
			name: "element found but empty role and label, no app", elem: capture.AXElement{},
			elemOK: true, windowApp: "",
			want: "",
		},
		{
			name: "grounding unavailable, with app", elem: capture.AXElement{},
			elemOK: false, windowApp: "Terminal",
			want: "Clicked in Terminal.",
		},
		{
			name: "grounding unavailable, no app", elem: capture.AXElement{},
			elemOK: false, windowApp: "",
			want: "",
		},
		{
			name: "focus just changed overrides everything else", elem: capture.AXElement{Role: "button", Label: "Export"},
			elemOK: true, windowApp: "Finder", focusJustChanged: true,
			want: "Switched to Finder.",
		},
		{
			name: "focus just changed, no app", elem: capture.AXElement{Role: "button", Label: "Export"},
			elemOK: true, windowApp: "", focusJustChanged: true,
			want: "",
		},
	}
	for _, c := range table {
		t.Run(c.name, func(t *testing.T) {
			got := buildAutoCaption(c.elem, c.elemOK, c.windowApp, c.focusJustChanged)
			if got != c.want {
				t.Errorf("buildAutoCaption(%+v, %v, %q, %v) = %q, want %q", c.elem, c.elemOK, c.windowApp, c.focusJustChanged, got, c.want)
			}
		})
	}
}

// TestRephraseCaptionSendsExpectedRequestAndParsesResponse confirms
// rephraseCaption builds a well-formed Anthropic Messages API request
// (auth header, API version, the input caption embedded in the prompt)
// and correctly extracts the reply text — using a local httptest.Server
// swapped in via anthropicMessagesURL rather than the real API, so this
// costs nothing and needs no real key.
func TestRephraseCaptionSendsExpectedRequestAndParsesResponse(t *testing.T) {
	var gotAPIKey, gotVersion string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]string{{"type": "text", "text": "  Clicked the Export button in Finder.  "}},
		})
	}))
	defer srv.Close()

	orig := anthropicMessagesURL
	anthropicMessagesURL = srv.URL
	defer func() { anthropicMessagesURL = orig }()

	got, err := rephraseCaption("test-key-123", `Clicked the "Export" button in Finder.`)
	if err != nil {
		t.Fatalf("rephraseCaption: %v", err)
	}
	if got != "Clicked the Export button in Finder." {
		t.Errorf("rephraseCaption result = %q, want trimmed response text", got)
	}
	if gotAPIKey != "test-key-123" {
		t.Errorf("x-api-key header = %q, want %q", gotAPIKey, "test-key-123")
	}
	if gotVersion == "" {
		t.Error("anthropic-version header missing")
	}
	messages, _ := gotBody["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("request body messages = %v, want exactly one message", messages)
	}
}

// TestRephraseCaptionNonOKStatusReturnsError confirms an API error
// response is surfaced as a Go error rather than silently returning an
// empty or garbage string — polishCaptionAsync relies on this to know
// when to leave the local caption alone.
func TestRephraseCaptionNonOKStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	orig := anthropicMessagesURL
	anthropicMessagesURL = srv.URL
	defer func() { anthropicMessagesURL = orig }()

	if _, err := rephraseCaption("bad-key", "some caption"); err == nil {
		t.Error("rephraseCaption with a 401 response succeeded, want an error")
	}
}

// TestPolishCaptionAsyncNoOpWithoutAPIKey confirms the optional-LLM path
// (decision #33) does nothing at all — no network call, no session
// mutation — when ANTHROPIC_API_KEY isn't set, which must be true by
// default for every user who never configured one.
func TestPolishCaptionAsyncNoOpWithoutAPIKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "")

	if _, _, err := session.Create("nopolish"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	step, err := session.AppendStep("nopolish", "Clicked in Terminal.", "Terminal", "", nil)
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}

	logs := []string{}
	polishCaptionAsync("nopolish", step.Index, step.Note, func(format string, a ...any) {
		logs = append(logs, format)
	})
	// polishCaptionAsync returns synchronously before doing anything when
	// there's no key — no goroutine even starts, so there's nothing to
	// wait for here.
	if len(logs) != 0 {
		t.Errorf("polishCaptionAsync with no API key logged %v, want no activity at all", logs)
	}
	reloaded, _, err := session.Load("nopolish")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.Steps[0].Note != "Clicked in Terminal." {
		t.Errorf("note = %q after polishCaptionAsync with no API key, want unchanged", reloaded.Steps[0].Note)
	}
}

// TestPolishCaptionAsyncUpdatesNoteWhenConfigured confirms the full
// opt-in path end to end: with an API key configured and a mocked
// Anthropic endpoint, the async rewrite eventually lands as the step's
// new Note via session.EditNote.
func TestPolishCaptionAsyncUpdatesNoteWhenConfigured(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]string{{"type": "text", "text": "Clicked the Export button in Finder."}},
		})
	}))
	defer srv.Close()
	orig := anthropicMessagesURL
	anthropicMessagesURL = srv.URL
	defer func() { anthropicMessagesURL = orig }()

	if _, _, err := session.Create("polish"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	step, err := session.AppendStep("polish", `Clicked the "Export" button in Finder.`, "Finder", "", nil)
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}

	polishCaptionAsync("polish", step.Index, step.Note, func(format string, a ...any) {})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		reloaded, _, err := session.Load("polish")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if reloaded.Steps[0].Note == "Clicked the Export button in Finder." {
			return // success
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("polishCaptionAsync did not update the note within the timeout")
}
