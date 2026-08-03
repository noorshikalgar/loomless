//go:build linux

package notify

import "os/exec"

// Send shows a desktop notification via notify-send (libnotify) — best
// effort, same reasoning as the macOS backend: no notification daemon
// running (common on minimal/headless setups) just means the nudge is
// silently missed, not an error worth surfacing.
func Send(title, message string) error {
	return exec.Command("notify-send", title, message).Run()
}
