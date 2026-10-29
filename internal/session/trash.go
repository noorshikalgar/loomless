package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// trashSep joins a session's name and its trash timestamp in a trashed
// directory's name ("checkout-bug__20261029-123150"). A double
// underscore rather than one, so names that already contain an
// underscore still split back unambiguously at the LAST separator.
const trashSep = "__"

func trashRoot() (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "trash"), nil
}

// Trash moves a session out of ~/.loomless/sessions into
// ~/.loomless/trash instead of deleting it, and returns the ID Restore
// needs to bring it back. "Delete" in a tool used by non-technical
// people should be undoable — one misclick shouldn't destroy a
// half-hour bug-repro recording — so nothing here removes any file.
//
// A session that's still recording is refused, same as Rename: its
// click daemon writes into the session directory by path, and moving
// the directory out from under it would fail every later capture.
func Trash(name string) (string, error) {
	s, dir, err := Load(name)
	if err != nil {
		return "", err
	}
	if s.StoppedAt == nil {
		return "", fmt.Errorf("session %q is still recording — stop it before deleting", name)
	}
	troot, err := trashRoot()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(troot, 0o755); err != nil {
		return "", fmt.Errorf("create trash dir: %w", err)
	}
	id := name + trashSep + time.Now().Format("20060102-150405.000")
	if err := os.Rename(dir, filepath.Join(troot, id)); err != nil {
		return "", fmt.Errorf("move session to trash: %w", err)
	}
	// Don't leave the "current session" pointer aimed at a directory
	// that no longer exists.
	_ = clearCurrentIfNamed(name)
	return id, nil
}

// Restore moves a trashed session (by the ID Trash returned) back into
// ~/.loomless/sessions under its original name. Fails rather than
// overwriting if a session with that name was created in the meantime.
func Restore(id string) (string, error) {
	if err := ValidateName(id); err != nil {
		return "", err
	}
	i := strings.LastIndex(id, trashSep)
	if i <= 0 {
		return "", fmt.Errorf("invalid trash id %q", id)
	}
	name := id[:i]
	dest, err := Dir(name)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dest); err == nil {
		return "", fmt.Errorf("a session named %q already exists — rename it first", name)
	}
	troot, err := trashRoot()
	if err != nil {
		return "", err
	}
	src := filepath.Join(troot, id)
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("nothing in trash with id %q", id)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(src, dest); err != nil {
		return "", fmt.Errorf("restore session: %w", err)
	}
	return name, nil
}

func clearCurrentIfNamed(name string) error {
	p, err := currentPath()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil // no current pointer at all
	}
	if strings.TrimSpace(string(b)) == name {
		return clearCurrent()
	}
	return nil
}
