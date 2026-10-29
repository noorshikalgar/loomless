// Package dashboard is a QA-friendly front door for Loomless: a single
// page listing every session (recent + full history), with buttons to
// start a new recording, stop one, rename it, export it, or open its
// editor — no CLI knowledge required. This is deliberately a thin
// presentation layer over the same internal/session and internal/engine
// functions the CLI, TUI, and MCP server already call; it holds no
// session state of its own beyond the map of per-session editor servers
// it has lazily started (mirroring cmd/loomless/tui.go's own
// runningEditor pattern, since a session's editor genuinely does need
// to keep running on its own dedicated port — see openEditor below for
// why this dashboard doesn't just mount the editor under a URL prefix
// instead).
package dashboard

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/noorshikalgar/loomless/internal/editor"
	"github.com/noorshikalgar/loomless/internal/engine"
	"github.com/noorshikalgar/loomless/internal/session"
	"github.com/noorshikalgar/loomless/internal/ui"
)

// runningEditor tracks one session's already-started per-session editor
// server, exactly like cmd/loomless/tui.go's own type of the same name —
// this package deliberately doesn't share that one (it lives in package
// main) rather than introduce a cross-cutting shared type for what's a
// two-field struct used in two presentation layers.
type runningEditor struct {
	url    string
	server *http.Server
}

// Handler is the dashboard's HTTP handler plus the small bit of
// process-lifetime state it needs: which sessions already have an
// editor server running, so clicking "Edit" twice reopens the same
// server/tab instead of binding a second port to the same session.
type Handler struct {
	mux *http.ServeMux

	editorsMu sync.Mutex
	editors   map[string]*runningEditor
}

// NewHandler builds the dashboard's HTTP handler.
func NewHandler() (http.Handler, error) {
	h := &Handler{
		mux:     http.NewServeMux(),
		editors: make(map[string]*runningEditor),
	}
	h.routes()
	return h, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) routes() {
	h.mux.HandleFunc("GET /{$}", h.handleHome)
	h.mux.HandleFunc("GET /sessions", h.handleSessions)
	h.mux.HandleFunc("POST /api/sessions", h.handleCreate)
	h.mux.HandleFunc("POST /api/sessions/{name}/stop", h.handleStop)
	h.mux.HandleFunc("POST /api/sessions/{name}/rename", h.handleRename)
	h.mux.HandleFunc("POST /api/sessions/{name}/export", h.handleExport)
	h.mux.HandleFunc("POST /api/sessions/{name}/edit", h.handleEdit)
	h.mux.HandleFunc("POST /api/sessions/{name}/reveal", h.handleReveal)
	h.mux.HandleFunc("POST /api/sessions/{name}/delete", h.handleDelete)
	h.mux.HandleFunc("POST /api/trash/{id}/restore", h.handleRestore)
	h.mux.HandleFunc("GET /api/sessions", h.handleList)
	h.mux.HandleFunc("GET /thumb/{name}", h.handleThumb)
	h.mux.Handle("/ui/", ui.Handler())
}

// summary is everything the dashboard shows about one session — the
// same facts cmd/loomless/tui.go's renderDetail shows, computed the
// same way (real files on disk, not an in-memory flag), just rendered
// as a table row instead of a detail pane.
type summary struct {
	Name        string `json:"name"`
	Recording   bool   `json:"recording"`
	Steps       int    `json:"steps"`
	Screenshots int    `json:"screenshots"`
	Started     string `json:"started"`
	StartedISO  string `json:"started_iso"`
	Stopped     string `json:"stopped,omitempty"`
	Dir         string `json:"dir"`
	ExportedMD  bool   `json:"exported_md"`
	ExportedPDF bool   `json:"exported_pdf"`
	ExportedSl  bool   `json:"exported_slides"`
	EditorURL   string `json:"editor_url,omitempty"`
	HasThumb    bool   `json:"has_thumb"`
	Duration    string `json:"duration,omitempty"`
}

func (h *Handler) summaries() ([]summary, error) {
	sessions, err := session.List()
	if err != nil {
		return nil, err
	}
	// session.List already sorts; newest-first is what both "recent" and
	// "all" views want, so sort once here rather than in each handler.
	sort.SliceStable(sessions, func(i, j int) bool {
		return sessions[i].StartedAt.After(sessions[j].StartedAt)
	})

	h.editorsMu.Lock()
	defer h.editorsMu.Unlock()

	out := make([]summary, 0, len(sessions))
	for _, s := range sessions {
		dir, err := session.Dir(s.Name)
		if err != nil {
			continue
		}
		screenshots := 0
		for _, step := range s.Steps {
			if step.ImageFile != "" {
				screenshots++
			}
		}
		sm := summary{
			Name:        s.Name,
			Recording:   s.StoppedAt == nil,
			Steps:       len(s.Steps),
			Screenshots: screenshots,
			Started:     s.StartedAt.Format("Jan 2, 2006 15:04"),
			StartedISO:  s.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
			Dir:         dir,
			ExportedMD:  fileExists(filepath.Join(dir, s.Name+".md")),
			ExportedPDF: fileExists(filepath.Join(dir, s.Name+".pdf")),
			ExportedSl:  fileExists(filepath.Join(dir, s.Name+".html")),
		}
		sm.HasThumb = screenshots > 0
		if s.StoppedAt != nil {
			sm.Stopped = s.StoppedAt.Format("2006-01-02 15:04")
			sm.Duration = humanDuration(s.StoppedAt.Sub(s.StartedAt))
		}
		if ed, ok := h.editors[s.Name]; ok {
			sm.EditorURL = ed.url
		}
		out = append(out, sm)
	}
	return out, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (h *Handler) handleHome(w http.ResponseWriter, r *http.Request) {
	sums, err := h.summaries()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	recent := sums
	if len(recent) > 6 {
		recent = recent[:6]
	}
	// Anything still recording gets a banner above everything else: for
	// a non-technical user, "is it still capturing my clicks?" is the
	// single most important piece of state on the page.
	var recording []summary
	for _, s := range sums {
		if s.Recording {
			recording = append(recording, s)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, homeHTML(recent, len(sums), recording))
}

func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	sums, err := h.summaries()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, sessionsHTML(sums))
}

// handleCreate starts a new recording — the browser equivalent of
// `loomless record start <name>`. Highlight ring and auto-captions stay
// off by default here too, same reasoning as the TUI's own "n" (new
// session) key: a new per-click behavior earns opt-in, not default-on.
func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	_, watching, warning, err := engine.StartRecording(body.Name, false, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Watching bool   `json:"watching"`
		Warning  string `json:"warning,omitempty"`
	}{watching, warning})
}

func (h *Handler) handleStop(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := engine.StopRecordingNamed(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleRename(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := session.Rename(name, body.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// The editor server we may have started for the old name is now
	// stale (session.Dir(oldName) no longer exists) — drop it so a
	// later "Edit" click starts a fresh one under the new name instead
	// of reopening a server pointed at a renamed-away directory.
	h.editorsMu.Lock()
	if ed, ok := h.editors[name]; ok {
		ed.server.Close()
		delete(h.editors, name)
	}
	h.editorsMu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// handleExport mirrors the editor's own POST /api/export exactly —
// same engine.Export* calls — so exporting from the dashboard can never
// produce different output than exporting from the CLI, TUI, or a
// session's own editor page.
func (h *Handler) handleExport(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var body struct {
		Format string `json:"format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var path string
	var err error
	switch body.Format {
	case "md", "markdown":
		path, _, err = engine.ExportMarkdown(name)
	case "pdf":
		path, err = engine.ExportPDF(name)
	case "slides":
		path, err = engine.ExportSlides(name)
	default:
		http.Error(w, fmt.Sprintf("unknown format %q (want md, pdf, or slides)", body.Format), http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Path string `json:"path"`
	}{path})
}

// handleEdit lazily starts (or reuses) a dedicated editor server for
// this session and returns its URL for the browser to open in a new
// tab. A session's editor HTML/JS (internal/editor/html.go) fetches
// its own API via absolute paths like "/api/session" — correct for the
// one-session-per-server model cmd/loomless/tui.go already uses, but
// meaning this dashboard can't just mount several sessions' editors
// under path prefixes on its own server (every session's JS would hit
// the dashboard's OWN "/api/session" instead of its own). Giving each
// session its own real server on its own port, exactly like the TUI
// already does, sidesteps that entirely rather than rewriting the
// editor's own routing to be prefix-aware for a case (multiple sessions
// open at once) that's the exception, not the rule.
func (h *Handler) handleEdit(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	h.editorsMu.Lock()
	defer h.editorsMu.Unlock()

	if ed, ok := h.editors[name]; ok {
		writeEditorURL(w, ed.url)
		return
	}

	handler, err := editor.NewHandler(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	url := fmt.Sprintf("http://%s/", ln.Addr().String())
	srv := &http.Server{Handler: handler}
	go srv.Serve(ln) //nolint:errcheck // srv.Close() always returns http.ErrServerClosed, nothing actionable
	h.editors[name] = &runningEditor{url: url, server: srv}
	writeEditorURL(w, url)
}

func writeEditorURL(w http.ResponseWriter, url string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		URL string `json:"url"`
	}{url})
}

// handleReveal shows a session's files in the OS file manager — the
// exported file itself (selected in Finder) when a format is given and
// that export exists, otherwise the session folder. For someone who
// doesn't live in a terminal, "Exported to /Users/…/.loomless/…" is a
// dead end; a button that opens the folder with the file highlighted
// is the actual finish line of the export flow.
func (h *Handler) handleReveal(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	dir, err := session.Dir(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := os.Stat(dir); err != nil {
		http.Error(w, fmt.Sprintf("session %q not found", name), http.StatusNotFound)
		return
	}
	var body struct {
		Format string `json:"format"`
	}
	// A body is optional; with none (or an empty one) we reveal the folder.
	_ = json.NewDecoder(r.Body).Decode(&body)

	target, isFile := dir, false
	if ext, ok := map[string]string{"md": ".md", "pdf": ".pdf", "slides": ".html"}[body.Format]; ok {
		if p := filepath.Join(dir, name+ext); fileExists(p) {
			target, isFile = p, true
		}
	}
	if err := revealPath(target, isFile); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// revealPath is a package-level var so tests can stub it instead of
// actually opening Finder windows.
var revealPath = func(path string, isFile bool) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		if isFile {
			cmd = exec.Command("open", "-R", path) // Finder, with the file selected
		} else {
			cmd = exec.Command("open", path)
		}
	case "linux":
		if isFile {
			path = filepath.Dir(path) // xdg-open has no "select this file" mode
		}
		cmd = exec.Command("xdg-open", path)
	default:
		return fmt.Errorf("revealing files isn't supported on %s", runtime.GOOS)
	}
	return cmd.Start()
}

// humanDuration renders a recording's length the way people say it
// ("4m 12s", "1h 03m") rather than Go's "4m12.345s".
func humanDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h, m, sec := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh %02dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm %02ds", m, sec)
	default:
		return fmt.Sprintf("%ds", sec)
	}
}

// handleList returns every session summary as JSON — what the home
// page polls while a recording is live, so its step count and latest
// screenshot update without a reload.
func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	sums, err := h.summaries()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(sums)
}

// handleThumb serves a session's first screenshot (or, with ?pos=last,
// its newest one — the live recording panel's "what was just captured"
// preview). Tiles without any screenshot get a 404 and render a
// placeholder instead.
func (h *Handler) handleThumb(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	s, dir, err := session.Load(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var img string
	if r.URL.Query().Get("pos") == "last" {
		for i := len(s.Steps) - 1; i >= 0 && img == ""; i-- {
			img = s.Steps[i].ImageFile
		}
	} else {
		for i := 0; i < len(s.Steps) && img == ""; i++ {
			img = s.Steps[i].ImageFile
		}
	}
	if img == "" {
		http.NotFound(w, r)
		return
	}
	// ImageFile comes from session.json; confine it to the session
	// directory anyway rather than trust a hand-edited file.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, filepath.Join(dir, filepath.Clean("/"+img)))
}

// handleDelete moves a session to ~/.loomless/trash (session.Trash) —
// never a hard delete — and returns the trash ID the page's "Undo"
// toast sends back to handleRestore.
func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id, err := session.Trash(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.editorsMu.Lock()
	if ed, ok := h.editors[name]; ok {
		ed.server.Close()
		delete(h.editors, name)
	}
	h.editorsMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		TrashID string `json:"trash_id"`
	}{id})
}

func (h *Handler) handleRestore(w http.ResponseWriter, r *http.Request) {
	name, err := session.Restore(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Name string `json:"name"`
	}{name})
}
