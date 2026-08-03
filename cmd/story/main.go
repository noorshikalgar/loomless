// Command story is the CLI for Story's core loop: start a recording
// (which watches clicks automatically), capture manual steps, stop, and
// export to Markdown. See docs/product/09-roadmap.md for what's
// deliberately not here yet (terminal capture, annotations, MCP).
package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "record":
		err = cmdRecord(os.Args[2:])
	case "step":
		err = cmdStep(os.Args[2:])
	case "run":
		err = cmdRun(os.Args[2:]) // exits the process itself on success paths; err is only non-nil for story-level failures
	case "windows":
		err = cmdWindows(os.Args[2:])
	case "export":
		err = cmdExport(os.Args[2:])
	case "__click-daemon":
		// Internal: relaunched by "record start" as a detached background
		// process. Not part of the public CLI surface.
		err = runClickDaemon(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "story:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `story — turn a working session into a readable story

Usage:
  story record start <name>     Begin a session (auto-captures clicks)
  story record stop             Finish the active session
  story step ["note"]           Manually capture the frontmost window as a step
  story run [--] <cmd> [args]   Run a command, capture it + its output as a step
  story windows list            List candidate windows to capture
  story export <name>           Render a session to Markdown

Session data lives under ~/.story/sessions/<name>/.`)
}

func cmdRecord(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: story record start <name> | story record stop")
	}
	switch args[0] {
	case "start":
		return recordStart(args[1:])
	case "stop":
		return recordStop()
	default:
		return fmt.Errorf("unknown record subcommand %q", args[0])
	}
}

func recordStart(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: story record start <name>")
	}
	name := args[0]
	_, dir, err := session.Create(name)
	if err != nil {
		return err
	}
	fmt.Printf("Recording %q started (%s)\n", name, dir)

	if err := startClickDaemon(name, dir); err != nil {
		fmt.Fprintln(os.Stderr, "story: warning: automatic click capture not available:", err)
		fmt.Fprintln(os.Stderr, "story: you can still use 'story step \"note\"' to capture manually")
		return nil
	}
	fmt.Println("Watching clicks — use 'story step \"note\"' to add notes without a click.")
	return nil
}

func recordStop() error {
	s, dir, err := session.Current()
	if err != nil {
		return err
	}
	stopClickDaemon(dir)

	// Reload: the daemon may have appended steps after our snapshot above.
	s, dir, err = session.Load(s.Name)
	if err != nil {
		return err
	}
	if err := s.Stop(dir); err != nil {
		return err
	}
	fmt.Printf("Recording %q stopped — %d step(s)\n", s.Name, len(s.Steps))
	return nil
}

// startClickDaemon relaunches the current executable in the background to
// run the global click listener, since holding a CFRunLoop open means that
// process can't also be the short-lived CLI command the user just ran.
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
	// started (it needs Accessibility permission) before we tell the user
	// everything's fine.
	time.Sleep(300 * time.Millisecond)
	if status, err := readLastLine(session.DaemonLogPath(dir)); err == nil && strings.Contains(status, "failed") {
		return fmt.Errorf("%s", status)
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

func readLastLine(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var last string
	for scanner.Scan() {
		last = scanner.Text()
	}
	return last, nil
}

// runClickDaemon is the click-daemon's entrypoint: installs the global
// click listener and, on every debounced click, captures the frontmost
// window as a step. Runs until SIGTERM (sent by "record stop").
func runClickDaemon(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: story __click-daemon <session-name>")
	}
	name := args[0]

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
			fmt.Println("append step failed:", err)
			return
		}
		warnIfMasked(step)
	}

	fmt.Println("click-daemon: listening")
	if err := capture.RunClickHook(onClick); err != nil {
		fmt.Println("click-daemon: failed:", err)
		return err
	}
	fmt.Println("click-daemon: stopped")
	return nil
}

func cmdStep(args []string) error {
	note := ""
	if len(args) > 0 {
		note = args[0]
	}

	s, _, err := session.Current()
	if err != nil {
		return err
	}

	win, werr := capture.Frontmost()
	var image []byte
	windowApp, windowTitle := "", ""
	if werr == nil {
		windowApp, windowTitle = win.App, win.Title
		image, err = capture.Screenshot(win)
		if err != nil {
			fmt.Fprintln(os.Stderr, "story: warning: screenshot failed:", err)
		}
	} else {
		fmt.Fprintln(os.Stderr, "story: warning: couldn't resolve frontmost window:", werr)
	}

	step, err := session.AppendStep(s.Name, note, windowApp, windowTitle, image)
	if err != nil {
		return err
	}
	fmt.Printf("Step %d captured", step.Index)
	if windowApp != "" {
		fmt.Printf(" (%s — %s)", windowApp, windowTitle)
	}
	fmt.Println()
	warnIfMasked(step)
	return nil
}

// warnIfMasked surfaces that secret-masking changed a step's text, without
// ever printing the matched content itself — see internal/mask.
func warnIfMasked(step session.Step) {
	if len(step.Masked) > 0 {
		fmt.Fprintf(os.Stderr, "story: masked possible secret(s) in step %d (%s)\n", step.Index, strings.Join(step.Masked, ", "))
	}
}

// cmdRun implements "story run [--] <command> [args...]": the explicit,
// per-command terminal capture wrapper (decision #6 in
// docs/product/11-decisions.md — not a full-shell-session hook). The
// command runs exactly as given, no shell interpretation, so pipes/globs
// need an explicit `sh -c "..."` if the caller wants them.
func cmdRun(args []string) error {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) < 1 {
		return fmt.Errorf("usage: story run [--] <command> [args...]")
	}

	s, _, err := session.Current()
	if err != nil {
		return err
	}

	cmd := exec.Command(args[0], args[1:]...)
	var buf bytes.Buffer
	// Tee to the real terminal too — this should behave like running the
	// command directly, not swallow its output while capturing.
	cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &buf)
	cmd.Stdin = os.Stdin

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return fmt.Errorf("run %q: %w", strings.Join(args, " "), runErr)
		}
	}

	step, err := session.AppendTerminalStep(s.Name, strings.Join(args, " "), buf.Bytes(), exitCode)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "story: step %d captured (exit %d)\n", step.Index, exitCode)
	warnIfMasked(step)

	// Propagate the wrapped command's own exit code so "story run --
	// make test && story record stop" chains the way a caller expects.
	os.Exit(exitCode)
	return nil
}

func cmdWindows(args []string) error {
	if len(args) < 1 || args[0] != "list" {
		return fmt.Errorf("usage: story windows list")
	}
	windows, err := capture.List()
	if err != nil {
		return err
	}
	if len(windows) == 0 {
		fmt.Println("No windows found.")
		return nil
	}
	for _, w := range windows {
		fmt.Printf("%s — %s (%.0fx%.0f at %.0f,%.0f)\n", w.App, w.Title, w.W, w.H, w.X, w.Y)
	}
	return nil
}

func cmdExport(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: story export <name>")
	}
	name := args[0]
	s, dir, err := session.Load(name)
	if err != nil {
		return err
	}
	md := export.Markdown(s)
	outPath := filepath.Join(dir, name+".md")
	if err := os.WriteFile(outPath, []byte(md), 0o644); err != nil {
		return fmt.Errorf("write export: %w", err)
	}
	fmt.Println(outPath)
	return nil
}
