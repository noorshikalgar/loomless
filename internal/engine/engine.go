// Package engine holds Loomless's capture orchestration — starting/stopping a
// recording (including the background click daemon), capturing manual and
// terminal steps, and exporting — independent of any particular front end.
// It exists so the CLI (cmd/loomless) and the MCP server (cmd/loomless's "mcp"
// subcommand, for the agent-driven path in
// docs/product/04-usage-scenarios.md Scenario B) share one implementation
// instead of drifting apart. Nothing in this package writes to os.Stdout —
// the MCP stdio transport uses stdout for the protocol stream, so any
// stray write there would corrupt it; callers that want a live terminal
// echo (the CLI's "loomless run") do that themselves.
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

	"github.com/noorshikalgar/loomless/internal/capture"
	"github.com/noorshikalgar/loomless/internal/export"
	"github.com/noorshikalgar/loomless/internal/notify"
	"github.com/noorshikalgar/loomless/internal/session"
)

// clickDebounce collapses rapid/double clicks on (roughly) the same
// moment into a single step, per docs/product/07-edge-cases.md.
const clickDebounce = 300 * time.Millisecond

// clickAnnotationRadiusPts sizes the automatic click annotation's bounding
// box (in the same point unit as WindowInfo bounds — converted to pixels
// using the screenshot's actual size, same as the old baked-in ring was).
const clickAnnotationRadiusPts = 14.0

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
// click-capture daemon for it. highlightClicks controls whether captured
// click screenshots get the red-ring annotation — off by default
// (decision #21; was on by default per decision #15, but turned out not
// to be useful in practice), opt-in per recording. autoCaption controls
// whether each click-captured step gets an automatically-generated
// caption (decision #33) — also off by default, same reasoning: a new
// per-click behavior earns opt-in first, not default-on. A failure to
// start click-watching (e.g. no Accessibility permission) is not a fatal
// error — it's returned as warning text so the caller can surface it,
// while the session itself is still usable via manual capture.
func StartRecording(name string, highlightClicks, autoCaption bool) (dir string, watching bool, warning string, err error) {
	_, dir, err = session.Create(name)
	if err != nil {
		return "", false, "", err
	}
	// Independent of click-capture: runs (and is useful) even where
	// click-capture isn't available at all, e.g. Linux — see decision #10.
	// Best-effort, non-fatal — losing the idle nudge shouldn't block
	// starting a recording.
	startIdleWatchdog(name, dir)

	if werr := startClickDaemon(name, dir, highlightClicks, autoCaption); werr != nil {
		return dir, false, werr.Error(), nil
	}
	return dir, true, "", nil
}

// StopRecording halts the click daemon and idle watchdog (if running) for
// the current session and marks it finished.
func StopRecording() (*session.Session, error) {
	s, _, err := session.Current()
	if err != nil {
		return nil, err
	}
	return stopRecordingNamed(s.Name)
}

// StopRecordingNamed is StopRecording for an arbitrary session by name,
// rather than only the single global "current" one — the TUI's
// per-session stop action needs this, since the session a user has
// selected in a list isn't necessarily the one `loomless record start` most
// recently marked current. Safe to call regardless of which session (if
// any) is current: session.Session.Stop only clears the "current"
// pointer when it's actually pointing at the session being stopped.
func StopRecordingNamed(name string) (*session.Session, error) {
	return stopRecordingNamed(name)
}

func stopRecordingNamed(name string) (*session.Session, error) {
	dir, err := session.Dir(name)
	if err != nil {
		return nil, err
	}
	stopClickDaemon(dir)
	stopIdleWatchdog(dir)

	// Reload: the daemon may have appended steps after stopClickDaemon
	// requested it shut down but before it actually finished doing so.
	s, dir, err := session.Load(name)
	if err != nil {
		return nil, err
	}
	if err := s.Stop(dir); err != nil {
		return nil, err
	}
	return s, nil
}

// CaptureStep captures the frontmost window (best-effort) as a step on the
// current session, with an optional note. Used by both "loomless step" and
// the MCP capture_step tool. The warning return surfaces a non-fatal
// problem (couldn't resolve the frontmost window, or resolved it but the
// screenshot itself failed) without making the whole capture an error — a
// step with just a note and no screenshot is still a useful step. An
// earlier version of this function discarded these silently (`_ = err`),
// which meant "loomless step"/capture_step gave no indication anything had
// gone wrong even when every step came back with no window/image at all —
// caught via the Linux backend's container-based testing
// (docs/product/11-decisions.md #10), where a bug in the Linux
// window-resolution code was invisible until the warning path was
// restored.
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
// so "loomless run" behaves like running the command directly; the MCP
// server passes nil, since stdout is the protocol stream there and must
// never receive anything else. Returns the wrapped command's exit code
// separately from err, which is reserved for loomless-level failures (no
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
	md := export.Markdown(s, dir)
	outPath := dir + "/" + name + ".md"
	if err := os.WriteFile(outPath, []byte(md), 0o644); err != nil {
		return "", "", fmt.Errorf("write export: %w", err)
	}
	return outPath, md, nil
}

// ExportPDF renders a session to PDF and writes it alongside the session.
// Unlike ExportMarkdown, the rendered bytes aren't returned as content —
// a PDF isn't something to inline into a chat response the way Markdown
// text is; the path is the useful part.
func ExportPDF(name string) (path string, err error) {
	s, dir, err := session.Load(name)
	if err != nil {
		return "", err
	}
	data, err := export.PDF(s, dir)
	if err != nil {
		return "", err
	}
	outPath := dir + "/" + name + ".pdf"
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write export: %w", err)
	}
	return outPath, nil
}

// ExportSlides renders a session to a self-contained HTML slide deck
// (screenshots embedded as base64 data URIs, no CDN dependency — see
// decision #13) and writes it alongside the session.
func ExportSlides(name string) (path string, err error) {
	s, dir, err := session.Load(name)
	if err != nil {
		return "", err
	}
	data, err := export.Slides(s, dir)
	if err != nil {
		return "", err
	}
	outPath := dir + "/" + name + ".html"
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write export: %w", err)
	}
	return outPath, nil
}

// startClickDaemon relaunches the current executable in the background to
// run the global click listener, since holding a CFRunLoop open means that
// process can't also be the short-lived command that started the session.
func startClickDaemon(name, dir string, highlightClicks, autoCaption bool) error {
	var extraArgs []string
	if highlightClicks {
		extraArgs = append(extraArgs, "--highlight")
	}
	if autoCaption {
		extraArgs = append(extraArgs, "--auto-caption")
	}
	if err := startBackgroundProcess("__click-daemon", name, session.DaemonPIDPath(dir), session.DaemonLogPath(dir), extraArgs...); err != nil {
		return err
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
	stopBackgroundProcess(session.DaemonPIDPath(dir))
}

// startIdleWatchdog relaunches the executable to run RunIdleWatchdog in
// the background — same relaunch-and-detach pattern as the click daemon,
// but a separate process/PID since it needs to keep running even when
// click-capture itself isn't available (e.g. Linux, decision #10) or
// fails to start (no Accessibility permission).
func startIdleWatchdog(name, dir string) {
	if err := startBackgroundProcess("__idle-watchdog", name, session.IdleWatchdogPIDPath(dir), session.IdleWatchdogLogPath(dir)); err != nil {
		// Best-effort: no idle nudge is a minor loss, not worth failing
		// "record start" over. Nothing to log to here since there's no
		// caller-facing warning channel for this the way click-capture
		// has one — this is deliberately lower-stakes.
		return
	}
}

func stopIdleWatchdog(dir string) {
	stopBackgroundProcess(session.IdleWatchdogPIDPath(dir))
}

// startBackgroundProcess relaunches the current executable as `<exe>
// <subcommand> <name>`, detached (new session via Setsid so it survives
// this process exiting), with stdout/stderr redirected to logPath and its
// PID recorded at pidPath so a later stopBackgroundProcess call (from a
// different, later process — "record stop") can find and signal it.
func startBackgroundProcess(subcommand, name, pidPath, logPath string, extraArgs ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log: %w", err)
	}

	cmd := exec.Command(exe, append([]string{subcommand, name}, extraArgs...)...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start %s: %w", subcommand, err)
	}
	pid := cmd.Process.Pid
	// We don't wait on the child — it outlives this command — so release
	// our hold on it rather than leaving a zombie-avoidance goroutine.
	_ = cmd.Process.Release()

	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		return fmt.Errorf("write pid: %w", err)
	}
	return nil
}

func stopBackgroundProcess(pidPath string) {
	pidBytes, err := os.ReadFile(pidPath)
	if err != nil {
		return // nothing running
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return
	}
	if proc, err := os.FindProcess(pid); err == nil {
		_ = proc.Signal(syscall.SIGTERM)
	}
	// Let it flush any in-flight work before the caller finalizes state
	// that depends on it having stopped.
	time.Sleep(300 * time.Millisecond)
	os.Remove(pidPath)
}

// findFailureLine scans the daemon log for a line indicating the click
// hook failed to start, returning the first match. Deliberately scans the
// whole file rather than just the last line: main()'s top-level error
// handler also logs "loomless: <err>" after the daemon's own "click-daemon:
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
// startClickDaemon as "loomless __click-daemon <name>"): installs the global
// click listener and, on every debounced click, captures the frontmost
// window as a step. Runs until SIGTERM (sent by stopClickDaemon). Logging
// goes through the provided logf so the CLI can direct it to the daemon's
// log file as before. autoCaption controls whether captured steps get an
// automatically-generated caption (decision #33) — see buildAutoCaption.
func RunClickDaemon(name string, highlightClicks, autoCaption bool, logf func(format string, args ...any)) error {
	// CFRunLoopRun must run on the thread that installed the tap.
	runtime.LockOSThread()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		capture.StopClickHook()
	}()

	// lastFrontmostApp tracks which app was frontmost as of the previous
	// click, so a click that itself brings a different app to the front
	// (switching windows) can be told apart from a click made within an
	// app that was already focused. Seeded with whatever's frontmost right
	// now, not left empty — otherwise the very first click of the session
	// would look like a "just switched focus" click even when the user
	// had already been working in that app before recording started.
	var lastFrontmostApp string
	if win, ferr := capture.Frontmost(); ferr == nil {
		lastFrontmostApp = win.App
	}

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
		wantAnnotation := false
		var annX, annY, annW, annH float64
		caption := ""

		if err == nil {
			windowApp, windowTitle = win.App, win.Title
			// A click that raises a background window to the front is
			// almost always "switch my attention here," not "interact
			// with this specific spot" — the coordinates are wherever the
			// background window happened to be, not a deliberate target.
			// Marking that point is more misleading than helpful, so the
			// screenshot is still captured (the window switch is real
			// context for the story) but without an annotation.
			focusJustChanged := windowApp != lastFrontmostApp
			lastFrontmostApp = windowApp

			shot, serr := capture.Screenshot(win) // best-effort; missing Screen Recording permission shouldn't kill the daemon
			if serr == nil {
				image = shot
				// The screenshot itself stays clean — no ring baked into
				// its pixels. Per decision #18, the marker is recorded as
				// a numbered Annotation instead, so it can be moved,
				// renumbered, or removed later the same as a manually
				// drawn one, both going through the same mechanism.
				if highlightClicks && !focusJustChanged {
					// Window may have moved/resized between the click and
					// this Frontmost() lookup; clamp so the marker stays
					// on-image instead of landing off the edge.
					relX := clamp(x-win.X, 0, win.W)
					relY := clamp(y-win.Y, 0, win.H)
					if pxW, _, perr := capture.PixelSize(shot); perr == nil && win.W > 0 {
						scale := float64(pxW) / win.W
						annW = clickAnnotationRadiusPts * 2 * scale
						annH = annW
						annX = relX*scale - annW/2
						annY = relY*scale - annH/2
						wantAnnotation = true
					}
				}
			}

			// AX element grounding is only queried when actually wanted —
			// it needs its own accessibility grant beyond the click tap's
			// own (see axelement_darwin.go), so a user who never asked for
			// auto-captions never sees that extra permission prompt.
			if autoCaption {
				elem, elemOK := capture.AXElementAt(x, y)
				caption = buildAutoCaption(elem, elemOK, windowApp, focusJustChanged)
			}
		}
		step, err := session.AppendStep(name, caption, windowApp, windowTitle, image)
		if err != nil {
			logf("append step failed: %v", err)
			return
		}
		if wantAnnotation {
			shape := session.AnnotationShape{X: annX, Y: annY, W: annW, H: annH, Kind: "rect", Annotated: true}
			if _, _, aerr := session.AddAnnotation(name, step.Index, shape); aerr != nil {
				logf("add annotation failed: %v", aerr)
			}
		}
		if len(step.Masked) > 0 {
			logf("masked possible secret(s) in step %d (%s)", step.Index, strings.Join(step.Masked, ", "))
		}
		// step.Note, not the local caption variable — AppendStep already
		// ran it through the same secret-masking pass every other
		// user-facing field gets, and only the MASKED text should ever
		// leave this process for a third-party API call.
		if autoCaption && step.Note != "" {
			polishCaptionAsync(name, step.Index, step.Note, logf)
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

// idleWarnAfter and idleCheckInterval are overridable via env vars so this
// is actually testable without waiting the real default out — and so a
// real user isn't stuck with a hardcoded threshold that doesn't fit how
// they work.
func idleWarnAfter() time.Duration {
	return durationEnv("LOOMLESS_IDLE_WARN_AFTER", 20*time.Minute)
}

func idleCheckInterval() time.Duration {
	return durationEnv("LOOMLESS_IDLE_CHECK_INTERVAL", time.Minute)
}

func durationEnv(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// RunIdleWatchdog is the idle-watchdog process's entrypoint (relaunched by
// startIdleWatchdog as "loomless __idle-watchdog <name>"): periodically
// checks how long it's been since the session's last step and sends a
// desktop notification once that exceeds idleWarnAfter — the "still
// recording — stop?" soft nudge from docs/product/07-edge-cases.md, for a
// recording left running accidentally. Exits when the session is stopped
// (session.json's stopped_at gets set — detected on the next poll) or on
// SIGTERM (sent by stopIdleWatchdog). Runs unconditionally alongside
// click-capture, not as part of it, so it still works when click-capture
// itself isn't available (e.g. Linux, decision #10).
func RunIdleWatchdog(name string, logf func(format string, args ...any)) error {
	warnAfter := idleWarnAfter()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	ticker := time.NewTicker(idleCheckInterval())
	defer ticker.Stop()

	logf("idle-watchdog: watching (warn after %s)", warnAfter)
	var lastNotified time.Time
	for {
		select {
		case <-sigCh:
			logf("idle-watchdog: stopped")
			return nil
		case <-ticker.C:
			s, _, err := session.Load(name)
			if err != nil || s.StoppedAt != nil {
				logf("idle-watchdog: session finished, exiting")
				return nil
			}

			lastActivity := s.StartedAt
			if len(s.Steps) > 0 {
				lastActivity = s.Steps[len(s.Steps)-1].Timestamp
			}
			idleFor := time.Since(lastActivity)

			if idleFor < warnAfter {
				lastNotified = time.Time{} // activity resumed since the last idle streak
				continue
			}
			// Re-notify at most once per warnAfter-sized window rather
			// than spamming every check interval while still idle.
			if !lastNotified.IsZero() && time.Since(lastNotified) < warnAfter {
				continue
			}
			msg := fmt.Sprintf("%q has had no activity for %s — still recording?", name, idleFor.Round(time.Second))
			if err := notify.Send("Loomless", msg); err != nil {
				logf("idle-watchdog: notify failed: %v", err)
			} else {
				logf("idle-watchdog: notified (%s idle)", idleFor.Round(time.Second))
			}
			lastNotified = time.Now()
		}
	}
}
