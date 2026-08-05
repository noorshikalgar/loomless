//go:build linux

package capture

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Linux backend, X11 only — same shell-out-to-standard-tools approach as
// window_darwin.go (no cgo, no X11 dev headers needed to build), using
// xdotool for the frontmost window and wmctrl for listing. Native Wayland
// sessions don't expose window geometry to these tools at all (per
// docs/product/07-edge-cases.md, Wayland restricts this by design) — an
// XWayland-compatible app may still work, a native Wayland compositor
// won't. Click-capture (the event-tap equivalent) is intentionally not
// implemented here yet — see clickhook_linux.go.
//
// Built and cross-compiled from macOS; not run against a live X11 session
// in this environment. Needs hands-on verification on real Linux hardware
// before this is more than "should work."

// Frontmost returns the active window via `xdotool getactivewindow`.
func Frontmost() (WindowInfo, error) {
	idOut, err := run("xdotool", "getactivewindow")
	if err != nil {
		return WindowInfo{}, fmt.Errorf("resolve active window (is xdotool installed, and is this an X11 session?): %w", err)
	}
	id := strings.TrimSpace(idOut)
	return windowInfoForID(id)
}

// List returns every window wmctrl can see (X11 only, same caveat as
// Frontmost).
func List() ([]WindowInfo, error) {
	out, err := run("wmctrl", "-lxG")
	if err != nil {
		return nil, fmt.Errorf("list windows (is wmctrl installed, and is this an X11 session?): %w", err)
	}
	var windows []WindowInfo
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		w, err := parseWmctrlLine(line)
		if err != nil {
			continue // skip windows wmctrl couldn't fully describe
		}
		windows = append(windows, w)
	}
	return windows, nil
}

// Screenshot captures the PNG bytes for exactly the given window's bounds
// using maim's geometry flag — the X11 equivalent of macOS's
// `screencapture -R`. maim (not the older/heavier ImageMagick `import`)
// because it's purpose-built for headless region capture without
// prompting for a display grab.
func Screenshot(w WindowInfo) ([]byte, error) {
	tmp, err := os.CreateTemp("", "loomless-capture-*.png")
	if err != nil {
		return nil, fmt.Errorf("create temp screenshot file: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	geometry := fmt.Sprintf("%dx%d+%d+%d", int(w.W), int(w.H), int(w.X), int(w.Y))
	if _, err := run("maim", "-g", geometry, tmpPath); err != nil {
		return nil, fmt.Errorf("maim failed (is maim installed?): %w", err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read screenshot: %w", err)
	}
	return data, nil
}

// parseWMClass extracts the class half of an xprop WM_CLASS query, e.g.
// `WM_CLASS(STRING) = "xterm", "XTerm"` -> "XTerm". WM_CLASS is always
// "instance", "Class" (in that order) per the ICCCM spec; the class half
// is the more recognizable "app name" — same choice parseWmctrlLine makes
// for wmctrl's dot-joined form of the same property.
func parseWMClass(out string) string {
	parts := strings.Split(out, `"`)
	var quoted []string
	for i := 1; i < len(parts); i += 2 {
		quoted = append(quoted, parts[i])
	}
	if len(quoted) == 0 {
		return ""
	}
	return quoted[len(quoted)-1]
}

func windowInfoForID(id string) (WindowInfo, error) {
	name, err := run("xdotool", "getwindowname", id)
	if err != nil {
		return WindowInfo{}, fmt.Errorf("get window name: %w", err)
	}
	// xdotool has no "get WM_CLASS" subcommand — confirmed against a real
	// xdotool binary via `xdotool help`, which doesn't list one. An
	// earlier version of this code called a nonexistent
	// "getwindowclassname", caught via container-based X11 testing (see
	// docs/product/11-decisions.md #10) when Frontmost() started failing
	// outright. xprop is the correct tool for this.
	classOut, err := run("xprop", "-id", id, "WM_CLASS")
	if err != nil {
		return WindowInfo{}, fmt.Errorf("get window class: %w", err)
	}
	class := parseWMClass(classOut)
	geomOut, err := run("xdotool", "getwindowgeometry", "--shell", id)
	if err != nil {
		return WindowInfo{}, fmt.Errorf("get window geometry: %w", err)
	}
	x, y, width, height, err := parseShellGeometry(geomOut)
	if err != nil {
		return WindowInfo{}, err
	}
	return WindowInfo{
		App:   strings.TrimSpace(class),
		Title: strings.TrimSpace(name),
		X:     x, Y: y, W: width, H: height,
	}, nil
}

// parseShellGeometry reads xdotool's "--shell" output format:
//
//	WINDOW=12345
//	X=100
//	Y=200
//	WIDTH=800
//	HEIGHT=600
//	SCREEN=0
func parseShellGeometry(out string) (x, y, width, height float64, err error) {
	values := map[string]float64{}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		v, perr := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if perr != nil {
			continue
		}
		values[strings.TrimSpace(parts[0])] = v
	}
	x, xok := values["X"]
	y, yok := values["Y"]
	width, wok := values["WIDTH"]
	height, hok := values["HEIGHT"]
	if !xok || !yok || !wok || !hok {
		return 0, 0, 0, 0, fmt.Errorf("incomplete geometry output: %q", out)
	}
	return x, y, width, height, nil
}

// parseWmctrlLine parses one line of `wmctrl -lxG` output:
//
//	WINDOWID  DESKTOP  X  Y  WIDTH  HEIGHT  WM_CLASS  HOSTNAME  TITLE...
func parseWmctrlLine(line string) (WindowInfo, error) {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return WindowInfo{}, fmt.Errorf("unexpected wmctrl line format: %q", line)
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
	// WM_CLASS is "instance.Class" — the class half is the more
	// recognizable "app name" (e.g. "Firefox" out of "Navigator.Firefox").
	app := fields[6]
	if i := strings.LastIndex(app, "."); i >= 0 && i+1 < len(app) {
		app = app[i+1:]
	}
	title := strings.Join(fields[8:], " ")
	if title == "" || width == 0 || height == 0 {
		return WindowInfo{}, fmt.Errorf("window has no title/bounds")
	}
	return WindowInfo{App: app, Title: title, X: x, Y: y, W: width, H: height}, nil
}

func run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %w", stderr.String(), err)
	}
	return stdout.String(), nil
}
