//go:build darwin

package capture

/*
#cgo LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>

extern void goMouseDown(double x, double y);

static CGEventRef storyEventTapCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *refcon) {
    if (type == kCGEventLeftMouseDown) {
        CGPoint loc = CGEventGetLocation(event);
        goMouseDown(loc.x, loc.y);
    }
    return event;
}

static CFMachPortRef storyTap = NULL;
// Stop must target the exact CFRunLoop that's running the tap, captured at
// start time — CFRunLoopGetCurrent() called later from a signal handler
// (which Go may schedule on a different OS thread) returns THAT thread's
// run loop, not the one actually blocked in CFRunLoopRun, so a stop call
// against "current" silently no-ops and the daemon never exits.
static CFRunLoopRef storyRunLoop = NULL;

// storyStartEventTap installs a listen-only tap: it observes clicks but
// never consumes or modifies them, so it can't interfere with normal use.
static int storyStartEventTap(void) {
    CGEventMask mask = CGEventMaskBit(kCGEventLeftMouseDown);
    storyTap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, kCGEventTapOptionListenOnly, mask, storyEventTapCallback, NULL);
    if (storyTap == NULL) {
        return 0;
    }
    storyRunLoop = CFRunLoopGetCurrent();
    CFRunLoopSourceRef source = CFMachPortCreateRunLoopSource(NULL, storyTap, 0);
    CFRunLoopAddSource(storyRunLoop, source, kCFRunLoopCommonModes);
    CGEventTapEnable(storyTap, true);
    return 1;
}

static void storyStopEventTap(void) {
    if (storyTap != NULL) {
        CGEventTapEnable(storyTap, false);
    }
    if (storyRunLoop != NULL) {
        CFRunLoopStop(storyRunLoop);
    }
}
*/
import "C"

import (
	"fmt"
	"sync"
)

var (
	clickMu       sync.Mutex
	clickCallback func(x, y float64)
)

//export goMouseDown
func goMouseDown(x, y C.double) {
	clickMu.Lock()
	cb := clickCallback
	clickMu.Unlock()
	if cb != nil {
		cb(float64(x), float64(y))
	}
}

// RunClickHook installs a global left-mouse-down listener and blocks,
// running a CFRunLoop, until StopClickHook is called from another
// goroutine. It must run on a dedicated goroutine with the OS thread
// locked (runtime.LockOSThread) — this is meant to be the click-daemon
// process's whole job, not something called from a short-lived command.
//
// Requires macOS Accessibility permission for the calling process, same
// requirement window.go's AppleScript calls already have — this doesn't
// add a new permission category.
func RunClickHook(onClick func(x, y float64)) error {
	clickMu.Lock()
	clickCallback = onClick
	clickMu.Unlock()

	if C.storyStartEventTap() == 0 {
		return fmt.Errorf("failed to start click listener (Accessibility permission required — System Settings > Privacy & Security > Accessibility)")
	}
	C.CFRunLoopRun()
	return nil
}

// StopClickHook halts the run loop started by RunClickHook. Safe to call
// from a signal handler on a different goroutine.
func StopClickHook() {
	C.storyStopEventTap()
}
