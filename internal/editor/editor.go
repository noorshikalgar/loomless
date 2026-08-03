// Package editor is Story's local web editor (Phase 2's "local editor,"
// per docs/product/03-features.md and open question #12 in
// docs/product/10-open-questions.md) — a localhost-only HTTP server
// serving a single-page app that lets a human reorder steps, hide ones
// they don't want in the exported story, and fix up captions, before
// exporting. Chosen over a TUI per 08-technical-considerations.md's own
// reasoning ("a browser-based local UI handles [visual editing] far more
// easily than a TUI") — this Phase 2 slice doesn't yet do the visual
// screenshot redaction/crop editing that reasoning was really about (see
// the follow-up task), but the same reasoning already applies to viewing
// screenshots inline while reordering/captioning, and keeps this on the
// same architecture the redaction follow-up will need anyway.
//
// Every mutation goes through internal/session's edit functions
// (ReorderSteps/SetStepHidden/EditNote), which reload from disk, mutate,
// log to Session.EditLog, and save — this package holds no session state
// itself beyond the name and directory, so it can't drift from what's
// actually on disk between requests.
package editor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/noorshikalgar/story/internal/session"
)

// imageFilePattern matches the only image filenames this project ever
// produces (session.AddStep's "step-%04d.png") — used to reject the
// /images/ route's filename parameter outright if it doesn't match,
// rather than trying to sanitize an arbitrary path (no "..", no "/"
// walking involved at all, since nothing but this shape is ever valid).
var imageFilePattern = regexp.MustCompile(`^step-\d{4}\.png$`)

// NewHandler builds the editor's HTTP handler for a named session. Fails
// fast if the session doesn't exist rather than serving a broken editor.
func NewHandler(name string) (http.Handler, error) {
	dir, err := session.Dir(name)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("session %q not found: %w", name, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, indexHTML(name))
	})
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		s, _, err := session.Load(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s)
	})
	mux.HandleFunc("GET /images/{file}", func(w http.ResponseWriter, r *http.Request) {
		file := r.PathValue("file")
		if !imageFilePattern.MatchString(file) {
			http.Error(w, "invalid image filename", http.StatusBadRequest)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "images", file))
	})
	mux.HandleFunc("POST /api/reorder", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Order []int `json:"order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := session.ReorderSteps(name, body.Order); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/hide", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Index  int  `json:"index"`
			Hidden bool `json:"hidden"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := session.SetStepHidden(name, body.Index, body.Hidden); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/caption", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Index int    `json:"index"`
			Note  string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := session.EditNote(name, body.Index, body.Note); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/edit-image", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Index  int     `json:"index"`
			Action string  `json:"action"` // "redact" | "highlight"
			X      float64 `json:"x"`
			Y      float64 `json:"y"`
			W      float64 `json:"w"`
			H      float64 `json:"h"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := session.EditImage(name, body.Index, body.Action, body.X, body.Y, body.W, body.H); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	return mux, nil
}
