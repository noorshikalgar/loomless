//go:build windows

package desktop

import (
	"os"
	"path/filepath"
	"runtime"

	webview2 "github.com/jchv/go-webview2"
)

func init() {
	// WebView2's window and message loop must stay on the thread that
	// created them; pin main.main to the process's main thread, as
	// webview_go's own init does on macOS.
	runtime.LockOSThread()
}

// Run opens a native window (Edge WebView2) showing url and blocks until
// it's closed. Must be called from the main goroutine. Windows 11 ships
// the WebView2 runtime; on a Windows 10 machine without it this returns
// ErrUnavailable and the caller falls back to the browser.
//
// Unlike WKWebView, WebView2 handles alert/confirm/prompt, file pickers
// and the clipboard itself, so nothing like hooks_darwin.m is needed.
func Run(url string, o Options) error {
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		// The default profile folder sits next to the .exe, which isn't
		// writable when installed under Program Files.
		DataPath:  dataPath(),
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  o.Title,
			Width:  uint(o.Width),
			Height: uint(o.Height),
			IconId: 1, // the app icon packaging/windows embeds as resource #1
			Center: true,
		},
	})
	if w == nil {
		return ErrUnavailable
	}
	defer w.Destroy()
	w.SetSize(o.MinWidth, o.MinHeight, webview2.HintMin)
	w.Init(`window.LOOMLESS_DESKTOP = true;`)
	w.Navigate(url)
	w.Run()
	return nil
}

func dataPath() string {
	dir, err := os.UserCacheDir() // %LOCALAPPDATA%
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Loomless", "WebView2")
}
