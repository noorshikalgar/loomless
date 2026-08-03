// Package session defines Story's on-disk session format: a folder per
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
	"strings"
	"time"

	"github.com/noorshikalgar/story/internal/capture"
	"github.com/noorshikalgar/story/internal/mask"
)

// Step is one captured moment in a session: either a window/click capture
// (Note + window fields + ImageFile) or a terminal capture (Command +
// Output/OutputFile) — a step has one or the other, not necessarily both.
type Step struct {
	Index       int       `json:"index"`
	Timestamp   time.Time `json:"timestamp"`
	Note        string    `json:"note"`
	WindowApp   string    `json:"window_app,omitempty"`
	WindowTitle string    `json:"window_title,omitempty"`
	ImageFile   string    `json:"image_file,omitempty"` // relative to session dir

	// Command/Output/OutputFile/ExitCode are set for steps captured via
	// "story run -- <command>". Output is a possibly-truncated head/tail
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
}

// Session is one recording: a named, ordered sequence of steps.
//
// Steps themselves are capture-time data — per decision #4 in
// docs/product/11-decisions.md, never renumbered or reordered in place,
// so a Step's Index stays a stable reference to its original capture
// (and its image_file/output_file names, which are derived from it).
// DisplayOrder, Hidden, and EditLog are the presentation-time layer the
// local editor (story edit) writes to: reordering, hiding, and captioning
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

// Root returns ~/.story, the base directory for all local session data.
func Root() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".story"), nil
}

// Dir returns the folder for a named session: ~/.story/sessions/<name>.
func Dir(name string) (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "sessions", name), nil
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

// currentPath points at the session name that "story step" / "story record
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

// Current returns the session currently marked active via "record start",
// for commands that operate on "the session in progress" without a name.
func Current() (*Session, string, error) {
	p, err := currentPath()
	if err != nil {
		return nil, "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, "", fmt.Errorf("no active session (run 'story record start <name>' first): %w", err)
	}
	return Load(string(b))
}

func setCurrent(name string) error {
	p, err := currentPath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(name), 0o644)
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
	if err := os.WriteFile(metaPath(dir), b, 0o644); err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	return nil
}

// AppendStep reloads a session by name from disk, appends one step, and
// saves — narrowing (not eliminating) the race window when the click
// daemon and a manual "story step" might write concurrently, since each
// append starts from the latest on-disk state rather than a stale
// in-memory copy.
func AppendStep(name, note, windowApp, windowTitle string, image []byte) (Step, error) {
	s, dir, err := Load(name)
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
// from "story run -- <command>", and saves. Mirrors AppendStep's
// reload-then-append pattern for the same concurrent-writer reason.
func AppendTerminalStep(name, command string, output []byte, exitCode int) (Step, error) {
	s, dir, err := Load(name)
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

// Stop marks the session finished and clears the "current session" pointer.
func (s *Session) Stop(dir string) error {
	now := time.Now()
	s.StoppedAt = &now
	if err := save(dir, s); err != nil {
		return err
	}
	return clearCurrent()
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
	s, dir, err := Load(name)
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

// SetStepHidden reloads the named session, marks a step hidden or
// visible, logs the change, and saves.
func SetStepHidden(name string, stepIndex int, hidden bool) (*Session, error) {
	s, dir, err := Load(name)
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
	s, dir, err := Load(name)
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

// EditImage reloads the named session and permanently redacts (solid
// black fill — the content must be genuinely unrecoverable, not just
// hidden) or highlights (colored outline) a rectangular region of a
// step's screenshot, given in the image's own pixel coordinates. This is
// the one deliberate exception to "capture-time data is never mutated"
// (decision #4): redaction only works if the original pixels are actually
// gone, so the image file itself is overwritten in place — but the edit
// is still logged (what happened and where, never the covered content
// itself, which was never retained anywhere to leak) per
// docs/product/06-editing-and-versioning.md's redaction carve-out.
func EditImage(name string, stepIndex int, action string, x, y, w, h float64) (*Session, error) {
	if action != "redact" && action != "highlight" {
		return nil, fmt.Errorf("unknown image edit action %q (want redact or highlight)", action)
	}
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("rectangle must have positive width and height")
	}

	s, dir, err := Load(name)
	if err != nil {
		return nil, err
	}

	var target *Step
	for i := range s.Steps {
		if s.Steps[i].Index == stepIndex {
			target = &s.Steps[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("step %d does not exist in this session", stepIndex)
	}
	if target.ImageFile == "" {
		return nil, fmt.Errorf("step %d has no screenshot to edit", stepIndex)
	}

	imgPath := filepath.Join(dir, target.ImageFile)
	data, err := os.ReadFile(imgPath)
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}

	var edited []byte
	if action == "redact" {
		edited, err = capture.Redact(data, x, y, w, h)
	} else {
		edited, err = capture.DrawBoxHighlight(data, x, y, w, h)
	}
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(imgPath, edited, 0o644); err != nil {
		return nil, fmt.Errorf("write edited image: %w", err)
	}

	s.EditLog = append(s.EditLog, EditEntry{
		Timestamp: time.Now(), Action: "image_" + action, StepIndex: stepIndex,
		NewValue: fmt.Sprintf("rect(%.0f,%.0f,%.0f,%.0f)", x, y, w, h),
	})
	if err := save(dir, s); err != nil {
		return nil, err
	}
	return s, nil
}

// LogEdit reloads the named session, appends an already-built EditEntry
// (its Timestamp is set here, overwriting whatever the caller passed), and
// saves. For edits whose actual mutation happens outside this package —
// image undo/redo/reset (internal/editor's in-memory version history)
// still needs a permanent, on-disk record that something happened, the
// same as every other edit in EditLog, even though the pixel-level work
// isn't session package's to do for those three operations.
func LogEdit(name string, entry EditEntry) error {
	s, dir, err := Load(name)
	if err != nil {
		return err
	}
	entry.Timestamp = time.Now()
	s.EditLog = append(s.EditLog, entry)
	return save(dir, s)
}
