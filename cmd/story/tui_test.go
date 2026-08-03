package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/noorshikalgar/story/internal/session"
)

func withTestHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func TestNewTUIModelListsSessions(t *testing.T) {
	withTestHome(t)
	if _, _, err := session.Create("alpha"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, _, err := session.Create("beta"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	m, err := newTUIModel()
	if err != nil {
		t.Fatalf("newTUIModel: %v", err)
	}
	if got := len(m.list.Items()); got != 2 {
		t.Fatalf("newTUIModel listed %d sessions, want 2", got)
	}
}

func TestSessionItemTitleShowsRecordingStatus(t *testing.T) {
	recording := sessionItem{s: session.Session{Name: "live"}} // StoppedAt nil: still recording
	if !strings.Contains(recording.Title(), "recording") {
		t.Errorf("Title() for an unstopped session = %q, want it to mention recording", recording.Title())
	}

	stopped := time.Now()
	done := sessionItem{s: session.Session{Name: "done", StoppedAt: &stopped}}
	if strings.Contains(done.Title(), "recording") {
		t.Errorf("Title() for a stopped session = %q, want no recording marker", done.Title())
	}
}

func TestTUIModelExportWritesFile(t *testing.T) {
	withTestHome(t)
	if _, _, err := session.Create("exportme"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	m, err := newTUIModel()
	if err != nil {
		t.Fatalf("newTUIModel: %v", err)
	}
	m.list.Select(0)

	m.export("md")
	if m.statusIsError {
		t.Fatalf("export(md) reported an error status: %s", m.status)
	}
	dir, err := session.Dir("exportme")
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "exportme.md")); err != nil {
		t.Errorf("export(md) didn't produce exportme.md: %v", err)
	}
}

func TestTUIModelRefreshPicksUpNewSessions(t *testing.T) {
	withTestHome(t)
	m, err := newTUIModel()
	if err != nil {
		t.Fatalf("newTUIModel: %v", err)
	}
	if got := len(m.list.Items()); got != 0 {
		t.Fatalf("newTUIModel on empty state listed %d sessions, want 0", got)
	}

	if _, _, err := session.Create("late-arrival"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	m.refresh()
	if got := len(m.list.Items()); got != 1 {
		t.Errorf("after refresh(), list has %d items, want 1", got)
	}
}

// TestTUIModelOpenEditorReusesRunningServer confirms pressing the "open
// editor" action twice on the same session doesn't bind a second HTTP
// server to a second port — it should just reopen the browser tab on the
// already-running one. openBrowser is swapped for a no-op recorder so
// this test doesn't actually pop open a real browser window.
func TestTUIModelOpenEditorReusesRunningServer(t *testing.T) {
	withTestHome(t)
	if _, _, err := session.Create("editme"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var openedURLs []string
	orig := openBrowser
	openBrowser = func(url string) { openedURLs = append(openedURLs, url) }
	defer func() { openBrowser = orig }()

	m, err := newTUIModel()
	if err != nil {
		t.Fatalf("newTUIModel: %v", err)
	}
	m.list.Select(0)

	m.openEditor()
	if m.statusIsError {
		t.Fatalf("openEditor: %s", m.status)
	}
	if len(m.editors) != 1 {
		t.Fatalf("after first openEditor(), len(editors) = %d, want 1", len(m.editors))
	}
	firstURL := m.editors["editme"].url
	firstServer := m.editors["editme"].server

	m.openEditor() // second call on the same session
	if len(m.editors) != 1 {
		t.Errorf("after second openEditor() on the same session, len(editors) = %d, want still 1 (reused)", len(m.editors))
	}
	if m.editors["editme"].server != firstServer {
		t.Errorf("second openEditor() replaced the running server instead of reusing it")
	}
	if len(openedURLs) != 2 || openedURLs[0] != firstURL || openedURLs[1] != firstURL {
		t.Errorf("openBrowser calls = %v, want [%s, %s] (same URL both times)", openedURLs, firstURL, firstURL)
	}

	m.closeAllEditors()
}
