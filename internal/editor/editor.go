// Package editor is Story's local web editor (Phase 2's "local editor,"
// per docs/product/03-features.md and open question #12 in
// docs/product/10-open-questions.md) — a localhost-only HTTP server
// serving a single-page app that lets a human reorder steps, hide ones
// they don't want in the exported story, fix up captions, redact
// sensitive parts of a screenshot, and add/move/renumber/delete numbered
// highlight annotations. Chosen over a TUI per
// 08-technical-considerations.md's own reasoning ("a browser-based local
// UI handles [visual editing] far more easily than a TUI").
//
// Most mutations go through internal/session's edit functions
// (ReorderSteps/SetStepHidden/EditNote/EditTitle/EditWindowInfo/
// EditImage/AddAnnotation/UpdateAnnotation/DeleteAnnotation/
// ReorderAnnotations), which reload
// from disk, mutate, log to Session.EditLog, and save — this package
// holds no *session* state itself beyond the name and directory, so it
// can't drift from what's actually on disk between requests. It does
// hold one piece of ephemeral, non-session state: the unified undo/redo
// history (history.go), which is intentionally process-local — see
// decision #17. Every mutating endpoint responds with the affected
// session.Step as JSON so the frontend can update its local view of that
// step directly instead of re-fetching and re-rendering the whole page.
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

	history := newEditHistory()

	// findStep loads the session fresh and returns the step at
	// stepIndex — shared by every handler below that needs to read
	// current step state before or after a mutation.
	findStep := func(stepIndex int) (session.Step, error) {
		s, _, err := session.Load(name)
		if err != nil {
			return session.Step{}, err
		}
		for _, step := range s.Steps {
			if step.Index == stepIndex {
				return step, nil
			}
		}
		return session.Step{}, fmt.Errorf("step %d does not exist in this session", stepIndex)
	}

	// stepSnapshot captures a step's full editable surface — current
	// image bytes plus its annotation list — as one editSnapshot, for
	// pushing onto the undo history before a mutation is applied.
	stepSnapshot := func(stepIndex int) (editSnapshot, error) {
		step, err := findStep(stepIndex)
		if err != nil {
			return editSnapshot{}, err
		}
		var imgBytes []byte
		if step.ImageFile != "" {
			data, err := os.ReadFile(filepath.Join(dir, step.ImageFile))
			if err != nil {
				return editSnapshot{}, err
			}
			imgBytes = data
		}
		return editSnapshot{Image: imgBytes, Annotations: cloneAnnotations(step.Annotations)}, nil
	}

	// applySnapshot restores a step's image bytes (if the step has a
	// screenshot) and annotation list from a snapshot — used by
	// undo/redo/reset to jump straight to a prior state in one atomic
	// step, rather than replaying individual mutations.
	applySnapshot := func(stepIndex int, snap editSnapshot) (session.Step, error) {
		step, err := findStep(stepIndex)
		if err != nil {
			return session.Step{}, err
		}
		if step.ImageFile != "" && snap.Image != nil {
			if err := os.WriteFile(filepath.Join(dir, step.ImageFile), snap.Image, 0o644); err != nil {
				return session.Step{}, fmt.Errorf("write image: %w", err)
			}
		}
		if _, err := session.SetAnnotations(name, stepIndex, snap.Annotations); err != nil {
			return session.Step{}, err
		}
		return findStep(stepIndex)
	}

	writeStep := func(w http.ResponseWriter, stepIndex int) {
		step, err := findStep(stepIndex)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(step)
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
	mux.HandleFunc("POST /api/title", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Index int    `json:"index"`
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := session.EditTitle(name, body.Index, body.Title); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/window", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Index       int    `json:"index"`
			WindowApp   string `json:"window_app"`
			WindowTitle string `json:"window_title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := session.EditWindowInfo(name, body.Index, body.WindowApp, body.WindowTitle); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	// /api/edit-image is redaction only — highlights go through
	// /api/annotation instead (decision #18), since unlike redaction
	// they're meant to stay movable/renumberable/deletable, not baked
	// into the image's pixels. style is "solid" (default, genuinely
	// unrecoverable) or "blur" (obscured but not guaranteed
	// irreversible — never used for actual secrets).
	mux.HandleFunc("POST /api/edit-image", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Index int     `json:"index"`
			Style string  `json:"style"`
			X     float64 `json:"x"`
			Y     float64 `json:"y"`
			W     float64 `json:"w"`
			H     float64 `json:"h"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		before, err := stepSnapshot(body.Index)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := session.EditImage(name, body.Index, body.Style, body.X, body.Y, body.W, body.H); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Only recorded once the edit actually succeeded — a failed edit
		// shouldn't leave a phantom "before" state with no matching
		// "after" on the undo stack.
		history.push(body.Index, before)
		writeStep(w, body.Index)
	})
	mux.HandleFunc("POST /api/annotation", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Index int     `json:"index"`
			X     float64 `json:"x"`
			Y     float64 `json:"y"`
			W     float64 `json:"w"`
			H     float64 `json:"h"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		before, err := stepSnapshot(body.Index)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		created, _, err := session.AddAnnotation(name, body.Index, body.X, body.Y, body.W, body.H)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		history.push(body.Index, before)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(created)
	})
	mux.HandleFunc("POST /api/annotation/update", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Index  int     `json:"index"`
			ID     int     `json:"id"`
			X      float64 `json:"x"`
			Y      float64 `json:"y"`
			W      float64 `json:"w"`
			H      float64 `json:"h"`
			Number int     `json:"number"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		before, err := stepSnapshot(body.Index)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := session.UpdateAnnotation(name, body.Index, body.ID, body.X, body.Y, body.W, body.H, body.Number); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		history.push(body.Index, before)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/annotation/delete", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Index int `json:"index"`
			ID    int `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		before, err := stepSnapshot(body.Index)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := session.DeleteAnnotation(name, body.Index, body.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		history.push(body.Index, before)
		w.WriteHeader(http.StatusNoContent)
	})
	// /api/annotation/reorder changes stacking/paint order only — array
	// order in the request is the new z-order, top (last) painted last.
	mux.HandleFunc("POST /api/annotation/reorder", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Index int   `json:"index"`
			Order []int `json:"order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		before, err := stepSnapshot(body.Index)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := session.ReorderAnnotations(name, body.Index, body.Order); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		history.push(body.Index, before)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/undo", func(w http.ResponseWriter, r *http.Request) {
		handleHistoryOp(w, r, name, history.undoStep, stepSnapshot, applySnapshot, "undo")
	})
	mux.HandleFunc("POST /api/redo", func(w http.ResponseWriter, r *http.Request) {
		handleHistoryOp(w, r, name, history.redoStep, stepSnapshot, applySnapshot, "redo")
	})
	mux.HandleFunc("POST /api/reset", func(w http.ResponseWriter, r *http.Request) {
		handleHistoryOp(w, r, name, history.resetStep, stepSnapshot, applySnapshot, "reset")
	})

	return mux, nil
}

// handleHistoryOp is shared by the undo/redo/reset endpoints: look up the
// step's current snapshot, ask the history store for the replacement
// (op is history.undoStep/redoStep/resetStep — all share this shape),
// apply it, log one edit entry for the action, and respond with the
// step's new state.
func handleHistoryOp(
	w http.ResponseWriter, r *http.Request, name string,
	op func(stepIndex int, current editSnapshot) (editSnapshot, bool),
	stepSnapshot func(stepIndex int) (editSnapshot, error),
	applySnapshot func(stepIndex int, snap editSnapshot) (session.Step, error),
	logAction string,
) {
	var body struct {
		Index int `json:"index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	current, err := stepSnapshot(body.Index)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	replacement, ok := op(body.Index, current)
	if !ok {
		http.Error(w, "nothing to undo/redo/reset for this step", http.StatusBadRequest)
		return
	}
	step, err := applySnapshot(body.Index, replacement)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := session.LogEdit(name, session.EditEntry{Action: logAction, StepIndex: step.Index}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(step)
}
