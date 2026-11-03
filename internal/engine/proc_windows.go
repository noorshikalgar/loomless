//go:build windows

package engine

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// detachedAttr starts a background process with no console window and
// outside the launcher's Ctrl+C group, so it survives the command (or
// app window) that launched it closing.
func detachedAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
		HideWindow:    true,
	}
}

// Windows can't deliver SIGTERM to another process, so a stop request is
// a marker file next to the PID file that the background process polls
// for and deletes as its acknowledgement.
func stopFilePath(pidPath string) string { return pidPath + ".stop" }

func requestStop(proc *os.Process, pidPath string) {
	stop := stopFilePath(pidPath)
	if err := os.WriteFile(stop, nil, 0o644); err == nil {
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
			if _, err := os.Stat(stop); os.IsNotExist(err) {
				return // acknowledged; it's on its way out
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	// Not acknowledged (wedged, or already gone): a daemon must never keep
	// recording clicks after the user pressed Stop.
	_ = proc.Kill()
	_ = os.Remove(stop)
}

func stopRequested(pidPath string) <-chan struct{} {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-sigCh:
				close(done)
				return
			case <-t.C:
				if _, err := os.Stat(stopFilePath(pidPath)); err == nil {
					_ = os.Remove(stopFilePath(pidPath))
					close(done)
					return
				}
			}
		}
	}()
	return done
}

// clearStopRequest removes a stale stop marker (from a daemon that was
// killed before it could delete it) so a new daemon doesn't exit the
// moment it starts.
func clearStopRequest(pidPath string) {
	_ = os.Remove(stopFilePath(pidPath))
}
