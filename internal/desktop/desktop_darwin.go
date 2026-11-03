//go:build darwin

package desktop

/*
#cgo LDFLAGS: -framework Cocoa -framework WebKit
void loomlessInstallHooks(void *window);
*/
import "C"

import webview "github.com/webview/webview_go"

// Run opens a native window showing url and blocks until the user
// closes it (or quits from the menu). It must be called from the main
// goroutine: Cocoa requires every UI call on the process's main thread,
// and webview_go's package init locks main.main to it.
func Run(url string, o Options) error {
	w := webview.New(false)
	if w == nil {
		return ErrUnavailable
	}
	defer w.Destroy()

	w.SetTitle(o.Title)
	w.SetSize(o.MinWidth, o.MinHeight, webview.HintMin)
	w.SetSize(o.Width, o.Height, webview.HintNone)
	C.loomlessInstallHooks(w.Window())
	// Lets page JS tell "native window" from "browser tab" — e.g. the
	// dashboard opens the editor in this same window rather than trying
	// (and failing) to pop a new one.
	w.Init(`window.LOOMLESS_DESKTOP = true;`)
	w.Navigate(url)
	w.Run()
	return nil
}
