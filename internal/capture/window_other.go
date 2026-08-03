//go:build !darwin && !linux

package capture

import "fmt"

// Windows (and anything else besides macOS/Linux) has no capture backend
// yet — window_darwin.go covers macOS, window_linux.go covers X11 Linux.
// Not a gap to silently paper over: every call here fails loudly with a
// clear "not implemented" rather than pretending to work.

func Frontmost() (WindowInfo, error) {
	return WindowInfo{}, fmt.Errorf("window capture not yet implemented on this platform (macOS only in Phase 0)")
}

func List() ([]WindowInfo, error) {
	return nil, fmt.Errorf("window capture not yet implemented on this platform (macOS only in Phase 0)")
}

func Screenshot(w WindowInfo) ([]byte, error) {
	return nil, fmt.Errorf("window capture not yet implemented on this platform (macOS only in Phase 0)")
}
