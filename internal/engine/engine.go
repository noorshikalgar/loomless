// Package engine holds Story's capture orchestration — starting/stopping a
// recording (including the background click daemon), capturing manual and
// terminal steps, and exporting — independent of any particular front end.
// It exists so the CLI (cmd/story) and the MCP server (cmd/story's "mcp"
// subcommand, for the agent-driven path in
// docs/product/04-usage-scenarios.md Scenario B) share one implementation
// instead of drifting apart. Nothing in this package writes to os.Stdout —
// the MCP stdio transport uses stdout for the protocol stream, so any
// stray write there would corrupt it; callers that want a live terminal
// echo (the CLI's "story run") do that themselves.
package engine

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/noorshikalgar/story/internal/capture"
	"github.com/noorshikalgar/story/internal/export"
	"github.com/noorshikalgar/story/internal/session"
)

// clickDebounce collapses rapid/double clicks on (roughly) the same
// moment into a single step, per docs/product/07-edge-cases.md.
const clickDebounce = 300 * time.Millisecond

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// StartRecording creates a session and, best-effort, starts the background
// click-capture daemon for it. A failure to start click-watching (e.g. no
// Accessibility permission) is not a fatal error — it's returned as
// warning text so the caller can surface it, while the session itself is
// still usable via manual capture.
func StartRecording(name string) (dir string, watching bool, warning string, err error) {
	_, dir, err = session.Create(name)
	if err != nil {
		return "", false, "", err
	}
	if werr := startClickDaemon(name, dir); werr != nil {
		return dir, false, werr.Error(), nil
	}
	return dir, true, "", nil
}

// StopRecording halts the click daemon (if running) for the current
// session and marks it finished.
func StopRecording() (*session.Session, error) {
	s, dir, err := session.Current()
	if err != nil {
		return nil, err
	}
	stopClickDaemon(dir)

	// Reload: the daemon may have appended steps after our snapshot above.
	s, dir, err = session.Load(s.Name)
	if err != nil {
		return nil, err
	}
	if err := s.Stop(dir); err != nil {
		return nil, err
	}
	return s, nil
}

// CaptureStep captures the frontmost window (best-effort) as a step on the
// current session, with an optional note. Used by both "story step" and
// the MCP capture_step tool.
// CaptureStep's warning return surfaces a non-fatal problem (couldn't
// resolve the frontmost window, or resolved it but the screenshot itself
// failed) without making the whole capture an error — a step with just a
// note and no screenshot is still a useful step. An earlier version of
// this function discarded these silently (`_ = err`), which meant "story
// step"/capture_step gave no indication anything had gone wrong even when
// every step came back with no window/image at all — caught via the Linux
// backend's container-based testing (docs/product/11-decisions.md #10),
// where a bug in the Linux window-resolution code was invisible until the
// warning path was restored.
func CaptureStep(note string) (step session.Step, warning string, err error) {
	s, _, err := session.Current()
	if err != nil {
		return session.Step{}, "", err
	}

	win, werr := capture.Frontmost()
	var image []byte
	windowApp, windowTitle := "", ""
	if werr != nil {
		warning = fmt.Sprintf("couldn't resolve frontmost window: %v", werr)
	} else {
		windowApp, windowTitle = win.App, win.Title
		image, err = capture.Screenshot(win)
		if err != nil {
			warning = fmt.Sprintf("screenshot failed: %v", err)
			err = nil
		}
	}

	step, err = session.AppendStep(s.Name, note, windowApp, windowTitle, image)
	return step, warning, err
}

// RunCommand executes a command (no shell interpretation — args[0] is the
// program, args[1:] its arguments) on the current session, capturing its
// combined stdout/stderr as a step. teeStdout/teeStderr, if non-nil, also
// receive the live output as it happens — the CLI passes os.Stdout/Stderr
// so "story run" behaves like running the command directly; the MCP
// server passes nil, since stdout is the protocol stream there and must
// never receive anything else. Returns the wrapped command's exit code
// separately from err, which is reserved for story-level failures (no
// active session, the command couldn't even start).
func RunCommand(args []string, teeStdout, teeStderr io.Writer) (step session.Step, exitCode int, err error) {
	if len(args) < 1 {
		return session.Step{}, 0, fmt.Errorf("no command given")
	}
	s, _, err := session.Current()
	if err != nil {
		return session.Step{}, 0, err
	}

	cmd := exec.Command(args[0], args[1:]...)
	var buf bytes.Buffer
	cmd.Stdout = teeWriter(&buf, teeStdout)
	cmd.Stderr = teeWriter(&buf, teeStderr)

	runErr := cmd.Run()
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return session.Step{}, 0, fmt.Errorf("run %q: %w", strings.Join(args, " "), runErr)
		}
	}

	step, err = session.AppendTerminalStep(s.Name, strings.Join(args, " "), buf.Bytes(), exitCode)
	if err != nil {
		return session.Step{}, 0, err
	}
	return step, exitCode, nil
}

// teeWriter returns w alone if tee is nil, otherwise a writer that fans
// out to both — avoids handing exec.Cmd a MultiWriter wrapping a nil
// io.Writer, which io.MultiWriter doesn't handle as "no-op".
func teeWriter(w io.Writer, tee io.Writer) io.Writer {
	if tee == nil {
		return w
	}
	return io.MultiWriter(w, tee)
}

// ExportMarkdown renders a session to Markdown, writes it alongside the
// session, and returns both the file path and the rendered content (the
// agent path wants the content directly to include in its own response —
// see Scenario B in docs/product/04-usage-scenarios.md — not just a path).
func ExportMarkdown(name string) (path, content string, err error) {
	s, dir, err := session.Load(name)
	if err != nil {
		return "", "", err
	}
	md := export.Markdown(s)
	outPath := dir + "/" + name + ".md"
	if err := os.WriteFile(outPath, []byte(md), 0o644); err != nil {
		return "", "", fmt.Errorf("write export: %w", err)
	}
	return outPath, md, nil
}

// startClickDaemon relaunches the current executable in the background to
// run the global click listener, since holding a CFRunLoop open means that
// process can't also be the short-lived command that started the session.
func startClickDaemon(name, dir string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	logFile, err := os.Create(session.DaemonLogPath(dir))
	if err != nil {
		return fmt.Errorf("create daemon log: %w", err)
	}

	cmd := exec.Command(exe, "__click-daemon", name)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start click daemon: %w", err)
	}
	pid := cmd.Process.Pid
	// We don't wait on the child — it outlives this command — so release
	// our hold on it rather than leaving a zombie-avoidance goroutine.
	_ = cmd.Process.Release()

	if err := os.WriteFile(session.DaemonPIDPath(dir), []byte(strconv.Itoa(pid)), 0o644); err != nil {
		return fmt.Errorf("write daemon pid: %w", err)
	}

	// Give the daemon a moment to report whether the event tap actually
	// started (it needs Accessibility permission) before we tell the
	// caller everything's fine.
	time.Sleep(300 * time.Millisecond)
	if failLine, ok := findFailureLine(session.DaemonLogPath(dir)); ok {
		return fmt.Errorf("%s", failLine)
	}
	return nil
}

func stopClickDaemon(dir string) {
	pidBytes, err := os.ReadFile(session.DaemonPIDPath(dir))
	if err != nil {
		return // no daemon was running for this session
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return
	}
	if proc, err := os.FindProcess(pid); err == nil {
		_ = proc.Signal(syscall.SIGTERM)
	}
	// Let it flush the in-flight step (if any) before we finalize the
	// session file.
	time.Sleep(300 * time.Millisecond)
	os.Remove(session.DaemonPIDPath(dir))
}

// findFailureLine scans the daemon log for a line indicating the click
// hook failed to start, returning the first match. Deliberately scans the
// whole file rather than just the last line: main()'s top-level error
// handler also logs "story: <err>" after the daemon's own "click-daemon:
// failed: ..." line (both share the daemon's redirected stdout/stderr),
// so a last-line-only check can land on that generic wrapper line instead
// — which doesn't contain "failed" — and silently miss a real failure.
// Caught via the Linux capture backend's container-based verification
// (docs/product/11-decisions.md #10): the click hook is *always* expected
// to fail there right now (no Linux click-capture implementation yet, see
// clickhook_linux.go), which is exactly the case this bug caused
// "record start" to misreport as "watching_clicks: true."
func findFailureLine(path string) (line string, found bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "failed") {
			return scanner.Text(), true
		}
	}
	return "", false
}

// RunClickDaemon is the click-daemon process's entrypoint (relaunched by
// startClickDaemon as "story __click-daemon <name>"): installs the global
// click listener and, on every debounced click, captures the frontmost
// window as a step. Runs until SIGTERM (sent by stopClickDaemon). Logging
// goes through the provided logf so the CLI can direct it to the daemon's
// log file as before.
func RunClickDaemon(name string, logf func(format string, args ...any)) error {
	// CFRunLoopRun must run on the thread that installed the tap.
	runtime.LockOSThread()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		capture.StopClickHook()
	}()

	var lastCapture time.Time
	onClick := func(x, y float64) {
		now := time.Now()
		if now.Sub(lastCapture) < clickDebounce {
			return
		}
		lastCapture = now

		win, err := capture.Frontmost()
		windowApp, windowTitle := "", ""
		var image []byte
		if err == nil {
			windowApp, windowTitle = win.App, win.Title
			shot, serr := capture.Screenshot(win) // best-effort; missing Screen Recording permission shouldn't kill the daemon
			if serr == nil {
				// Window may have moved/resized between the click and this
				// Frontmost() lookup; clamp so the marker stays on-image
				// instead of drawing off the edge.
				relX := clamp(x-win.X, 0, win.W)
				relY := clamp(y-win.Y, 0, win.H)
				if annotated, aerr := capture.Highlight(shot, relX, relY, win.W, win.H); aerr == nil {
					shot = annotated
				}
				image = shot
			}
		}
		step, err := session.AppendStep(name, "", windowApp, windowTitle, image)
		if err != nil {
			logf("append step failed: %v", err)
			return
		}
		if len(step.Masked) > 0 {
			logf("masked possible secret(s) in step %d (%s)", step.Index, strings.Join(step.Masked, ", "))
		}
	}

	logf("click-daemon: listening")
	if err := capture.RunClickHook(onClick); err != nil {
		logf("click-daemon: failed: %v", err)
		return err
	}
	logf("click-daemon: stopped")
	return nil
}
