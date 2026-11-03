//go:build windows

package capture

import (
	"fmt"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

// msllHookStruct is MSLLHOOKSTRUCT.
type msllHookStruct struct {
	X, Y      int32
	MouseData uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

var (
	hookThread  atomic.Uint32
	hookStopped atomic.Bool // StopClickHook ran, possibly before the loop started
)

// RunClickHook installs a global low-level mouse hook and blocks pumping
// the thread's message queue (the hook is delivered through it) until
// StopClickHook. The caller must have locked the OS thread, as on macOS.
//
// Like the macOS event tap it only observes: every event is passed on
// unchanged via CallNextHookEx. No permission prompt exists on Windows,
// but a low-level hook can't see clicks in windows of elevated
// ("Run as administrator") processes unless Loomless is elevated too.
func RunClickHook(onClick func(x, y float64)) error {
	const whMouseLL, wmLButtonDown = 14, 0x0201

	// Windows silently uninstalls a low-level hook whose callback takes
	// longer than ~300ms-1s, and a click's screenshot + session write can
	// take that long. The callback only queues the point; a separate
	// goroutine does the slow work.
	clicks := make(chan [2]float64, 64)
	go func() {
		for c := range clicks {
			onClick(c[0], c[1])
		}
	}()
	defer close(clicks)

	cb := windows.NewCallback(func(code int32, wParam uintptr, m *msllHookStruct) uintptr {
		if code >= 0 && wParam == wmLButtonDown {
			select {
			case clicks <- [2]float64{float64(m.X), float64(m.Y)}:
			default: // backlog full: drop rather than stall the user's mouse
			}
		}
		r, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, uintptr(unsafe.Pointer(m)))
		return r
	})

	hook, _, err := procSetWindowsHookExW.Call(whMouseLL, cb, 0, 0)
	if hook == 0 {
		return fmt.Errorf("failed to start click listener: SetWindowsHookEx: %v", err)
	}
	defer procUnhookWindowsHookEx.Call(hook)

	hookThread.Store(windows.GetCurrentThreadId())
	defer hookThread.Store(0)
	if hookStopped.Load() {
		return nil
	}

	var msg [6]uint64 // MSG (48 bytes); contents unused, GetMessage just needs somewhere to write
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg[0])), 0, 0, 0)
		if int32(r) <= 0 { // 0 = WM_QUIT, -1 = error
			return nil
		}
	}
}

// StopClickHook ends RunClickHook's message loop. Safe from any
// goroutine.
func StopClickHook() {
	const wmQuit = 0x0012
	hookStopped.Store(true)
	if id := hookThread.Load(); id != 0 {
		procPostThreadMessageW.Call(uintptr(id), wmQuit, 0, 0)
	}
}
