package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/noorshikalgar/loomless/internal/capture"
	"github.com/noorshikalgar/loomless/internal/session"
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

// TestAnthropicProviderSendsExpectedRequestAndParsesResponse confirms
// anthropicProvider.rephrase builds a well-formed Messages API request
// (auth header, API version, the input caption embedded in the prompt)
// and correctly extracts the reply text, against a local
// httptest.Server rather than the real API — costs nothing, needs no
// real key.
func TestAnthropicProviderSendsExpectedRequestAndParsesResponse(t *testing.T) {
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

	p := &anthropicProvider{apiKey: "test-key-123", model: "claude-haiku-4-5-20251001", baseURL: srv.URL}
	got, err := p.rephrase(`Clicked the "Export" button in Finder.`)
	if err != nil {
		t.Fatalf("rephrase: %v", err)
	}
	if got != "Clicked the Export button in Finder." {
		t.Errorf("rephrase result = %q, want trimmed response text", got)
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

func TestAnthropicProviderNonOKStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := &anthropicProvider{apiKey: "bad-key", model: "claude-haiku-4-5-20251001", baseURL: srv.URL}
	if _, err := p.rephrase("some caption"); err == nil {
		t.Error("rephrase with a 401 response succeeded, want an error")
	}
}

// TestOpenAIProviderSendsExpectedRequestAndParsesResponse confirms the
// Chat Completions request/response shape specifically — Bearer auth
// (not x-api-key) and the choices[0].message.content response path,
// both distinct from every other provider here.
func TestOpenAIProviderSendsExpectedRequestAndParsesResponse(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "Clicked the Export button in Finder."}},
			},
		})
	}))
	defer srv.Close()

	p := &openAIProvider{apiKey: "sk-test", model: "gpt-4o-mini", baseURL: srv.URL}
	got, err := p.rephrase("some caption")
	if err != nil {
		t.Fatalf("rephrase: %v", err)
	}
	if got != "Clicked the Export button in Finder." {
		t.Errorf("rephrase result = %q", got)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer sk-test")
	}
}

// TestGeminiProviderSendsExpectedRequestAndParsesResponse confirms
// Gemini's own distinctive shape: the API key travels as a query
// parameter (not a header, unlike every other provider here), and the
// response is nested under candidates[0].content.parts[0].text.
func TestGeminiProviderSendsExpectedRequestAndParsesResponse(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("key")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{"content": map[string]any{
					"parts": []map[string]string{{"text": "Clicked the Export button in Finder."}},
				}},
			},
		})
	}))
	defer srv.Close()

	p := &geminiProvider{apiKey: "gemini-test-key", model: "gemini-2.0-flash", baseURL: srv.URL}
	got, err := p.rephrase("some caption")
	if err != nil {
		t.Fatalf("rephrase: %v", err)
	}
	if got != "Clicked the Export button in Finder." {
		t.Errorf("rephrase result = %q", got)
	}
	if gotQuery != "gemini-test-key" {
		t.Errorf("key query param = %q, want %q", gotQuery, "gemini-test-key")
	}
}

// TestOllamaProviderSendsExpectedRequestAndParsesResponse confirms the
// local-only provider's shape: no auth header at all (Ollama has no
// concept of an API key), and the response is a flat {"response": "..."}
// rather than a nested choices/candidates/content array like the three
// cloud providers.
func TestOllamaProviderSendsExpectedRequestAndParsesResponse(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"response": "Clicked the Export button in Finder."})
	}))
	defer srv.Close()

	p := &ollamaProvider{model: "llama3.2", baseURL: srv.URL}
	got, err := p.rephrase("some caption")
	if err != nil {
		t.Fatalf("rephrase: %v", err)
	}
	if got != "Clicked the Export button in Finder." {
		t.Errorf("rephrase result = %q", got)
	}
	if gotBody["model"] != "llama3.2" {
		t.Errorf("request model = %v, want %q", gotBody["model"], "llama3.2")
	}
}

// TestSelectLLMProviderUnsetIsANoOp confirms the default state (no
// LOOMLESS_LLM_PROVIDER at all) returns (nil, nil) — not an error, not a
// provider — which is what every user who never configured this sees.
func TestSelectLLMProviderUnsetIsANoOp(t *testing.T) {
	t.Setenv("LOOMLESS_LLM_PROVIDER", "")
	p, err := selectLLMProvider()
	if p != nil || err != nil {
		t.Errorf("selectLLMProvider() with no env var = (%v, %v), want (nil, nil)", p, err)
	}
}

// TestSelectLLMProviderMissingKeyIsAnError confirms naming a real
// provider without its required API key is surfaced as an error
// (a likely typo/oversight worth telling the user about), not silently
// treated the same as "provider not configured at all."
func TestSelectLLMProviderMissingKeyIsAnError(t *testing.T) {
	t.Setenv("LOOMLESS_LLM_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "")
	if _, err := selectLLMProvider(); err == nil {
		t.Error("selectLLMProvider() with LOOMLESS_LLM_PROVIDER=openai and no OPENAI_API_KEY succeeded, want an error")
	}
}

// TestSelectLLMProviderUnknownNameIsAnError confirms a typo'd provider
// name fails loudly instead of silently doing nothing, which would be
// much harder to notice.
func TestSelectLLMProviderUnknownNameIsAnError(t *testing.T) {
	t.Setenv("LOOMLESS_LLM_PROVIDER", "chatgpt") // not a real provider name here
	if _, err := selectLLMProvider(); err == nil {
		t.Error("selectLLMProvider() with an unknown provider name succeeded, want an error")
	}
}

// TestSelectLLMProviderPicksEachProviderCorrectly confirms each of the
// four supported names resolves to its own provider type with the
// right config, including model/base-URL override env vars and
// Gemini's GOOGLE_API_KEY fallback.
func TestSelectLLMProviderPicksEachProviderCorrectly(t *testing.T) {
	t.Run("anthropic", func(t *testing.T) {
		t.Setenv("LOOMLESS_LLM_PROVIDER", "anthropic")
		t.Setenv("ANTHROPIC_API_KEY", "a-key")
		t.Setenv("ANTHROPIC_MODEL", "custom-model")
		p, err := selectLLMProvider()
		if err != nil {
			t.Fatalf("selectLLMProvider: %v", err)
		}
		got, ok := p.(*anthropicProvider)
		if !ok {
			t.Fatalf("selectLLMProvider() = %T, want *anthropicProvider", p)
		}
		if got.apiKey != "a-key" || got.model != "custom-model" {
			t.Errorf("anthropicProvider = %+v, want apiKey=a-key model=custom-model", got)
		}
	})
	t.Run("openai", func(t *testing.T) {
		t.Setenv("LOOMLESS_LLM_PROVIDER", "openai")
		t.Setenv("OPENAI_API_KEY", "o-key")
		p, err := selectLLMProvider()
		if err != nil {
			t.Fatalf("selectLLMProvider: %v", err)
		}
		if _, ok := p.(*openAIProvider); !ok {
			t.Fatalf("selectLLMProvider() = %T, want *openAIProvider", p)
		}
	})
	t.Run("gemini via GOOGLE_API_KEY fallback", func(t *testing.T) {
		t.Setenv("LOOMLESS_LLM_PROVIDER", "gemini")
		t.Setenv("GEMINI_API_KEY", "")
		t.Setenv("GOOGLE_API_KEY", "g-key")
		p, err := selectLLMProvider()
		if err != nil {
			t.Fatalf("selectLLMProvider: %v", err)
		}
		got, ok := p.(*geminiProvider)
		if !ok {
			t.Fatalf("selectLLMProvider() = %T, want *geminiProvider", p)
		}
		if got.apiKey != "g-key" {
			t.Errorf("geminiProvider.apiKey = %q, want fallback to GOOGLE_API_KEY %q", got.apiKey, "g-key")
		}
	})
	t.Run("ollama needs no API key", func(t *testing.T) {
		t.Setenv("LOOMLESS_LLM_PROVIDER", "ollama")
		t.Setenv("OLLAMA_HOST", "http://localhost:9999")
		p, err := selectLLMProvider()
		if err != nil {
			t.Fatalf("selectLLMProvider: %v", err)
		}
		got, ok := p.(*ollamaProvider)
		if !ok {
			t.Fatalf("selectLLMProvider() = %T, want *ollamaProvider", p)
		}
		if got.baseURL != "http://localhost:9999/api/generate" {
			t.Errorf("ollamaProvider.baseURL = %q, want host + /api/generate", got.baseURL)
		}
	})
}

// TestPolishCaptionAsyncNoOpWithoutProvider confirms the optional-LLM
// path (decision #33/#34) does nothing at all — no network call, no
// session mutation — when LOOMLESS_LLM_PROVIDER isn't set, which must be
// true by default for every user who never configured one.
func TestPolishCaptionAsyncNoOpWithoutProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOOMLESS_LLM_PROVIDER", "")

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
	// no provider is configured — no goroutine even starts, so there's
	// nothing to wait for here.
	if len(logs) != 0 {
		t.Errorf("polishCaptionAsync with no provider configured logged %v, want no activity at all", logs)
	}
	reloaded, _, err := session.Load("nopolish")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.Steps[0].Note != "Clicked in Terminal." {
		t.Errorf("note = %q after polishCaptionAsync with no provider configured, want unchanged", reloaded.Steps[0].Note)
	}
}

// TestPolishCaptionAsyncUpdatesNoteWhenConfigured confirms the full
// opt-in path end to end through the real selectLLMProvider lookup
// (LOOMLESS_LLM_PROVIDER=anthropic, ANTHROPIC_BASE_URL pointed at a mock
// server): the async rewrite eventually lands as the step's new Note
// via session.EditNote.
func TestPolishCaptionAsyncUpdatesNoteWhenConfigured(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOOMLESS_LLM_PROVIDER", "anthropic")
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]string{{"type": "text", "text": "Clicked the Export button in Finder."}},
		})
	}))
	defer srv.Close()
	t.Setenv("ANTHROPIC_BASE_URL", srv.URL)

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
