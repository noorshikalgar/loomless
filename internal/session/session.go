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
	// docs/product/11-decisions.md. Output is NOT secret-masked yet.
	Command    string `json:"command,omitempty"`
	Output     string `json:"output,omitempty"`
	OutputFile string `json:"output_file,omitempty"` // relative to session dir
	ExitCode   *int   `json:"exit_code,omitempty"`
}

// Session is one recording: a named, ordered sequence of steps.
type Session struct {
	Name      string     `json:"name"`
	StartedAt time.Time  `json:"started_at"`
	StoppedAt *time.Time `json:"stopped_at,omitempty"`
	Steps     []Step     `json:"steps"`
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
	step := Step{
		Index:       len(s.Steps) + 1,
		Timestamp:   time.Now(),
		Note:        note,
		WindowApp:   windowApp,
		WindowTitle: windowTitle,
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

// AddTerminalStep appends a terminal-capture step: the full output is
// always written to outputs/step-NNNN.log, and a head/tail preview is
// stored inline on the step for rendering. Output is not secret-masked —
// see decision #6 in docs/product/11-decisions.md.
func (s *Session) AddTerminalStep(dir, command string, output []byte, exitCode int) (Step, error) {
	step := Step{
		Index:     len(s.Steps) + 1,
		Timestamp: time.Now(),
		Command:   command,
		ExitCode:  &exitCode,
	}

	filename := fmt.Sprintf("step-%04d.log", step.Index)
	if err := os.WriteFile(filepath.Join(outputsDir(dir), filename), output, 0o644); err != nil {
		return Step{}, fmt.Errorf("write command output: %w", err)
	}
	step.OutputFile = filepath.Join("outputs", filename)
	step.Output = truncateOutput(string(output), terminalPreviewHeadLines, terminalPreviewTailLines)

	s.Steps = append(s.Steps, step)
	if err := save(dir, s); err != nil {
		return Step{}, err
	}
	return step, nil
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
