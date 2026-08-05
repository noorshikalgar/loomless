//go:build darwin

package capture

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// No inner try/catch around the actual window access below: if Accessibility
// permission isn't granted, System Events throws -1728 ("not allowed
// assistive access") and we want that to surface as a real osascript
// failure (caught in runAppleScript, reported with actionable guidance) —
// not get swallowed into a misleadingly empty result. See
// docs/product/07-edge-cases.md on macOS permissions needing to fail loud.
const frontmostScript = `
tell application "System Events"
    set frontApp to first application process whose frontmost is true
    set appName to name of frontApp
    set winTitle to name of front window of frontApp
    set winPos to position of front window of frontApp
    set winSize to size of front window of frontApp
end tell
return appName & tab & winTitle & tab & (item 1 of winPos) & tab & (item 2 of winPos) & tab & (item 1 of winSize) & tab & (item 2 of winSize)
`

const listScript = `
tell application "System Events"
    -- Canary: throws -1728 immediately if Accessibility permission is
    -- missing, instead of letting every per-process try below swallow it
    -- into a silent, misleading "zero windows" result.
    get windows of first application process whose visible is true

    set output to ""
    repeat with proc in (application processes whose visible is true)
        set appName to name of proc
        try
            repeat with w in windows of proc
                set wPos to position of w
                set wSize to size of w
                set output to output & appName & tab & (name of w) & tab & (item 1 of wPos) & tab & (item 2 of wPos) & tab & (item 1 of wSize) & tab & (item 2 of wSize) & linefeed
            end repeat
        end try
    end repeat
end tell
return output
`

// Frontmost returns the active application's front window.
func Frontmost() (WindowInfo, error) {
	out, err := runAppleScript(frontmostScript)
	if err != nil {
		return WindowInfo{}, err
	}
	w, err := parseWindowLine(out)
	if err != nil {
		return WindowInfo{}, fmt.Errorf("no frontmost window (nothing focused, or it exposes no window): %w", err)
	}
	return w, nil
}

// List returns every visible window across running applications, for
// picking a specific one to capture instead of the frontmost.
func List() ([]WindowInfo, error) {
	out, err := runAppleScript(listScript)
	if err != nil {
		return nil, err
	}
	var windows []WindowInfo
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		w, err := parseWindowLine(line)
		if err != nil {
			continue // skip windows System Events couldn't fully describe
		}
		windows = append(windows, w)
	}
	return windows, nil
}

// Screenshot captures the PNG bytes for exactly the given window's bounds
// using the macOS `screencapture` tool's region flag, which takes point
// coordinates and handles Retina backing-scale conversion itself.
func Screenshot(w WindowInfo) ([]byte, error) {
	tmp, err := os.CreateTemp("", "loomless-capture-*.png")
	if err != nil {
		return nil, fmt.Errorf("create temp screenshot file: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	region := fmt.Sprintf("%d,%d,%d,%d", int(w.X), int(w.Y), int(w.W), int(w.H))
	cmd := exec.Command("screencapture", "-x", "-o", "-R", region, tmpPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("screencapture failed (Screen Recording permission may be needed — System Settings > Privacy & Security > Screen Recording): %s: %w", stderr.String(), err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read screenshot: %w", err)
	}
	return data, nil
}

func runAppleScript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("osascript failed (Accessibility permission may be needed — System Settings > Privacy & Security > Accessibility): %s: %w", stderr.String(), err)
	}
	return stdout.String(), nil
}

func parseWindowLine(line string) (WindowInfo, error) {
	fields := strings.Split(strings.TrimRight(line, "\r\n"), "\t")
	if len(fields) != 6 {
		return WindowInfo{}, fmt.Errorf("unexpected window line format: %q", line)
	}
	x, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return WindowInfo{}, err
	}
	y, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return WindowInfo{}, err
	}
	width, err := strconv.ParseFloat(fields[4], 64)
	if err != nil {
		return WindowInfo{}, err
	}
	height, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return WindowInfo{}, err
	}
	if fields[1] == "" || width == 0 || height == 0 {
		return WindowInfo{}, fmt.Errorf("window has no title/bounds (likely no window open)")
	}
	return WindowInfo{App: fields[0], Title: fields[1], X: x, Y: y, W: width, H: height}, nil
}
