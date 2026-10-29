//go:build !unix

package session

// lockDir is a no-op where flock isn't available (Windows, which doesn't
// build yet anyway — see README's platform table). Atomic writes in save
// still prevent torn reads there; only concurrent read-modify-write
// protection is missing until a LockFileEx implementation lands.
func lockDir(dir string) (func(), error) {
	return func() {}, nil
}
