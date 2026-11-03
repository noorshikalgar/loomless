//go:build windows

package capture

import (
	"runtime"
	"testing"
	"time"
)

func TestScreenshotProducesPNGOfTheRequestedSize(t *testing.T) {
	png, err := Screenshot(WindowInfo{X: 0, Y: 0, W: 40, H: 30})
	if err != nil {
		t.Fatal(err)
	}
	w, h, err := PixelSize(png)
	if err != nil {
		t.Fatal(err)
	}
	if w != 40 || h != 30 {
		t.Fatalf("screenshot is %dx%d, want 40x30", w, h)
	}
}

func TestListDoesNotFailAndRepeatsCleanly(t *testing.T) {
	// Called many times to prove the shared EnumWindows callback doesn't
	// leak a new Windows callback per call (Go caps those at ~2000).
	for i := 0; i < 3000; i++ {
		if _, err := List(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestClickHookStopsWhenAsked(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		done <- RunClickHook(func(x, y float64) {})
	}()
	time.Sleep(200 * time.Millisecond)
	StopClickHook()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunClickHook did not return after StopClickHook")
	}
}
