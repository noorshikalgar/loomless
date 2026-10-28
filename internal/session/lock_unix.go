//go:build unix

package session

import (
	"os"
	"path/filepath"
	"syscall"
)

// lockDir takes an exclusive, cross-process advisory lock on a session
// directory and returns the function that releases it. The click daemon,
// idle watchdog, CLI, editor, and dashboard are separate processes that
// all read-modify-write the same session.json; without this, two of them
// loading the same version and saving back would silently drop one
// side's change (a captured click, an annotation, a caption edit).
//
// flock locks belong to the open file description, so the lock is
// released automatically if the process dies while holding it — a
// crashed daemon can never wedge a session permanently.
func lockDir(dir string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { f.Close() }, nil // closing the fd releases the flock
}
