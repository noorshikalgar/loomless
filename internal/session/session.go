// Package session defines Loomless's on-disk session format: a folder per
// recording containing session.json (ordered steps) and an images/
// subfolder (one PNG per step). This is the "capture-time data" referenced
// in docs/product/06-editing-and-versioning.md — written once per step and
// not mutated by later export/edit operations.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/noorshikalgar/loomless/internal/capture"
	"github.com/noorshikalgar/loomless/internal/mask"
)

// Step is one captured moment in a session: either a window/click capture
// (Note + window fields + ImageFile) or a terminal capture (Command +
// Output/OutputFile) — a step has one or the other, not necessarily both.
type Step struct {
	Index       int       `json:"index"`
	Title       string    `json:"title,omitempty"` // user override for the step heading; empty means "Step <Index>"
	Timestamp   time.Time `json:"timestamp"`
	Note        string    `json:"note"`
	WindowApp   string    `json:"window_app,omitempty"`
	WindowTitle string    `json:"window_title,omitempty"`
	ImageFile   string    `json:"image_file,omitempty"` // relative to session dir

	// Command/Output/OutputFile/ExitCode are set for steps captured via
	// "loomless run -- <command>". Output is a possibly-truncated head/tail
	// preview (see truncateOutput); OutputFile is the untruncated raw
	// output, always saved regardless of size — see decision #6 in
	// docs/product/11-decisions.md.
	Command    string `json:"command,omitempty"`
	Output     string `json:"output,omitempty"`
	OutputFile string `json:"output_file,omitempty"` // relative to session dir
	ExitCode   *int   `json:"exit_code,omitempty"`

	// Masked names the secret-masking rules (internal/mask) that fired on
	// this step's text fields, if any — never the matched content itself.
	// Applied unconditionally before anything in the step is written to
	// disk, not something callers opt into. See docs/product/07-edge-cases.md.
	Masked []string `json:"masked,omitempty"`

	// Annotations are numbered highlight markers on ImageFile — decision
	// #18: generic and editable, unlike redaction. Both the automatic
	// click-ring (engine.RunClickDaemon) and manually-drawn highlights
	// (the local editor) go through this same list rather than each
	// baking a marker into the screenshot's pixels, which is what let
	// neither be moved, renumbered, or removed after the fact. ImageFile
	// itself stays clean; annotations are rendered as an overlay in the
	// editor and only flattened onto a copy of the image at export time
	// (internal/export). Redaction is deliberately NOT part of this list
	// — it stays pixel-destructive (EditImage), because the whole point
	// of redacting something is that it's actually gone, not an object
	// you could accidentally un-hide later.
	Annotations []Annotation `json:"annotations,omitempty"`

	// Crop is the visible region of ImageFile, in the image's own pixel
	// coordinates, or nil for "the whole image" (every step before this
	// field existed, and every step nobody has cropped). Like
	// Annotations and unlike EditImage's redaction, this is
	// presentation-time data — ImageFile's pixels are never trimmed on
	// disk, only at export/flatten time (internal/export), so a crop is
	// always adjustable or reversible later. Framing is a legibility
	// choice ("cut the noise around what matters"), not a security one;
	// nothing about cropping needs the destroy-the-pixels guarantee
	// redaction exists for.
	Crop *CropRect `json:"crop,omitempty"`

	// HeaderStyle picks which parts of the captured window info a
	// step's editor/export header shows: "" (default — App — Title,
	// the original behavior every step before this field existed
	// keeps), "app" (app name only), "title" (window title only), or
	// "hidden" (neither — the window line doesn't render at all). This
	// is purely a display choice over the SAME underlying WindowApp/
	// WindowTitle data (decision #31) — it never clears or overwrites
	// either field, so switching back to "" (or any other style) later
	// always has the full original captured info to show again.
	HeaderStyle string `json:"header_style,omitempty"`

	// IsContent marks a step authored directly in the editor (decision
	// #29) rather than captured from a window/click or terminal command —
	// a freeform Markdown block, inserted between captured steps to add
	// context a screenshot can't (an explanation, a diagram, a linked
	// video). It has no Command, ImageFile, or meaningful window info, so
	// this exists purely to tell the editor UI not to show the "+ add
	// app" window-info affordance a real capture-time note step still
	// legitimately wants — every exporter already treats an empty
	// WindowApp/Command the same way regardless of this flag, so IsContent
	// changes nothing about how a step exports, only how it's edited.
	IsContent bool `json:"is_content,omitempty"`
}

// CropRect is a rectangular region in an image's own pixel coordinates —
// same shape as an Annotation's position/size fields, but its own type
// rather than reusing Annotation, since a crop has no ID/Number/Color
// and there's at most one per step (cropping again always starts over
// from the full original image, not stacked with a previous crop).
type CropRect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Annotation is one numbered highlight marker in image pixel coordinates.
// ID is assigned once at creation and never changes — it's how the editor
// API addresses a specific annotation. Number is the user-facing badge
// shown on the marker and referenced in captions ("1: the export button,
// 2: the confirmation dialog") — freely editable, defaults to creation
// order, not required to be unique (renumbering is the user's call, not
// enforced here).
type Annotation struct {
	ID     int     `json:"id"`
	Number int     `json:"number"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	W      float64 `json:"w"`
	H      float64 `json:"h"`
	// Color is a "#rrggbb" hex string, or empty for the default
	// highlight color (capture.DefaultAnnotationColor) — every
	// annotation created before this field existed has Color == "", so
	// omitempty keeps old session.json files loading as the same
	// visible color they always rendered, not an unset/invalid one.
	Color string `json:"color,omitempty"`
	// Points, when non-empty, makes this a freehand stroke instead of a
	// rectangle highlight — X/Y/W/H still hold the stroke's bounding box
	// (recomputed from Points on every create/move, via boundingBox), so
	// every existing consumer of an annotation's position (the badge,
	// crop's coordinate-shifting, EditLog's rect summaries) keeps working
	// unchanged for both kinds without a type switch. A rectangle
	// annotation's Points is always empty — the two kinds share one list
	// (one ID space, one numbering, one undo history) rather than being
	// separate features, the same "keep it generic" reasoning decision
	// #18 already established for the click-ring vs. manual highlight.
	Points []Point `json:"points,omitempty"`

	// Kind selects how this shape renders (decision #30): "" or "rect"
	// (default, a rectangle border), "circle" (an ellipse inscribed in
	// X/Y/W/H), "line" or "arrow" (a straight segment through
	// Points[0]/Points[1] — arrow additionally draws an arrowhead at the
	// second point), or "freehand" (a path through every point in
	// Points). Every annotation saved before this field existed has
	// Kind == "" — RenderAnnotations treats "" with non-empty Points the
	// same as "freehand" (the only kind "" could ever have meant before
	// Kind existed), so old session.json files render identically
	// without any migration step.
	Kind string `json:"kind,omitempty"`

	// Annotated is whether this shape currently shows its numbered
	// badge. A shape's default, per decision #30, is UNANNOTATED — most
	// drawn shapes are just markup, not part of a numbered callout
	// sequence, and forcing every one to claim a number was the exact
	// friction this field exists to remove. Annotated is a *bool, not a
	// bool, specifically so "the JSON key is absent" (every annotation
	// saved before this field existed, which — under the old,
	// pre-decision-#30 rules — was ALWAYS shown numbered) can be told
	// apart from "explicitly set to false" (a new shape someone drew and
	// deliberately left as a plain shape): nil means "legacy, keep
	// showing its badge exactly as it always has," never "unset,
	// default to off." Every NEWLY CREATED annotation must set this
	// explicitly (see AnnotationShape) — nil is reserved for pre-
	// existing data, not a valid state for anything created going
	// forward. See IsAnnotated.
	Annotated *bool `json:"annotated,omitempty"`
}

// IsAnnotated reports whether a currently shows its numbered badge —
// the one place that resolves Annotated's nil-means-legacy-true
// convention into a real bool, so nothing else needs to know that
// convention exists.
func (a Annotation) IsAnnotated() bool {
	return a.Annotated == nil || *a.Annotated
}

// AnnotationShape carries every field that defines what a drawn shape
// looks like and whether it currently shows a numbered badge — shared
// by AddAnnotation and UpdateAnnotation now that an annotation can be
// five different kinds of shape (decision #30), which pushed what used
// to be a positional argument list (already at eight for
// UpdateAnnotation) well past the point of being readable at the call
// site.
type AnnotationShape struct {
	X, Y, W, H float64
	// Color is a "#rrggbb" hex string, or "" for the default highlight
	// color.
	Color string
	// Kind is "", "rect", "circle", "line", "arrow", or "freehand" — see
	// Annotation.Kind.
	Kind string
	// Points is nil for a box shape (rect/circle — X/Y/W/H are used as
	// given) and non-nil for a path shape (line/arrow/freehand — X/Y/W/H
	// are ignored in favor of the path's own bounding box, via
	// boundingBox, so the two can never disagree about the shape's
	// position).
	Points []Point
	// Annotated is whether this shape shows its numbered badge. Callers
	// creating a brand-new shape must set this explicitly — see
	// Annotation.Annotated's doc comment for why nil is reserved for
	// legacy data and never produced by a live create/update call.
	Annotated bool
}

// Point is one vertex of a freehand annotation's path, in the image's
// own pixel coordinates.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// boundingBox returns the smallest rectangle containing every point —
// used to keep a freehand annotation's X/Y/W/H in sync with its Points
// whenever the path is created or moved, so every place that already
// reads an annotation's position/size doesn't need to know freehand
// annotations exist at all.
func boundingBox(points []Point) (x, y, w, h float64) {
	if len(points) == 0 {
		return 0, 0, 0, 0
	}
	minX, minY := points[0].X, points[0].Y
	maxX, maxY := points[0].X, points[0].Y
	for _, p := range points[1:] {
		minX = min(minX, p.X)
		minY = min(minY, p.Y)
		maxX = max(maxX, p.X)
		maxY = max(maxY, p.Y)
	}
	return minX, minY, maxX - minX, maxY - minY
}

// Session is one recording: a named, ordered sequence of steps.
//
// Steps themselves are capture-time data — per decision #4 in
// docs/product/11-decisions.md, never renumbered or reordered in place,
// so a Step's Index stays a stable reference to its original capture
// (and its image_file/output_file names, which are derived from it).
// DisplayOrder, Hidden, and EditLog are the presentation-time layer the
// local editor (loomless edit) writes to: reordering, hiding, and captioning
// change how a story is *read*, not what was *captured*.
type Session struct {
	Name      string     `json:"name"`
	StartedAt time.Time  `json:"started_at"`
	StoppedAt *time.Time `json:"stopped_at,omitempty"`
	Steps     []Step     `json:"steps"`

	// DisplayOrder lists Step.Index values in the order they should be
	// rendered. Empty/nil means natural capture order — every exporter
	// must go through VisibleSteps() rather than ranging over Steps
	// directly, so this is never forgotten in a new export format.
	DisplayOrder []int `json:"display_order,omitempty"`
	// Hidden lists Step.Index values excluded from rendering. The
	// underlying Step and its files are never deleted — "hidden" is
	// reversible, matching the edit log's own reversibility.
	Hidden []int `json:"hidden,omitempty"`
	// EditLog is every edit made after capture, oldest first — the
	// "visible activity log" docs/product/06-editing-and-versioning.md
	// requires so a reader can tell a story was touched after recording,
	// not just trust it silently.
	EditLog []EditEntry `json:"edit_log,omitempty"`
}

// EditEntry is one recorded change to a session's presentation layer.
type EditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"` // "reorder" | "hide" | "unhide" | "edit_note"
	StepIndex int       `json:"step_index"`
	OldValue  string    `json:"old_value,omitempty"`
	NewValue  string    `json:"new_value,omitempty"`
}

// Root returns ~/.loomless, the base directory for all local session data.
func Root() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".loomless"), nil
}

// Dir returns the folder for a named session: ~/.loomless/sessions/<name>.
func Dir(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "sessions", name), nil
}

// ValidateName rejects any session name that isn't a single, plain path
// component — empty, ".", "..", or anything containing a path separator
// would let filepath.Join in Dir resolve somewhere other than one
// directory directly under ~/.loomless/sessions (e.g. "../../x" escapes
// to ~/x). Enforced here, in Dir, rather than in each caller, because
// every entry point — Create, Load, Rename, export, the editor, the
// browser dashboard — goes through Dir; a check in only some of them is
// exactly how Create ended up without one while Rename had it.
func ValidateName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") || strings.ContainsRune(name, 0) {
		return fmt.Errorf("invalid session name %q (use letters, numbers, dashes — no slashes)", name)
	}
	return nil
}

func metaPath(dir string) string {
	return filepath.Join(dir, "session.json")
}

func imagesDir(dir string) string {
	return filepath.Join(dir, "images")
}

func outputsDir(dir string) string {
	return filepath.Join(dir, "outputs")
}

// DaemonPIDPath and DaemonLogPath locate the click-capture daemon's
// bookkeeping files for a session, so "record start" and "record stop"
// (which run as separate short-lived processes) agree on where to find
// them without passing state between them.
func DaemonPIDPath(dir string) string {
	return filepath.Join(dir, "daemon.pid")
}

func DaemonLogPath(dir string) string {
	return filepath.Join(dir, "daemon.log")
}

// IdleWatchdogPIDPath and IdleWatchdogLogPath are the same pattern as
// DaemonPIDPath/DaemonLogPath, for the separate idle-timeout watchdog
// process (internal/engine's startIdleWatchdog) — kept as its own
// process/PID rather than folded into the click daemon so the idle nudge
// still works even on platforms/permission states where click-capture
// itself isn't available (e.g. Linux — see decision #10).
func IdleWatchdogPIDPath(dir string) string {
	return filepath.Join(dir, "idle-watchdog.pid")
}

func IdleWatchdogLogPath(dir string) string {
	return filepath.Join(dir, "idle-watchdog.log")
}

// currentPath points at the session name that "loomless step" / "loomless record
// stop" act on when no name is given explicitly.
func currentPath() (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "current"), nil
}

// Create starts a new session on disk and marks it current. Fails if a
// session with this name already exists.
func Create(name string) (*Session, string, error) {
	dir, err := Dir(name)
	if err != nil {
		return nil, "", err
	}
	if _, err := os.Stat(dir); err == nil {
		return nil, "", fmt.Errorf("session %q already exists", name)
	}
	if err := os.MkdirAll(imagesDir(dir), 0o755); err != nil {
		return nil, "", fmt.Errorf("create session dir: %w", err)
	}
	if err := os.MkdirAll(outputsDir(dir), 0o755); err != nil {
		return nil, "", fmt.Errorf("create session dir: %w", err)
	}
	s := &Session{Name: name, StartedAt: time.Now()}
	if err := save(dir, s); err != nil {
		return nil, "", err
	}
	if err := setCurrent(name); err != nil {
		return nil, "", err
	}
	return s, dir, nil
}

// List returns every session found under ~/.loomless/sessions, sorted by
// StartedAt descending (most recent first) — used by the TUI's session
// list. A session directory that fails to load (corrupted or mid-write
// session.json) is skipped rather than failing the whole listing, since
// one bad session shouldn't hide every other one from view.
func List() ([]Session, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(root, "sessions"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	var sessions []Session
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, _, err := Load(e.Name())
		if err != nil {
			continue
		}
		sessions = append(sessions, *s)
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartedAt.After(sessions[j].StartedAt)
	})
	return sessions, nil
}

// Load reads a session by name.
func Load(name string) (*Session, string, error) {
	dir, err := Dir(name)
	if err != nil {
		return nil, "", err
	}
	b, err := os.ReadFile(metaPath(dir))
	if err != nil {
		return nil, "", fmt.Errorf("read session %q: %w", name, err)
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, "", fmt.Errorf("parse session %q: %w", name, err)
	}
	return &s, dir, nil
}

// Rename changes a session's on-disk name — the session identity itself
// (folder name, Session.Name, and the "current" pointer if it was
// pointing at this session), distinct from EditTitle, which only
// overrides one step's display heading.
//
// A session still actively recording (StoppedAt == nil) can't be safely
// renamed: the click daemon (internal/engine.RunClickDaemon) was started
// with, and holds in memory for its whole lifetime, the name it was
// given — it has no way to learn the name changed, so every subsequent
// click would try to load a directory that no longer exists at that
// path. Rejected outright rather than left as a silent landmine; the
// caller needs to stop the recording first.
func Rename(oldName, newName string) (*Session, error) {
	if newName == oldName {
		// Renaming to the same name is a no-op, not an error, and
		// doesn't need the recording-state check below.
		s, _, err := Load(oldName)
		return s, err
	}
	if err := ValidateName(newName); err != nil {
		return nil, err
	}

	s, oldDir, err := Load(oldName)
	if err != nil {
		return nil, err
	}
	if s.StoppedAt == nil {
		return nil, fmt.Errorf("session %q is still recording — stop it before renaming", oldName)
	}

	newDir, err := Dir(newName)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(newDir); err == nil {
		return nil, fmt.Errorf("a session named %q already exists", newName)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	// Checked before the directory move below, not after: once oldDir is
	// gone, Current() can no longer load it to compare names against.
	wasCurrent := false
	if cur, _, err := Current(); err == nil && cur.Name == oldName {
		wasCurrent = true
	}

	if err := os.Rename(oldDir, newDir); err != nil {
		return nil, fmt.Errorf("rename session directory: %w", err)
	}
	s.Name = newName
	if err := save(newDir, s); err != nil {
		return nil, err
	}

	if wasCurrent {
		if err := setCurrent(newName); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Current returns the session currently marked active via "record start",
// for commands that operate on "the session in progress" without a name.
func Current() (*Session, string, error) {
	p, err := currentPath()
	if err != nil {
		return nil, "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, "", fmt.Errorf("no active session (run 'loomless record start <name>' first): %w", err)
	}
	return Load(string(b))
}

func setCurrent(name string) error {
	p, err := currentPath()
	if err != nil {
		return err
	}
	return writeFileAtomic(p, []byte(name))
}

func clearCurrent() error {
	p, err := currentPath()
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func save(dir string, s *Session) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	if err := writeFileAtomic(metaPath(dir), b); err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	return nil
}

// writeFileAtomic writes data to a temp file in path's own directory and
// renames it over path. A plain os.WriteFile truncates first and writes
// second, so any concurrent reader — the dashboard listing sessions, the
// editor loading one, the TUI refreshing — could catch a zero-length or
// half-written session.json mid-write and fail to parse it ("unexpected
// end of JSON input"). rename(2) within one directory is atomic: readers
// see either the complete old file or the complete new one, never a mix.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// loadForUpdate is Load for read-modify-write callers: it takes the
// session directory's cross-process lock (lockDir) BEFORE reading, so the
// version being modified is guaranteed to still be the latest when it's
// saved back. The returned unlock is always non-nil and safe to defer
// immediately, even on error.
func loadForUpdate(name string) (*Session, string, func(), error) {
	noop := func() {}
	dir, err := Dir(name)
	if err != nil {
		return nil, "", noop, err
	}
	unlock, err := lockDir(dir)
	if err != nil {
		// Most likely the session doesn't exist (no directory to put a
		// lock file in) — let Load produce its normal, clearer error.
		if _, _, lerr := Load(name); lerr != nil {
			return nil, "", noop, lerr
		}
		return nil, "", noop, fmt.Errorf("lock session %q: %w", name, err)
	}
	s, dir, err := Load(name)
	if err != nil {
		unlock()
		return nil, "", noop, err
	}
	return s, dir, unlock, nil
}

// StopNamed marks a session stopped under the session lock — the locked
// replacement for callers that used to Load and then call (*Session).Stop
// themselves, which left a window for a concurrent writer's change to be
// overwritten by the stale copy.
func StopNamed(name string) (*Session, error) {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return nil, err
	}
	if err := s.Stop(dir); err != nil {
		return nil, err
	}
	return s, nil
}

// AppendStep reloads a session by name from disk, appends one step, and
// saves — narrowing (not eliminating) the race window when the click
// daemon and a manual "loomless step" might write concurrently, since each
// append starts from the latest on-disk state rather than a stale
// in-memory copy.
func AppendStep(name, note, windowApp, windowTitle string, image []byte) (Step, error) {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return Step{}, err
	}
	return s.AddStep(dir, note, windowApp, windowTitle, image)
}

// AddStep appends a step, writes the screenshot bytes (if any) alongside
// it, and persists the updated session.
func (s *Session) AddStep(dir, note, windowApp, windowTitle string, image []byte) (Step, error) {
	var matched []string
	note, m := mask.Text(note)
	matched = mergeRules(matched, m)
	windowTitle, m = mask.Text(windowTitle)
	matched = mergeRules(matched, m)

	step := Step{
		Index:       len(s.Steps) + 1,
		Timestamp:   time.Now(),
		Note:        note,
		WindowApp:   windowApp,
		WindowTitle: windowTitle,
		Masked:      matched,
	}
	if len(image) > 0 {
		filename := fmt.Sprintf("step-%04d.png", step.Index)
		if err := os.WriteFile(filepath.Join(imagesDir(dir), filename), image, 0o644); err != nil {
			return Step{}, fmt.Errorf("write screenshot: %w", err)
		}
		step.ImageFile = filepath.Join("images", filename)
	}
	s.Steps = append(s.Steps, step)
	if err := save(dir, s); err != nil {
		return Step{}, err
	}
	return step, nil
}

// terminalPreviewHeadLines/TailLines bound how much raw command output is
// inlined into the step (and so into the rendered story) — per
// docs/product/07-edge-cases.md, a full build log shouldn't turn one step
// into an unreadable wall of text. The untruncated output is always saved
// to OutputFile regardless.
const (
	terminalPreviewHeadLines = 20
	terminalPreviewTailLines = 20
)

// AppendTerminalStep reloads a session by name, appends a step captured
// from "loomless run -- <command>", and saves. Mirrors AppendStep's
// reload-then-append pattern for the same concurrent-writer reason.
func AppendTerminalStep(name, command string, output []byte, exitCode int) (Step, error) {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return Step{}, err
	}
	return s.AddTerminalStep(dir, command, output, exitCode)
}

// AddTerminalStep appends a terminal-capture step. Command and output are
// secret-masked (internal/mask) before either the log file or session.json
// is written — per docs/product/07-edge-cases.md, masking after the fact
// is too late, the raw secret would already have touched disk. The full
// (masked) output is always written to outputs/step-NNNN.log, and a
// head/tail preview is stored inline on the step for rendering.
func (s *Session) AddTerminalStep(dir, command string, output []byte, exitCode int) (Step, error) {
	var matched []string
	command, m := mask.Text(command)
	matched = mergeRules(matched, m)
	maskedOutput, m := mask.Text(string(output))
	matched = mergeRules(matched, m)

	step := Step{
		Index:     len(s.Steps) + 1,
		Timestamp: time.Now(),
		Command:   command,
		ExitCode:  &exitCode,
		Masked:    matched,
	}

	filename := fmt.Sprintf("step-%04d.log", step.Index)
	if err := os.WriteFile(filepath.Join(outputsDir(dir), filename), []byte(maskedOutput), 0o644); err != nil {
		return Step{}, fmt.Errorf("write command output: %w", err)
	}
	step.OutputFile = filepath.Join("outputs", filename)
	step.Output = truncateOutput(maskedOutput, terminalPreviewHeadLines, terminalPreviewTailLines)

	s.Steps = append(s.Steps, step)
	if err := save(dir, s); err != nil {
		return Step{}, err
	}
	return step, nil
}

// mergeRules appends rule names from src into dst, skipping ones already
// present — a step can trigger the same rule across multiple fields (e.g.
// a token in both the command and its output).
func mergeRules(dst, src []string) []string {
	for _, name := range src {
		found := false
		for _, existing := range dst {
			if existing == name {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, name)
		}
	}
	return dst
}

// truncateOutput keeps the first headLines and last tailLines of output,
// collapsing anything in between into a one-line summary — the "head/tail
// preview, link to full output" approach from docs/product/07-edge-cases.md.
func truncateOutput(output string, headLines, tailLines int) string {
	output = strings.TrimRight(output, "\n")
	if output == "" {
		return ""
	}
	lines := strings.Split(output, "\n")
	if len(lines) <= headLines+tailLines {
		return output
	}
	omitted := len(lines) - headLines - tailLines
	head := strings.Join(lines[:headLines], "\n")
	tail := strings.Join(lines[len(lines)-tailLines:], "\n")
	return fmt.Sprintf("%s\n\n… %d lines omitted (see full output file) …\n\n%s", head, omitted, tail)
}

// Stop marks the session finished, and clears the "current session"
// pointer only if it was actually pointing at this session — stopping
// session A must never blow away session B's "current" marker just
// because B happens to be the live one at the time. That distinction
// didn't matter while the only caller (engine.StopRecording) always
// operated via Current() itself, guaranteeing a match; it does matter
// once a session can be stopped by name (engine.StopRecordingNamed),
// where the target and the current session are no longer guaranteed to
// be the same one.
func (s *Session) Stop(dir string) error {
	now := time.Now()
	s.StoppedAt = &now
	if err := save(dir, s); err != nil {
		return err
	}
	if cur, _, err := Current(); err == nil && cur.Name == s.Name {
		return clearCurrent()
	}
	return nil
}

// VisibleSteps returns steps in display order with hidden ones excluded —
// every exporter (Markdown/PDF/Slides) must use this instead of ranging
// over Session.Steps directly, or a reorder/hide edit silently wouldn't
// take effect in that format.
func (s *Session) VisibleSteps() []Step {
	byIndex := make(map[int]Step, len(s.Steps))
	for _, step := range s.Steps {
		byIndex[step.Index] = step
	}
	hidden := make(map[int]bool, len(s.Hidden))
	for _, i := range s.Hidden {
		hidden[i] = true
	}

	order := s.DisplayOrder
	if len(order) == 0 {
		order = make([]int, len(s.Steps))
		for i, step := range s.Steps {
			order[i] = step.Index
		}
	}

	visible := make([]Step, 0, len(order))
	for _, idx := range order {
		if hidden[idx] {
			continue
		}
		if step, ok := byIndex[idx]; ok {
			visible = append(visible, step)
		}
	}
	return visible
}

// ReorderSteps reloads the named session, replaces DisplayOrder wholesale
// with newOrder (validated to be a permutation of the session's actual
// step indices — partial or garbage input is rejected rather than
// silently dropping steps from the story), logs the change, and saves.
func ReorderSteps(name string, newOrder []int) (*Session, error) {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return nil, err
	}

	valid := make(map[int]bool, len(s.Steps))
	for _, step := range s.Steps {
		valid[step.Index] = true
	}
	if len(newOrder) != len(valid) {
		return nil, fmt.Errorf("reorder must include exactly the session's %d step(s), got %d", len(valid), len(newOrder))
	}
	seen := make(map[int]bool, len(newOrder))
	for _, idx := range newOrder {
		if !valid[idx] {
			return nil, fmt.Errorf("step %d does not exist in this session", idx)
		}
		if seen[idx] {
			return nil, fmt.Errorf("step %d listed more than once", idx)
		}
		seen[idx] = true
	}

	s.DisplayOrder = newOrder
	s.EditLog = append(s.EditLog, EditEntry{Timestamp: time.Now(), Action: "reorder", NewValue: fmt.Sprint(newOrder)})
	if err := save(dir, s); err != nil {
		return nil, err
	}
	return s, nil
}

// InsertContentStep reloads the named session and creates a new, empty
// IsContent step (decision #29) at the given position in the display
// order — 0 inserts before every other step, len(order) inserts after
// the last one, and anything in between is clamped rather than
// rejected, since the position comes from the editor's own current
// rendering of the list and can't meaningfully be "invalid" the way an
// out-of-range step index would be. Unlike AddStep (used by live
// capture, which only ever appends), this is the one place a step can
// land anywhere in the story, which is exactly the point: a content
// step exists to add context between two ALREADY-captured steps, not
// just at the end. Returns the new step's index alongside the updated
// session so the caller (the editor's /api/insert-step handler) can
// tell the frontend which card to focus.
func InsertContentStep(name string, position int) (*Session, int, error) {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return nil, 0, err
	}

	order := s.DisplayOrder
	if len(order) == 0 {
		order = make([]int, len(s.Steps))
		for i, step := range s.Steps {
			order[i] = step.Index
		}
	}
	if position < 0 {
		position = 0
	}
	if position > len(order) {
		position = len(order)
	}

	step := Step{Index: len(s.Steps) + 1, Timestamp: time.Now(), IsContent: true}
	s.Steps = append(s.Steps, step)

	newOrder := make([]int, 0, len(order)+1)
	newOrder = append(newOrder, order[:position]...)
	newOrder = append(newOrder, step.Index)
	newOrder = append(newOrder, order[position:]...)
	s.DisplayOrder = newOrder

	s.EditLog = append(s.EditLog, EditEntry{
		Timestamp: time.Now(), Action: "insert_content_step", StepIndex: step.Index,
		NewValue: fmt.Sprintf("position %d", position),
	})
	if err := save(dir, s); err != nil {
		return nil, 0, err
	}
	return s, step.Index, nil
}

// SetStepHidden reloads the named session, marks a step hidden or
// visible, logs the change, and saves.
func SetStepHidden(name string, stepIndex int, hidden bool) (*Session, error) {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return nil, err
	}

	found := false
	for _, step := range s.Steps {
		if step.Index == stepIndex {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("step %d does not exist in this session", stepIndex)
	}

	currentlyHidden := false
	newHidden := s.Hidden[:0:0]
	for _, idx := range s.Hidden {
		if idx == stepIndex {
			currentlyHidden = true
			continue // drop it; re-added below if hidden==true
		}
		newHidden = append(newHidden, idx)
	}
	if hidden {
		newHidden = append(newHidden, stepIndex)
	}
	s.Hidden = newHidden

	if hidden != currentlyHidden {
		action := "unhide"
		if hidden {
			action = "hide"
		}
		s.EditLog = append(s.EditLog, EditEntry{Timestamp: time.Now(), Action: action, StepIndex: stepIndex})
	}
	if err := save(dir, s); err != nil {
		return nil, err
	}
	return s, nil
}

// EditNote reloads the named session, replaces a step's Note (masked the
// same way capture-time text is — an edited caption is just as capable of
// containing a pasted secret as an originally-typed one), logs the old
// and new values, and saves.
func EditNote(name string, stepIndex int, newNote string) (*Session, error) {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return nil, err
	}

	maskedNote, matched := mask.Text(newNote)
	found := false
	var oldNote string
	for i := range s.Steps {
		if s.Steps[i].Index == stepIndex {
			oldNote = s.Steps[i].Note
			s.Steps[i].Note = maskedNote
			s.Steps[i].Masked = mergeRules(s.Steps[i].Masked, matched)
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("step %d does not exist in this session", stepIndex)
	}

	s.EditLog = append(s.EditLog, EditEntry{
		Timestamp: time.Now(), Action: "edit_note", StepIndex: stepIndex,
		OldValue: oldNote, NewValue: maskedNote,
	})
	if err := save(dir, s); err != nil {
		return nil, err
	}
	return s, nil
}

// EditTitle reloads the named session and sets a step's display title —
// the user-facing override for the "Step <Index>" heading every exporter
// and the editor otherwise fall back to. An empty title clears the
// override rather than being rejected, since "go back to the default
// heading" is a legitimate edit, not an error. Masked the same way Note
// is: a custom title is free-text a user typed, just as capable of
// carrying a pasted secret as a caption.
func EditTitle(name string, stepIndex int, newTitle string) (*Session, error) {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return nil, err
	}

	maskedTitle, matched := mask.Text(newTitle)
	found := false
	var oldTitle string
	for i := range s.Steps {
		if s.Steps[i].Index == stepIndex {
			oldTitle = s.Steps[i].Title
			s.Steps[i].Title = maskedTitle
			s.Steps[i].Masked = mergeRules(s.Steps[i].Masked, matched)
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("step %d does not exist in this session", stepIndex)
	}

	s.EditLog = append(s.EditLog, EditEntry{
		Timestamp: time.Now(), Action: "edit_title", StepIndex: stepIndex,
		OldValue: oldTitle, NewValue: maskedTitle,
	})
	if err := save(dir, s); err != nil {
		return nil, err
	}
	return s, nil
}

// EditWindowInfo reloads the named session and overwrites a step's
// WindowApp/WindowTitle — captured automatically at record time, but not
// always accurate (a generic process name, a stale title) or what the
// user wants to show in the finished story. Masked the same way
// WindowTitle already is at capture time (see AddStep).
func EditWindowInfo(name string, stepIndex int, app, windowTitle string) (*Session, error) {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return nil, err
	}

	maskedTitle, matched := mask.Text(windowTitle)
	found := false
	var oldApp, oldTitle string
	for i := range s.Steps {
		if s.Steps[i].Index == stepIndex {
			oldApp, oldTitle = s.Steps[i].WindowApp, s.Steps[i].WindowTitle
			s.Steps[i].WindowApp = app
			s.Steps[i].WindowTitle = maskedTitle
			s.Steps[i].Masked = mergeRules(s.Steps[i].Masked, matched)
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("step %d does not exist in this session", stepIndex)
	}

	s.EditLog = append(s.EditLog, EditEntry{
		Timestamp: time.Now(), Action: "edit_window", StepIndex: stepIndex,
		OldValue: oldApp + " — " + oldTitle, NewValue: app + " — " + maskedTitle,
	})
	if err := save(dir, s); err != nil {
		return nil, err
	}
	return s, nil
}

// validHeaderStyles are the only values EditHeaderStyle accepts —
// rejecting anything else outright rather than silently storing a typo
// that would then just fall through to "" (Both) behavior with no
// indication anything was wrong.
var validHeaderStyles = map[string]bool{"": true, "app": true, "title": true, "hidden": true}

// EditHeaderStyle reloads the named session and sets which parts of a
// step's captured window info its header displays (decision #31) —
// "", "app", "title", or "hidden". This never touches WindowApp/
// WindowTitle themselves, only how they're shown, so it's always
// reversible with the original data intact.
func EditHeaderStyle(name string, stepIndex int, style string) (*Session, error) {
	if !validHeaderStyles[style] {
		return nil, fmt.Errorf("invalid header style %q (want \"\", \"app\", \"title\", or \"hidden\")", style)
	}
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return nil, err
	}

	found := false
	var old string
	for i := range s.Steps {
		if s.Steps[i].Index == stepIndex {
			old = s.Steps[i].HeaderStyle
			s.Steps[i].HeaderStyle = style
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("step %d does not exist in this session", stepIndex)
	}

	s.EditLog = append(s.EditLog, EditEntry{
		Timestamp: time.Now(), Action: "edit_header_style", StepIndex: stepIndex,
		OldValue: old, NewValue: style,
	})
	if err := save(dir, s); err != nil {
		return nil, err
	}
	return s, nil
}

// EditImage reloads the named session and permanently redacts a
// rectangular region of a step's screenshot, given in the image's own
// pixel coordinates, using the given style (capture.RedactStyleSolid or
// capture.RedactStyleBlur — an empty style defaults to solid). This is
// the one deliberate exception to "capture-time data is never mutated"
// (decision #4): redaction only works if the original pixels are actually
// gone (or, for blur, thoroughly obscured), so the image file itself is
// overwritten in place — but the edit is still logged (what happened,
// where, and which style, never the covered content itself, which was
// never retained anywhere to leak) per
// docs/product/06-editing-and-versioning.md's redaction carve-out.
func EditImage(name string, stepIndex int, style string, x, y, w, h float64) (*Session, error) {
	if style == "" {
		style = capture.RedactStyleSolid
	}
	if style != capture.RedactStyleSolid && style != capture.RedactStyleBlur {
		return nil, fmt.Errorf("unknown redact style %q (want %q or %q)", style, capture.RedactStyleSolid, capture.RedactStyleBlur)
	}
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("rectangle must have positive width and height")
	}

	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return nil, err
	}
	target, err := findStep(s, stepIndex)
	if err != nil {
		return nil, err
	}
	if target.ImageFile == "" {
		return nil, fmt.Errorf("step %d has no screenshot to edit", stepIndex)
	}

	imgPath := filepath.Join(dir, target.ImageFile)
	data, err := os.ReadFile(imgPath)
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}

	edited, err := capture.Redact(data, x, y, w, h, style)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(imgPath, edited, 0o644); err != nil {
		return nil, fmt.Errorf("write edited image: %w", err)
	}

	s.EditLog = append(s.EditLog, EditEntry{
		Timestamp: time.Now(), Action: "image_redact", StepIndex: stepIndex,
		NewValue: fmt.Sprintf("%s rect(%.0f,%.0f,%.0f,%.0f)", style, x, y, w, h),
	})
	if err := save(dir, s); err != nil {
		return nil, err
	}
	return s, nil
}

// findStep returns a pointer into s.Steps for the given step index, so
// callers can mutate it in place before save() — shared by EditImage and
// every Annotation function below to avoid repeating the same lookup loop.
func findStep(s *Session, stepIndex int) (*Step, error) {
	for i := range s.Steps {
		if s.Steps[i].Index == stepIndex {
			return &s.Steps[i], nil
		}
	}
	return nil, fmt.Errorf("step %d does not exist in this session", stepIndex)
}

// AddAnnotation reloads the named session, appends a new shape to a
// step, logs the change, and saves. Used by the click daemon's
// automatic ring and every shape the editor's toolbar can draw (decision
// #18's "keep it generic" principle, extended by decision #30 to cover
// rect/circle/line/arrow/freehand as one family rather than separate
// features). Its badge number is auto-assigned one past the highest
// existing ANNOTATED number on the step regardless of whether shape.
// Annotated is itself true — so a shape drawn while unannotated and
// toggled on later still gets a sensible number, not 0 or a collision —
// though the badge itself only renders when Annotated is true. Returns
// the created Annotation directly (not just buried in the returned
// Session) so a caller like the editor's API can hand it straight back
// to the client without making the client guess which entry in the
// step's list is the new one.
func AddAnnotation(name string, stepIndex int, shape AnnotationShape) (Annotation, *Session, error) {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return Annotation{}, nil, err
	}
	target, err := findStep(s, stepIndex)
	if err != nil {
		return Annotation{}, nil, err
	}

	x, y, w, h := shape.X, shape.Y, shape.W, shape.H
	if shape.Points != nil {
		x, y, w, h = boundingBox(shape.Points)
	}

	nextID, nextNumber := 1, 1
	for _, a := range target.Annotations {
		if a.ID >= nextID {
			nextID = a.ID + 1
		}
		if a.Number >= nextNumber {
			nextNumber = a.Number + 1
		}
	}
	annotated := shape.Annotated
	created := Annotation{
		ID: nextID, Number: nextNumber, X: x, Y: y, W: w, H: h,
		Color: shape.Color, Points: shape.Points, Kind: shape.Kind, Annotated: &annotated,
	}
	target.Annotations = append(target.Annotations, created)

	s.EditLog = append(s.EditLog, EditEntry{
		Timestamp: time.Now(), Action: "add_annotation", StepIndex: stepIndex,
		NewValue: fmt.Sprintf("#%d %s(%.0f,%.0f,%.0f,%.0f)", nextNumber, shapeKindOrDefault(shape.Kind), x, y, w, h),
	})
	if err := save(dir, s); err != nil {
		return Annotation{}, nil, err
	}
	return created, s, nil
}

func shapeKindOrDefault(kind string) string {
	if kind == "" {
		return "rect"
	}
	return kind
}

// SetStepPresentation reloads the named session and replaces a step's
// entire annotation list and crop rectangle wholesale — every piece of
// *presentation-time* step state (as opposed to EditImage's pixel-level
// redaction, or the on-disk screenshot itself). Low-level and
// deliberately unlogged itself (unlike every other function in this
// file) — it's the primitive the editor's unified undo/redo/reset
// (internal/editor/history.go) uses to restore a prior snapshot
// atomically; the caller is responsible for logging the semantic action
// ("undo"/"redo"/"reset" — see LogEdit), the same way image undo/redo/
// reset already do for pixel restores.
func SetStepPresentation(name string, stepIndex int, anns []Annotation, crop *CropRect) (*Session, error) {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return nil, err
	}
	target, err := findStep(s, stepIndex)
	if err != nil {
		return nil, err
	}
	target.Annotations = anns
	target.Crop = crop
	if err := save(dir, s); err != nil {
		return nil, err
	}
	return s, nil
}

// ReorderAnnotations reloads the named session and reorders a step's
// annotation list. List order doubles as paint/stacking order — later in
// the list renders on top, in both the editor and every exporter — so
// this is also how "bring to front"/"send to back" work, not a separate
// z-index field to keep in sync. newOrder must be a permutation of the
// step's existing annotation IDs; partial or garbage input is rejected
// rather than silently dropping an annotation from the stack.
func ReorderAnnotations(name string, stepIndex int, newOrder []int) (*Session, error) {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return nil, err
	}
	target, err := findStep(s, stepIndex)
	if err != nil {
		return nil, err
	}

	byID := make(map[int]Annotation, len(target.Annotations))
	for _, a := range target.Annotations {
		byID[a.ID] = a
	}
	if len(newOrder) != len(byID) {
		return nil, fmt.Errorf("reorder must include exactly the step's %d annotation(s), got %d", len(byID), len(newOrder))
	}
	seen := make(map[int]bool, len(newOrder))
	reordered := make([]Annotation, 0, len(newOrder))
	for _, id := range newOrder {
		a, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("annotation %d does not exist on step %d", id, stepIndex)
		}
		if seen[id] {
			return nil, fmt.Errorf("annotation %d listed more than once", id)
		}
		seen[id] = true
		reordered = append(reordered, a)
	}
	target.Annotations = reordered

	s.EditLog = append(s.EditLog, EditEntry{
		Timestamp: time.Now(), Action: "reorder_annotations", StepIndex: stepIndex,
		NewValue: fmt.Sprint(newOrder),
	})
	if err := save(dir, s); err != nil {
		return nil, err
	}
	return s, nil
}

// UpdateAnnotation reloads the named session and replaces an existing
// annotation's position, size, display number, and shape fields —
// identified by its stable ID, which never changes, not by its position
// in the list or its (freely reassignable) Number. Every commit path in
// the editor (drag/resize, renumber, recolor, and toggling the
// annotated badge) must resend the FULL shape, not just the field it
// changed — there is no partial merge here, matching the exact lesson
// decision #25 already learned about color (a plain drag that forgot to
// resend color would have silently reset a custom color back to
// default; the same is now true for Kind and Annotated too).
// shape.Points is nil for every update that isn't moving a path shape
// (rect/circle resize or move, renumber, recolor, or toggling annotated
// — shape.X/Y/W/H are used as given); non-nil replaces the path and
// recomputes X/Y/W/H from it via boundingBox, ignoring the given
// X/Y/W/H, for the same reason AddAnnotation does.
func UpdateAnnotation(name string, stepIndex, annotationID, number int, shape AnnotationShape) (*Session, error) {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return nil, err
	}
	target, err := findStep(s, stepIndex)
	if err != nil {
		return nil, err
	}

	x, y, w, h := shape.X, shape.Y, shape.W, shape.H
	if shape.Points != nil {
		x, y, w, h = boundingBox(shape.Points)
	}

	var old Annotation
	found := false
	annotated := shape.Annotated
	for i := range target.Annotations {
		if target.Annotations[i].ID == annotationID {
			old = target.Annotations[i]
			target.Annotations[i].X, target.Annotations[i].Y = x, y
			target.Annotations[i].W, target.Annotations[i].H = w, h
			target.Annotations[i].Number = number
			target.Annotations[i].Color = shape.Color
			target.Annotations[i].Kind = shape.Kind
			target.Annotations[i].Annotated = &annotated
			if shape.Points != nil {
				target.Annotations[i].Points = shape.Points
			}
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("annotation %d does not exist on step %d", annotationID, stepIndex)
	}

	s.EditLog = append(s.EditLog, EditEntry{
		Timestamp: time.Now(), Action: "update_annotation", StepIndex: stepIndex,
		OldValue: fmt.Sprintf("#%d %s(%.0f,%.0f,%.0f,%.0f)", old.Number, shapeKindOrDefault(old.Kind), old.X, old.Y, old.W, old.H),
		NewValue: fmt.Sprintf("#%d %s(%.0f,%.0f,%.0f,%.0f)", number, shapeKindOrDefault(shape.Kind), x, y, w, h),
	})
	if err := save(dir, s); err != nil {
		return nil, err
	}
	return s, nil
}

// DeleteAnnotation reloads the named session, removes an annotation
// (identified by ID) from a step, logs the change, and saves. The
// underlying screenshot is untouched — annotations were never baked into
// its pixels, so there's nothing to restore.
func DeleteAnnotation(name string, stepIndex, annotationID int) (*Session, error) {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return nil, err
	}
	target, err := findStep(s, stepIndex)
	if err != nil {
		return nil, err
	}

	idx := -1
	for i, a := range target.Annotations {
		if a.ID == annotationID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, fmt.Errorf("annotation %d does not exist on step %d", annotationID, stepIndex)
	}
	removed := target.Annotations[idx]
	target.Annotations = append(target.Annotations[:idx], target.Annotations[idx+1:]...)

	s.EditLog = append(s.EditLog, EditEntry{
		Timestamp: time.Now(), Action: "delete_annotation", StepIndex: stepIndex,
		OldValue: fmt.Sprintf("#%d rect(%.0f,%.0f,%.0f,%.0f)", removed.Number, removed.X, removed.Y, removed.W, removed.H),
	})
	if err := save(dir, s); err != nil {
		return nil, err
	}
	return s, nil
}

// SetCrop reloads the named session and sets (or replaces) a step's
// crop rectangle, in the image's own pixel coordinates.
func SetCrop(name string, stepIndex int, x, y, w, h float64) (*Session, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("crop rectangle must have positive width and height")
	}
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return nil, err
	}
	target, err := findStep(s, stepIndex)
	if err != nil {
		return nil, err
	}
	old := target.Crop
	target.Crop = &CropRect{X: x, Y: y, W: w, H: h}

	s.EditLog = append(s.EditLog, EditEntry{
		Timestamp: time.Now(), Action: "set_crop", StepIndex: stepIndex,
		OldValue: cropLogValue(old), NewValue: cropLogValue(target.Crop),
	})
	if err := save(dir, s); err != nil {
		return nil, err
	}
	return s, nil
}

// ClearCrop reloads the named session and removes a step's crop
// rectangle, restoring the full original image at export time. A no-op
// (not an error) if the step had no crop set — clearing an already-clear
// crop isn't a mistake worth rejecting.
func ClearCrop(name string, stepIndex int) (*Session, error) {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return nil, err
	}
	target, err := findStep(s, stepIndex)
	if err != nil {
		return nil, err
	}
	if target.Crop == nil {
		return s, nil
	}
	old := target.Crop
	target.Crop = nil

	s.EditLog = append(s.EditLog, EditEntry{
		Timestamp: time.Now(), Action: "clear_crop", StepIndex: stepIndex,
		OldValue: cropLogValue(old),
	})
	if err := save(dir, s); err != nil {
		return nil, err
	}
	return s, nil
}

func cropLogValue(c *CropRect) string {
	if c == nil {
		return "none"
	}
	return fmt.Sprintf("rect(%.0f,%.0f,%.0f,%.0f)", c.X, c.Y, c.W, c.H)
}

// LogEdit reloads the named session, appends an already-built EditEntry
// (its Timestamp is set here, overwriting whatever the caller passed), and
// saves. For edits whose actual mutation happens outside this package —
// image undo/redo/reset (internal/editor's in-memory version history)
// still needs a permanent, on-disk record that something happened, the
// same as every other edit in EditLog, even though the pixel-level work
// isn't session package's to do for those three operations.
func LogEdit(name string, entry EditEntry) error {
	s, dir, unlock, err := loadForUpdate(name)
	defer unlock()
	if err != nil {
		return err
	}
	entry.Timestamp = time.Now()
	s.EditLog = append(s.EditLog, entry)
	return save(dir, s)
}
