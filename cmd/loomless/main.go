// Command loomless is the CLI (and, via "loomless mcp", the MCP server) for
// Loomless's core loop: start a recording (which watches clicks
// automatically), capture manual and terminal steps, stop, and export to
// Markdown/PDF/slides. Business logic lives in internal/engine, shared
// with the MCP server — this file is presentation only (usage text,
// printing, exit-code plumbing). See docs/product/09-roadmap.md for
// what's deliberately not here yet.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/noorshikalgar/loomless/internal/capture"
	"github.com/noorshikalgar/loomless/internal/editor"
	"github.com/noorshikalgar/loomless/internal/engine"
	"github.com/noorshikalgar/loomless/internal/session"
)

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
		err = cmdRun(os.Args[2:]) // exits the process itself on success paths; err is only non-nil for loomless-level failures
	case "windows":
		err = cmdWindows(os.Args[2:])
	case "export":
		err = cmdExport(os.Args[2:])
	case "edit":
		err = cmdEdit(os.Args[2:])
	case "tui":
		err = cmdTUI(os.Args[2:])
	case "mcp":
		err = cmdMCP(os.Args[2:])
	case "__click-daemon":
		// Internal: relaunched by "record start" as a detached background
		// process. Not part of the public CLI surface.
		err = runClickDaemon(os.Args[2:])
	case "__idle-watchdog":
		// Internal: same relaunch pattern as __click-daemon, see
		// internal/engine.RunIdleWatchdog.
		err = runIdleWatchdog(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "loomless:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `loomless — turn a working session into a readable story

Usage:
  loomless record start <name> [--highlight] [--auto-caption]   Begin a session (auto-captures clicks)
  loomless record stop             Finish the active session
  loomless step ["note"]           Manually capture the frontmost window as a step
  loomless run [--] <cmd> [args]   Run a command, capture it + its output as a step
  loomless windows list            List candidate windows to capture
  loomless export <name> [--format md|pdf|slides]   Render a session (default: md)
  loomless edit <name>             Open the local web editor (reorder/hide/caption)
  loomless tui                     Session dashboard — list, edit, export, all from the terminal
  loomless mcp                     Run the MCP server (stdio) for agent-driven capture

Session data lives under ~/.loomless/sessions/<name>/.`)
}

func cmdRecord(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: loomless record start <name> | loomless record stop")
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
	name := ""
	// Both off by default: decision #21 for the highlight ring, decision
	// #33 for auto-captions — a new per-click behavior earns opt-in
	// first, not default-on.
	highlightClicks := false
	autoCaption := false
	for _, a := range args {
		if a == "--highlight" {
			highlightClicks = true
			continue
		}
		if a == "--auto-caption" {
			autoCaption = true
			continue
		}
		if name != "" {
			return fmt.Errorf("usage: loomless record start <name> [--highlight] [--auto-caption]")
		}
		name = a
	}
	if name == "" {
		return fmt.Errorf("usage: loomless record start <name> [--highlight] [--auto-caption]")
	}

	dir, watching, warning, err := engine.StartRecording(name, highlightClicks, autoCaption)
	if err != nil {
		return err
	}
	fmt.Printf("Recording %q started (%s)\n", name, dir)
	if !watching {
		fmt.Fprintln(os.Stderr, "loomless: warning: automatic click capture not available:", warning)
		fmt.Fprintln(os.Stderr, "loomless: you can still use 'loomless step \"note\"' to capture manually")
		return nil
	}
	var extras []string
	if highlightClicks {
		extras = append(extras, "highlight ring enabled")
	}
	if autoCaption {
		extras = append(extras, "auto-captions enabled")
	}
	if len(extras) > 0 {
		fmt.Printf("Watching clicks (%s) — use 'loomless step \"note\"' to add notes without a click.\n", strings.Join(extras, ", "))
	} else {
		fmt.Println("Watching clicks — use 'loomless step \"note\"' to add notes without a click.")
	}
	return nil
}

func recordStop() error {
	s, err := engine.StopRecording()
	if err != nil {
		return err
	}
	fmt.Printf("Recording %q stopped — %d step(s)\n", s.Name, len(s.Steps))
	return nil
}

// runClickDaemon is the "__click-daemon" hidden subcommand's entrypoint —
// see engine.RunClickDaemon.
func runClickDaemon(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: loomless __click-daemon <session-name> [--highlight] [--auto-caption]")
	}
	highlightClicks := false
	autoCaption := false
	for _, a := range args[1:] {
		if a == "--highlight" {
			highlightClicks = true
		}
		if a == "--auto-caption" {
			autoCaption = true
		}
	}
	return engine.RunClickDaemon(args[0], highlightClicks, autoCaption, func(format string, a ...any) {
		fmt.Printf(format+"\n", a...)
	})
}

// runIdleWatchdog is the "__idle-watchdog" hidden subcommand's entrypoint
// — see engine.RunIdleWatchdog.
func runIdleWatchdog(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: loomless __idle-watchdog <session-name>")
	}
	return engine.RunIdleWatchdog(args[0], func(format string, a ...any) {
		fmt.Printf(format+"\n", a...)
	})
}

func cmdStep(args []string) error {
	note := ""
	if len(args) > 0 {
		note = args[0]
	}
	step, warning, err := engine.CaptureStep(note)
	if err != nil {
		return err
	}
	fmt.Printf("Step %d captured", step.Index)
	if step.WindowApp != "" {
		fmt.Printf(" (%s — %s)", step.WindowApp, step.WindowTitle)
	}
	fmt.Println()
	if warning != "" {
		fmt.Fprintln(os.Stderr, "loomless: warning:", warning)
	}
	warnIfMasked(step)
	return nil
}

// warnIfMasked surfaces that secret-masking changed a step's text, without
// ever printing the matched content itself — see internal/mask.
func warnIfMasked(step session.Step) {
	if len(step.Masked) > 0 {
		fmt.Fprintf(os.Stderr, "loomless: masked possible secret(s) in step %d (%s)\n", step.Index, strings.Join(step.Masked, ", "))
	}
}

// cmdRun implements "loomless run [--] <command> [args...]": the explicit,
// per-command terminal capture wrapper (decision #6 in
// docs/product/11-decisions.md — not a full-shell-session hook). The
// command runs exactly as given, no shell interpretation, so pipes/globs
// need an explicit `sh -c "..."` if the caller wants them. Tees the
// command's real output live to the terminal — running under "loomless run"
// should look like running the command directly, not swallow its output
// while capturing it.
func cmdRun(args []string) error {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) < 1 {
		return fmt.Errorf("usage: loomless run [--] <command> [args...]")
	}

	step, exitCode, err := engine.RunCommand(args, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "loomless: step %d captured (exit %d)\n", step.Index, exitCode)
	warnIfMasked(step)

	// Propagate the wrapped command's own exit code so "loomless run --
	// make test && loomless record stop" chains the way a caller expects.
	os.Exit(exitCode)
	return nil
}

func cmdWindows(args []string) error {
	if len(args) < 1 || args[0] != "list" {
		return fmt.Errorf("usage: loomless windows list")
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
	name := ""
	format := "md"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 >= len(args) {
				return fmt.Errorf("--format needs a value (md, pdf, or slides)")
			}
			format = args[i+1]
			i++
		default:
			if name != "" {
				return fmt.Errorf("usage: loomless export <name> [--format md|pdf|slides]")
			}
			name = args[i]
		}
	}
	if name == "" {
		return fmt.Errorf("usage: loomless export <name> [--format md|pdf|slides]")
	}

	switch format {
	case "md", "markdown":
		path, _, err := engine.ExportMarkdown(name)
		if err != nil {
			return err
		}
		fmt.Println(path)
	case "pdf":
		path, err := engine.ExportPDF(name)
		if err != nil {
			return err
		}
		fmt.Println(path)
	case "slides":
		path, err := engine.ExportSlides(name)
		if err != nil {
			return err
		}
		fmt.Println(path)
	default:
		return fmt.Errorf("unknown format %q (want md, pdf, or slides)", format)
	}
	return nil
}

func cmdMCP(args []string) error {
	return runMCPServer(context.Background())
}

// cmdEdit starts the local web editor (internal/editor) bound to
// 127.0.0.1 only — this is a single-user local tool, not something to
// expose on the network — on an OS-assigned free port, best-effort opens
// it in the default browser, and serves until Ctrl+C.
func cmdEdit(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: loomless edit <name>")
	}
	name := args[0]

	handler, err := editor.NewHandler(name)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start editor server: %w", err)
	}
	url := fmt.Sprintf("http://%s/", ln.Addr().String())
	fmt.Printf("Editor for %q running at %s (Ctrl+C to stop)\n", name, url)
	openBrowser(url)

	server := &http.Server{Handler: handler}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		server.Close()
	}()

	if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// openBrowser is a package-level var, not a plain func, so tests (the
// TUI's in particular, which exercises the exact "open the editor" path
// interactively) can swap it out for a no-op rather than actually
// spawning a real browser window every test run.
var openBrowser = func(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Start() // best-effort; the printed URL works regardless if this fails
}
