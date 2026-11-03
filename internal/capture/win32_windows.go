//go:build windows

package capture

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Win32 entry points x/sys/windows doesn't wrap. Lazy-loaded, so a
// missing optional one (dwmapi on a stripped-down Server Core) only fails
// the call that needs it.
var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	gdi32  = windows.NewLazySystemDLL("gdi32.dll")
	dwmapi = windows.NewLazySystemDLL("dwmapi.dll")

	procGetWindowRect                 = user32.NewProc("GetWindowRect")
	procGetWindowTextW                = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW          = user32.NewProc("GetWindowTextLengthW")
	procGetWindowLongW                = user32.NewProc("GetWindowLongW")
	procGetDC                         = user32.NewProc("GetDC")
	procReleaseDC                     = user32.NewProc("ReleaseDC")
	procSetWindowsHookExW             = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx                = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx           = user32.NewProc("UnhookWindowsHookEx")
	procGetMessageW                   = user32.NewProc("GetMessageW")
	procPostThreadMessageW            = user32.NewProc("PostThreadMessageW")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")

	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procGetDIBits              = gdi32.NewProc("GetDIBits")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procDeleteDC               = gdi32.NewProc("DeleteDC")

	procDwmGetWindowAttribute = dwmapi.NewProc("DwmGetWindowAttribute")
)

type rect struct{ Left, Top, Right, Bottom int32 }

// bitmapInfoHeader is BITMAPINFOHEADER.
type bitmapInfoHeader struct {
	Size          uint32
	Width, Height int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

func init() {
	// Per-monitor DPI awareness, so the click hook's coordinates, window
	// rectangles, and BitBlt all speak the same physical pixels. Without
	// it, on a 150% display Windows hands this process scaled-down
	// "virtualized" coordinates and screenshots come out the wrong size
	// and offset. Falls back to system-DPI awareness on Windows < 10 1703.
	const perMonitorAwareV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = (HANDLE)-4
	if procSetProcessDpiAwarenessContext.Find() == nil {
		if r, _, _ := procSetProcessDpiAwarenessContext.Call(perMonitorAwareV2); r != 0 {
			return
		}
	}
	procSetProcessDPIAware.Call()
}

func windowText(hwnd windows.HWND) string {
	n, _, _ := procGetWindowTextLengthW.Call(uintptr(hwnd))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
}

// windowBounds prefers DWM's extended frame bounds: GetWindowRect on
// Windows 10+ includes an invisible ~7px resize border on three sides,
// which would put a strip of whatever's behind the window into every
// screenshot.
func windowBounds(hwnd windows.HWND) (rect, bool) {
	const dwmwaExtendedFrameBounds = 9
	var r rect
	if procDwmGetWindowAttribute.Find() == nil {
		hr, _, _ := procDwmGetWindowAttribute.Call(uintptr(hwnd), dwmwaExtendedFrameBounds,
			uintptr(unsafe.Pointer(&r)), unsafe.Sizeof(r))
		if hr == 0 {
			return r, true
		}
	}
	ok, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	return r, ok != 0
}

// isCloaked reports windows DWM hides without un-showing them: UWP apps
// suspended in the background, windows on other virtual desktops.
// IsWindowVisible says yes to those, but nobody can see them.
func isCloaked(hwnd windows.HWND) bool {
	const dwmwaCloaked = 14
	if procDwmGetWindowAttribute.Find() != nil {
		return false
	}
	var cloaked uint32
	hr, _, _ := procDwmGetWindowAttribute.Call(uintptr(hwnd), dwmwaCloaked,
		uintptr(unsafe.Pointer(&cloaked)), unsafe.Sizeof(cloaked))
	return hr == 0 && cloaked != 0
}
