//go:build !darwin && !linux

package notify

// No notification backend on this platform yet — the idle-timeout
// watchdog still runs and logs, it just can't nudge the user visibly.
func Send(title, message string) error {
	return nil
}
