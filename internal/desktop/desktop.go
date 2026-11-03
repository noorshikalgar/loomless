// Package desktop shows Loomless's local web UI (the dashboard and the
// per-session editors) inside a native application window instead of a
// browser tab, so a QA tester double-clicks an app and gets an app —
// no terminal, no browser, no "which tab was it in".
//
// The UI itself is unchanged: the same HTTP handlers serve the same
// pages, and the window is just a system web view (WKWebView on macOS)
// pointed at the loopback server. That keeps one UI codebase for the
// browser (`loomless serve`) and the desktop app (`loomless app`).
package desktop

import "errors"

// ErrUnavailable is returned by Run when this platform (or this build)
// has no native web view. Callers fall back to opening the system
// browser, which is exactly what `loomless serve` does.
var ErrUnavailable = errors.New("native window not available on this platform")

// Options configures the window Run opens.
type Options struct {
	Title         string
	Width, Height int // initial content size, in points
	MinWidth      int // the dashboard's layout collapses below ~860pt; don't let the window go much narrower
	MinHeight     int
}

// DefaultOptions is the window Loomless opens on launch: large enough
// for the library grid's three columns, small enough for a 13" laptop.
var DefaultOptions = Options{
	Title:     "Loomless",
	Width:     1240,
	Height:    820,
	MinWidth:  720,
	MinHeight: 520,
}
