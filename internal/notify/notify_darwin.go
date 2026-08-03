//go:build darwin

package notify

import "os/exec"

// Send shows a native macOS notification banner via osascript. Best
// effort: notifications can be silently suppressed by the user's Do Not
// Disturb / notification settings, which isn't something worth surfacing
// as an error — this is a soft nudge, not a critical alert.
func Send(title, message string) error {
	script := `display notification "` + escape(message) + `" with title "` + escape(title) + `"`
	return exec.Command("osascript", "-e", script).Run()
}

func escape(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '"' || s[i] == '\\' {
			out = append(out, '\\')
		}
		out = append(out, s[i])
	}
	return string(out)
}
