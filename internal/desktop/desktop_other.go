//go:build !darwin && !windows

package desktop

// Run has no native window on this platform yet; the caller opens the
// system browser instead.
func Run(url string, o Options) error {
	return ErrUnavailable
}
