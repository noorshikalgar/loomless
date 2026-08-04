package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
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
// already-saved local caption through the Anthropic API, entirely
// optional (decision #33): only attempted when ANTHROPIC_API_KEY is set,
// and never on the click-capture hot path itself — buildAutoCaption's
// result is already a complete, correct caption saved via AppendStep
// before this is even called, so a slow network, a missing key, or an
// API error just leaves that local caption standing; nothing about
// capture responsiveness or correctness depends on this succeeding.
// stepIndex identifies the step to update if/when the rewrite lands.
func polishCaptionAsync(sessionName string, stepIndex int, localCaption string, logf func(format string, args ...any)) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" || localCaption == "" {
		return
	}
	go func() {
		polished, err := rephraseCaption(apiKey, localCaption)
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

// anthropicMessagesURL is a var, not a const, so tests can point it at a
// local httptest.Server instead of the real API — the only seam needed
// to exercise rephraseCaption's request/response handling without
// spending real API credits or requiring a real key.
var anthropicMessagesURL = "https://api.anthropic.com/v1/messages"

// rephraseCaption asks a small, fast model to turn the local template
// caption into one natural sentence — a short, bounded, single-turn call
// (max_tokens: 60), not an open-ended generation task, so cost and
// latency stay predictable even though it's on a per-click cadence.
func rephraseCaption(apiKey, caption string) (string, error) {
	reqBody := map[string]any{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 60,
		"messages": []map[string]string{
			{
				"role": "user",
				"content": "Rewrite this UI-interaction caption as one short, natural sentence for a " +
					"step-by-step how-to guide. Keep it factual — don't add anything the input doesn't " +
					"already say. Reply with only the rewritten sentence, no quotes, no preamble.\n\n" +
					"Input: " + caption,
			},
		},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, anthropicMessagesURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
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
