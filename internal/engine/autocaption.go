package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/noorshikalgar/story/internal/capture"
	"github.com/noorshikalgar/story/internal/session"
)

// buildAutoCaption composes a short, local, zero-cost caption describing
// a click-captured step from whatever grounding data was actually
// available — never requires network access or an API key (decision
// #33's "local template" default). elemOK false means the accessibility
// query itself found nothing (permission not granted, or no element at
// that point); an empty elem with elemOK true means a real element was
// found but had no readable role/label (common for icon-only or custom-
// drawn controls) — both degrade gracefully rather than fabricating a
// caption from data that isn't actually there.
//
// focusJustChanged gets the same special treatment RunClickDaemon's own
// annotation-suppression logic already gives it (see engine.go's own
// comment on that): a click that raises a different app to the front is
// "switch my attention here," not "interact with this specific control"
// — describing it as a click on whatever happened to be under the
// cursor in the newly-frontmost app would be actively misleading, not
// just imprecise.
func buildAutoCaption(elem capture.AXElement, elemOK bool, windowApp string, focusJustChanged bool) string {
	if focusJustChanged {
		if windowApp == "" {
			return ""
		}
		return fmt.Sprintf("Switched to %s.", windowApp)
	}
	if !elemOK || (elem.Role == "" && elem.Label == "") {
		if windowApp == "" {
			return ""
		}
		return fmt.Sprintf("Clicked in %s.", windowApp)
	}
	role := elem.Role
	if role == "" {
		role = "element"
	}
	switch {
	case elem.Label != "" && windowApp != "":
		return fmt.Sprintf("Clicked the %q %s in %s.", elem.Label, role, windowApp)
	case elem.Label != "":
		return fmt.Sprintf("Clicked the %q %s.", elem.Label, role)
	case windowApp != "":
		return fmt.Sprintf("Clicked a %s in %s.", role, windowApp)
	default:
		return fmt.Sprintf("Clicked a %s.", role)
	}
}

// polishCaptionAsync fires a best-effort, non-blocking rewrite of an
// already-saved local caption through an LLM, entirely optional
// (decision #33, extended by decision #34 to more than one provider):
// only attempted when STORY_LLM_PROVIDER names a configured provider,
// and never on the click-capture hot path itself — buildAutoCaption's
// result is already a complete, correct caption saved via AppendStep
// before this is even called, so a slow network, a missing/misconfigured
// provider, or an API error just leaves that local caption standing;
// nothing about capture responsiveness or correctness depends on this
// succeeding. stepIndex identifies the step to update if/when the
// rewrite lands.
func polishCaptionAsync(sessionName string, stepIndex int, localCaption string, logf func(format string, args ...any)) {
	provider, err := selectLLMProvider()
	if err != nil {
		logf("auto-caption polish: skipped (%v)", err)
		return
	}
	if provider == nil || localCaption == "" {
		return
	}
	go func() {
		polished, err := provider.rephrase(localCaption)
		if err != nil {
			logf("auto-caption polish: skipped (%v)", err)
			return
		}
		if polished == "" || polished == localCaption {
			return
		}
		if _, err := session.EditNote(sessionName, stepIndex, polished); err != nil {
			logf("auto-caption polish: failed to save: %v", err)
		}
	}()
}

// llmProvider rewrites one caption into a more natural sentence. Each
// implementation owns its own request/response shape — the four
// providers below agree on almost nothing at the wire level (auth
// scheme, request body, response body), so there's no shared HTTP
// helper worth extracting beyond the prompt text itself.
type llmProvider interface {
	rephrase(caption string) (string, error)
}

// rewritePromptFor builds the single-turn instruction sent to every
// provider — identical wording everywhere so a caption reads the same
// regardless of which backend happened to be configured.
func rewritePromptFor(caption string) string {
	return "Rewrite this UI-interaction caption as one short, natural sentence for a " +
		"step-by-step how-to guide. Keep it factual — don't add anything the input doesn't " +
		"already say. Reply with only the rewritten sentence, no quotes, no preamble.\n\n" +
		"Input: " + caption
}

// selectLLMProvider reads STORY_LLM_PROVIDER and returns the matching
// provider, fully configured from that provider's own env vars — or
// (nil, nil) if the env var isn't set at all, which is the normal,
// silent "no LLM polish" case every user starts in. Deliberately an
// EXPLICIT env var, not auto-detection by scanning for whichever API
// key happens to be set (decision #34): a user might have
// ANTHROPIC_API_KEY exported for something else entirely (e.g. Claude
// Code itself) with no intention of Story spending it on caption
// rewrites — silently picking that up would be a surprise, not a
// convenience. An error is returned only when STORY_LLM_PROVIDER names
// a real provider but that provider's own required config is missing
// (e.g. "openai" with no OPENAI_API_KEY set) — a likely typo/oversight
// worth logging once, unlike "provider unset," which needs no comment.
// Each provider (except Ollama, which has no API key at all) also
// honors a `*_BASE_URL` override (ANTHROPIC_BASE_URL, OPENAI_BASE_URL,
// GEMINI_BASE_URL) — mainly for pointing at a self-hosted/compatible
// gateway, and incidentally the only seam this file's own tests need to
// exercise these providers against a local httptest.Server instead of
// spending real API credits.
func selectLLMProvider() (llmProvider, error) {
	switch os.Getenv("STORY_LLM_PROVIDER") {
	case "":
		return nil, nil
	case "anthropic":
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("STORY_LLM_PROVIDER=anthropic but ANTHROPIC_API_KEY is not set")
		}
		model := os.Getenv("ANTHROPIC_MODEL")
		if model == "" {
			model = "claude-haiku-4-5-20251001"
		}
		baseURL := os.Getenv("ANTHROPIC_BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.anthropic.com/v1/messages"
		}
		return &anthropicProvider{apiKey: apiKey, model: model, baseURL: baseURL}, nil
	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("STORY_LLM_PROVIDER=openai but OPENAI_API_KEY is not set")
		}
		model := os.Getenv("OPENAI_MODEL")
		if model == "" {
			model = "gpt-4o-mini"
		}
		baseURL := os.Getenv("OPENAI_BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1/chat/completions"
		}
		return &openAIProvider{apiKey: apiKey, model: model, baseURL: baseURL}, nil
	case "gemini":
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("STORY_LLM_PROVIDER=gemini but GEMINI_API_KEY (or GOOGLE_API_KEY) is not set")
		}
		model := os.Getenv("GEMINI_MODEL")
		if model == "" {
			model = "gemini-2.0-flash"
		}
		baseURL := os.Getenv("GEMINI_BASE_URL")
		if baseURL == "" {
			baseURL = "https://generativelanguage.googleapis.com/v1beta/models"
		}
		return &geminiProvider{apiKey: apiKey, model: model, baseURL: baseURL}, nil
	case "ollama":
		host := os.Getenv("OLLAMA_HOST")
		if host == "" {
			host = "http://localhost:11434"
		}
		model := os.Getenv("OLLAMA_MODEL")
		if model == "" {
			model = "llama3.2"
		}
		return &ollamaProvider{model: model, baseURL: strings.TrimSuffix(host, "/") + "/api/generate"}, nil
	default:
		return nil, fmt.Errorf("unknown STORY_LLM_PROVIDER %q (want anthropic, openai, gemini, or ollama)", os.Getenv("STORY_LLM_PROVIDER"))
	}
}

// httpTimeout bounds every provider's call — a short, single-sentence
// rewrite should never legitimately take long, and a hung request here
// must not leak a goroutine or delay stopping a recording indefinitely.
const httpTimeout = 8 * time.Second

// anthropicProvider calls the Messages API — see
// https://docs.anthropic.com/en/api/messages. max_tokens: 60 keeps this
// a bounded, single-sentence rewrite rather than an open-ended
// generation task, so cost and latency stay predictable even on a
// per-click cadence.
type anthropicProvider struct {
	apiKey, model, baseURL string
}

func (p *anthropicProvider) rephrase(caption string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":      p.model,
		"max_tokens": 60,
		"messages":   []map[string]string{{"role": "user", "content": rewritePromptFor(caption)}},
	})
	req, err := http.NewRequest(http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: httpTimeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic api returned status %d", resp.StatusCode)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return strings.TrimSpace(result.Content[0].Text), nil
}

// openAIProvider calls the Chat Completions API — see
// https://platform.openai.com/docs/api-reference/chat.
type openAIProvider struct {
	apiKey, model, baseURL string
}

func (p *openAIProvider) rephrase(caption string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":      p.model,
		"max_tokens": 60,
		"messages":   []map[string]string{{"role": "user", "content": rewritePromptFor(caption)}},
	})
	req, err := http.NewRequest(http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: httpTimeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai api returned status %d", resp.StatusCode)
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

// geminiProvider calls the generateContent API — see
// https://ai.google.dev/api/generate-content. Unlike the other three
// providers, the API key travels as a query parameter, not a header —
// Google's own documented convention for this endpoint, not a
// deviation this package invented.
type geminiProvider struct {
	apiKey, model, baseURL string
}

func (p *geminiProvider) rephrase(caption string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]string{{"text": rewritePromptFor(caption)}}},
		},
		"generationConfig": map[string]any{"maxOutputTokens": 60},
	})
	endpoint := fmt.Sprintf("%s/%s:generateContent?key=%s", p.baseURL, p.model, url.QueryEscape(p.apiKey))
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: httpTimeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini api returned status %d", resp.StatusCode)
	}
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text), nil
}

// ollamaProvider calls a local Ollama server's generate API — see
// https://github.com/ollama/ollama/blob/main/docs/api.md#generate-a-completion.
// The only provider here that needs no API key and sends nothing over
// the network at all by default (localhost only, unless OLLAMA_HOST
// points elsewhere) — the natural fit for this project's local-first
// stance, for anyone who'd rather run the rewrite step through a model
// they already have pulled than call out to a cloud API.
type ollamaProvider struct {
	model, baseURL string
}

func (p *ollamaProvider) rephrase(caption string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":  p.model,
		"prompt": rewritePromptFor(caption),
		"stream": false,
	})
	req, err := http.NewRequest(http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: httpTimeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status %d (is the server running, and has %q been pulled?)", resp.StatusCode, p.model)
	}
	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Response), nil
}
