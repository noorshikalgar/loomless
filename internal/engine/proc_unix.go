//go:build !windows

package engine

import (
	"os"
	"os/signal"
	"syscall"
)

// detachedAttr starts a background process in its own session (Setsid)
// so it survives the command that launched it exiting.
func detachedAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// requestStop asks a background process started by startBackgroundProcess
// to shut down cleanly.
func requestStop(proc *os.Process, pidPath string) {
	_ = proc.Signal(syscall.SIGTERM)
}

// stopRequested is closed when this background process is asked to stop
// (requestStop from another process, or Ctrl+C when run by hand).
func stopRequested(pidPath string) <-chan struct{} {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	done := make(chan struct{})
	go func() {
		<-sigCh
		close(done)
	}()
	return done
}

func clearStopRequest(pidPath string) {}
