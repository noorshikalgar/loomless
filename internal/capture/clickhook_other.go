//go:build !darwin && !linux

package capture

import "fmt"

func RunClickHook(onClick func(x, y float64)) error {
	return fmt.Errorf("click capture not implemented on this platform")
}

func StopClickHook() {}
