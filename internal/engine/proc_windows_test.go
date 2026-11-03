//go:build windows

package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The stop handshake that replaces SIGTERM on Windows: requestStop
// drops a marker, the background process's stopRequested sees it,
// deletes it as an acknowledgement, and requestStop returns without
// killing anything. (If the handshake were broken, requestStop would
// fall through to Kill — here, the test process itself.)
func TestStopFileHandshake(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "daemon.pid")
	stop := stopRequested(pidPath)
	self, _ := os.FindProcess(os.Getpid())

	start := time.Now()
	requestStop(self, pidPath)
	select {
	case <-stop:
	case <-time.After(time.Second):
		t.Fatal("stopRequested never fired")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("requestStop waited for its timeout instead of the acknowledgement")
	}
	if _, err := os.Stat(stopFilePath(pidPath)); !os.IsNotExist(err) {
		t.Fatal("stop marker left behind")
	}
}
