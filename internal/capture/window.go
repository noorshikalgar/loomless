// Package capture resolves "which window is this" and turns that into a
// screenshot scoped to just that window, per docs/product/11-decisions.md
// decision #5: captures are window-specific, never full-screen.
package capture

// WindowInfo identifies a window and its on-screen bounds, in the screen's
// point coordinate system (not backing pixels — screencapture's -R flag
// expects points and handles Retina scaling itself).
type WindowInfo struct {
	App   string
	Title string
	X, Y  float64
	W, H  float64
}
