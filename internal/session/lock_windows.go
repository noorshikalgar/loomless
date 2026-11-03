//go:build windows

package session

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

// lockDir is lock_unix.go's lockDir via LockFileEx. Like flock, the lock
// belongs to the open handle and is released when the process exits, so
// a crashed daemon can't wedge a session.
func lockDir(dir string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	ol := new(windows.Overlapped)
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, ol); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol)
		f.Close()
	}, nil
}

// renameReplacing retries os.Rename briefly: on Windows, replacing a file
// fails with "Access is denied" while another process (the dashboard
// polling session.json, say) has it open for reading. Readers hold it for
// milliseconds, so a short retry rides that out.
func renameReplacing(from, to string) error {
	var err error
	for i := 0; i < 50; i++ {
		if err = os.Rename(from, to); err == nil || !errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}
	return err
}
