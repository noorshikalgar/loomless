//go:build windows

package capture

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Coordinates on Windows are physical pixels (the process is per-monitor
// DPI aware, see win32_windows.go), so a WindowInfo's W/H equal its
// screenshot's pixel size and the engine's point→pixel scale is 1.

// Frontmost returns the foreground window.
func Frontmost() (WindowInfo, error) {
	hwnd := windows.GetForegroundWindow()
	if hwnd == 0 {
		return WindowInfo{}, fmt.Errorf("no foreground window (the desktop or lock screen has focus)")
	}
	return describe(hwnd)
}

// List returns every top-level window a person could see and pick.
func List() ([]WindowInfo, error) {
	listMu.Lock()
	defer listMu.Unlock()
	listOut = nil
	if err := windows.EnumWindows(listCallback, nil); err != nil {
		return nil, fmt.Errorf("enumerate windows: %w", err)
	}
	return listOut, nil
}

// One callback for the process's lifetime: Go can only ever create a
// couple of thousand Windows callbacks and never frees them, so List
// mustn't make a new one per call.
var (
	listMu       sync.Mutex
	listOut      []WindowInfo
	listCallback = windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		if !windows.IsWindowVisible(hwnd) || isCloaked(hwnd) || isToolWindow(hwnd) || windowText(hwnd) == "" {
			return 1
		}
		if w, err := describe(hwnd); err == nil {
			listOut = append(listOut, w)
		}
		return 1 // keep enumerating
	})
)

func isToolWindow(hwnd windows.HWND) bool {
	const gwlExStyle = ^uintptr(19) // GWL_EXSTYLE = -20
	const wsExToolWindow = 0x80
	style, _, _ := procGetWindowLongW.Call(uintptr(hwnd), gwlExStyle)
	return style&wsExToolWindow != 0
}

func describe(hwnd windows.HWND) (WindowInfo, error) {
	r, ok := windowBounds(hwnd)
	if !ok || r.Right <= r.Left || r.Bottom <= r.Top {
		return WindowInfo{}, fmt.Errorf("window has no on-screen bounds (minimized?)")
	}
	app := appName(hwnd)
	title := windowText(hwnd)
	if title == "" {
		title = app
	}
	return WindowInfo{
		App:   app,
		Title: title,
		X:     float64(r.Left),
		Y:     float64(r.Top),
		W:     float64(r.Right - r.Left),
		H:     float64(r.Bottom - r.Top),
	}, nil
}

// appName is the owning program's friendly name ("Google Chrome") from
// its version resource, else its file name without ".exe" ("chrome").
func appName(hwnd windows.HWND) string {
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &pid); err != nil || pid == 0 {
		return ""
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return ""
	}
	path := windows.UTF16ToString(buf[:n])
	if desc := fileDescription(path); desc != "" {
		return desc
	}
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func fileDescription(path string) string {
	size, err := windows.GetFileVersionInfoSize(path, nil)
	if err != nil || size == 0 {
		return ""
	}
	info := make([]byte, size)
	if err := windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&info[0])); err != nil {
		return ""
	}
	// The description lives under the first language/codepage pair the
	// file declares.
	var trans *[2]uint16
	var tlen uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&info[0]), `\VarFileInfo\Translation`,
		unsafe.Pointer(&trans), &tlen); err != nil || tlen < 4 {
		return ""
	}
	key := fmt.Sprintf(`\StringFileInfo\%04x%04x\FileDescription`, trans[0], trans[1])
	var desc *uint16
	var dlen uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&info[0]), key, unsafe.Pointer(&desc), &dlen); err != nil || dlen == 0 {
		return ""
	}
	return strings.TrimSpace(windows.UTF16PtrToString(desc))
}

// Screenshot copies the window's rectangle off the screen, like the
// macOS backend's `screencapture -R`: what's visible there, which is the
// window itself since it was just clicked and is in front.
func Screenshot(w WindowInfo) ([]byte, error) {
	x, y, width, height := int32(w.X), int32(w.Y), int32(w.W), int32(w.H)
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("window has no area to capture")
	}

	screen, _, _ := procGetDC.Call(0)
	if screen == 0 {
		return nil, fmt.Errorf("GetDC failed")
	}
	defer procReleaseDC.Call(0, screen)
	mem, _, _ := procCreateCompatibleDC.Call(screen)
	if mem == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(mem)
	bmp, _, _ := procCreateCompatibleBitmap.Call(screen, uintptr(width), uintptr(height))
	if bmp == 0 {
		return nil, fmt.Errorf("CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(bmp)
	old, _, _ := procSelectObject.Call(mem, bmp)
	defer procSelectObject.Call(mem, old)

	const srcCopy, captureBlt = 0x00CC0020, 0x40000000 // CAPTUREBLT: include layered windows (menus, tooltips)
	if ok, _, err := procBitBlt.Call(mem, 0, 0, uintptr(width), uintptr(height),
		screen, uintptr(x), uintptr(y), srcCopy|captureBlt); ok == 0 {
		return nil, fmt.Errorf("BitBlt failed: %v", err)
	}

	hdr := bitmapInfoHeader{
		Size:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:    width,
		Height:   -height, // negative: top-down rows, same order as image.RGBA
		Planes:   1,
		BitCount: 32,
	}
	pix := make([]byte, int(width)*int(height)*4)
	const dibRGBColors = 0
	if n, _, _ := procGetDIBits.Call(mem, bmp, 0, uintptr(height),
		uintptr(unsafe.Pointer(&pix[0])), uintptr(unsafe.Pointer(&hdr)), dibRGBColors); n == 0 {
		return nil, fmt.Errorf("GetDIBits failed")
	}

	img := &image.RGBA{Pix: pix, Stride: int(width) * 4, Rect: image.Rect(0, 0, int(width), int(height))}
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+2], pix[i+3] = pix[i+2], pix[i], 0xff // BGRX → RGBA
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode screenshot: %w", err)
	}
	return buf.Bytes(), nil
}
