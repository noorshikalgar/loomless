// Command story is the CLI for Story's Phase 0 core loop: start a
// recording, capture window-scoped steps, stop, export to Markdown.
// See docs/product/09-roadmap.md for what's deliberately not here yet
// (auto click-capture, terminal capture, annotations, PDF/slides).
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/noorshikalgar/story/internal/capture"
	"github.com/noorshikalgar/story/internal/export"
	"github.com/noorshikalgar/story/internal/session"
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
	case "windows":
		err = cmdWindows(os.Args[2:])
	case "export":
		err = cmdExport(os.Args[2:])
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
  story record start <name>     Begin a new session
  story record stop             Finish the active session
  story step ["note"]           Capture the frontmost window as a step
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
		if len(args) < 2 {
			return fmt.Errorf("usage: story record start <name>")
		}
		name := args[1]
		_, dir, err := session.Create(name)
		if err != nil {
			return err
		}
		fmt.Printf("Recording %q started (%s)\n", name, dir)
		return nil
	case "stop":
		s, dir, err := session.Current()
		if err != nil {
			return err
		}
		if err := s.Stop(dir); err != nil {
			return err
		}
		fmt.Printf("Recording %q stopped — %d step(s)\n", s.Name, len(s.Steps))
		return nil
	default:
		return fmt.Errorf("unknown record subcommand %q", args[0])
	}
}

func cmdStep(args []string) error {
	note := ""
	if len(args) > 0 {
		note = args[0]
	}

	s, dir, err := session.Current()
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

	step, err := s.AddStep(dir, note, windowApp, windowTitle, image)
	if err != nil {
		return err
	}
	fmt.Printf("Step %d captured", step.Index)
	if windowApp != "" {
		fmt.Printf(" (%s — %s)", windowApp, windowTitle)
	}
	fmt.Println()
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
