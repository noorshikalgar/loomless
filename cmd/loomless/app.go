package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-isatty"

	"github.com/noorshikalgar/loomless/internal/dashboard"
	"github.com/noorshikalgar/loomless/internal/desktop"
	"github.com/noorshikalgar/loomless/internal/session"
)

// launchedAsApp reports whether this process was started by double-
// clicking (Loomless.app in Finder, loomless.exe in Explorer) rather than
// typed into a terminal: no arguments and no terminal on stdin. That's
// the case where printing CLI usage and exiting would look, to someone
// who's never opened a terminal, like the app crashed on launch.
//
// Very old macOS versions passed a "-psn_<serial>" argument to bundled
// apps launched from Finder; accept it as "no arguments" too.
func launchedAsApp(args []string) bool {
	if len(args) == 1 && strings.HasPrefix(args[0], "-psn_") {
		args = nil
	}
	if len(args) > 0 {
		return false
	}
	// A real terminal check, not "is stdin a character device": Finder
	// hands apps /dev/null, which is one.
	return !isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd())
}

// cmdApp runs the dashboard inside a native desktop window. Closing the
// window quits; a recording that's in progress keeps capturing (its
// click daemon is its own process) and shows up again as live the next
// time the app opens.
func cmdApp(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("usage: loomless app")
	}
	// A Finder-launched app has no terminal: anything written to stderr
	// vanishes. Keep it in a file a support person can ask for.
	if !stderrIsTerminal() {
		if f, err := openAppLog(); err == nil {
			os.Stderr = f
			log.SetOutput(f)
		}
	}

	ln, server, err := startDashboard()
	if err != nil {
		return err
	}
	defer server.Close()
	go server.Serve(ln) //nolint:errcheck // Close() makes Serve return http.ErrServerClosed, nothing to act on
	url := fmt.Sprintf("http://%s/", ln.Addr().String())

	err = desktop.Run(url, desktop.DefaultOptions)
	if errors.Is(err, desktop.ErrUnavailable) {
		// No native web view on this platform/build: same experience as
		// `loomless serve`, which is still a lot better than nothing.
		log.Printf("loomless: native window unavailable, opening %s in the browser", url)
		openBrowser(url)
		select {} // serve until the process is killed, like `serve`
	}
	return err
}

// startDashboard binds the dashboard to a random loopback port. Shared
// by `serve` (browser tab) and `app` (native window).
func startDashboard() (net.Listener, *http.Server, error) {
	handler, err := dashboard.NewHandler()
	if err != nil {
		return nil, nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, fmt.Errorf("start dashboard server: %w", err)
	}
	return ln, &http.Server{Handler: handler}, nil
}

func stderrIsTerminal() bool {
	return isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd())
}

// openAppLog opens ~/.loomless/app.log for appending.
func openAppLog() (*os.File, error) {
	root, err := session.Root()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(root, "app.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}
