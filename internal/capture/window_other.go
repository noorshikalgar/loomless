//go:build !darwin

package capture

import "fmt"

// Phase 0 targets macOS first, per docs/product/09-roadmap.md ("working on
// one OS first") and the platform-order assumption in
// docs/product/11-decisions.md. Linux/Windows capture backends are future
// work, not a Phase 0 gap to silently paper over.

func Frontmost() (WindowInfo, error) {
	return WindowInfo{}, fmt.Errorf("window capture not yet implemented on this platform (macOS only in Phase 0)")
}

func List() ([]WindowInfo, error) {
	return nil, fmt.Errorf("window capture not yet implemented on this platform (macOS only in Phase 0)")
}

func Screenshot(w WindowInfo) ([]byte, error) {
	return nil, fmt.Errorf("window capture not yet implemented on this platform (macOS only in Phase 0)")
}
