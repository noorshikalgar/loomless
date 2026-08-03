//go:build !darwin

package capture

import "fmt"

func RunClickHook(onClick func(x, y float64)) error {
	return fmt.Errorf("click capture not yet implemented on this platform (macOS only in Phase 0/1)")
}

func StopClickHook() {}
