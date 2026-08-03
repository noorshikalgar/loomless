//go:build linux

package capture

import "fmt"

// Deliberately not implemented yet — not an oversight. Unlike macOS's
// single CGEventTap API, X11 global click observation needs either the
// XTest/RECORD extension (nontrivial without cgo + X11 dev headers, which
// this package avoids elsewhere by shelling out to standard tools instead)
// or a privileged /dev/input read, and native Wayland compositors restrict
// global input observation by design (per docs/product/07-edge-cases.md) —
// there's no equivalent of "shell out to a standard CLI tool" here the way
// window_linux.go does for window listing/geometry/screenshots. Manual
// capture (story step / capture_step, story run / run_command) works
// today on Linux; only automatic click-capture is the gap.
func RunClickHook(onClick func(x, y float64)) error {
	return fmt.Errorf("automatic click capture isn't implemented on Linux yet (X11 needs XTest/RECORD, Wayland restricts this further) — use 'story step'/capture_step and 'story run'/run_command for manual capture instead")
}

func StopClickHook() {}
