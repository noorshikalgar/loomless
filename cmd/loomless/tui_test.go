package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/noorshikalgar/loomless/internal/session"
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

// TestTUIModelStopEditorClosesServer confirms 'x' actually closes the
// running server rather than just forgetting about it — the running
// process would otherwise leak until the whole TUI quit, which is
// exactly the gap being closed here.
func TestTUIModelStopEditorClosesServer(t *testing.T) {
	withTestHome(t)
	if _, _, err := session.Create("editme"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	orig := openBrowser
	openBrowser = func(string) {}
	defer func() { openBrowser = orig }()

	m, err := newTUIModel()
	if err != nil {
		t.Fatalf("newTUIModel: %v", err)
	}
	m.list.Select(0)
	m.openEditor()
	if len(m.editors) != 1 {
		t.Fatalf("openEditor: len(editors) = %d, want 1", len(m.editors))
	}
	url := m.editors["editme"].url

	if _, err := http.Get(url + "api/session"); err != nil {
		t.Fatalf("editor server not reachable before stopEditor: %v", err)
	}

	m.stopEditor()
	if m.statusIsError {
		t.Fatalf("stopEditor: %s", m.status)
	}
	if len(m.editors) != 0 {
		t.Errorf("after stopEditor(), len(editors) = %d, want 0", len(m.editors))
	}
	if _, err := http.Get(url + "api/session"); err == nil {
		t.Error("editor server still reachable after stopEditor()")
	}
}

func TestTUIModelStopEditorWithNothingRunningSetsErrorStatus(t *testing.T) {
	withTestHome(t)
	if _, _, err := session.Create("noeditor"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	m, err := newTUIModel()
	if err != nil {
		t.Fatalf("newTUIModel: %v", err)
	}
	m.list.Select(0)

	m.stopEditor()
	if !m.statusIsError {
		t.Errorf("stopEditor() with nothing running: statusIsError = false, want true (status: %q)", m.status)
	}
}

// TestTUIModelNewSessionFlow drives the exact key sequence a user would:
// 'n' opens the prompt, typed characters land in the text field, enter
// submits. Confirms the session actually exists on disk afterward, not
// just that no error was returned. startRecording is swapped for a fake
// that creates the session the same way engine.StartRecording's first
// step does (session.Create) without actually forking the real click
// daemon — engine.StartRecording relaunches the current executable as a
// background process, which in a `go test` binary is the compiled test
// binary itself, not the loomless CLI; spawning that for real here would be
// unpredictable (wrong argv for the test binary's own flag parsing) and
// exactly the kind of real-subprocess side effect a unit test shouldn't
// have. The actual background-process plumbing is covered separately by
// decision #15's process-inspection verification, not re-tested here.
func TestTUIModelNewSessionFlow(t *testing.T) {
	withTestHome(t)
	orig := startRecording
	startRecording = func(name string, highlightClicks, autoCaption bool) (string, bool, string, error) {
		_, dir, err := session.Create(name)
		return dir, true, "", err
	}
	defer func() { startRecording = orig }()

	m, err := newTUIModel()
	if err != nil {
		t.Fatalf("newTUIModel: %v", err)
	}

	m.startInput(inputNewSession)
	if m.inputPurpose != inputNewSession {
		t.Fatalf("startInput(inputNewSession): inputPurpose = %v, want inputNewSession", m.inputPurpose)
	}
	m.textInput.SetValue("brand-new-session")

	m.submitInput()
	if m.statusIsError {
		t.Fatalf("submitInput (new session): %s", m.status)
	}
	if m.inputPurpose != inputNone {
		t.Errorf("after submitInput, inputPurpose = %v, want inputNone", m.inputPurpose)
	}

	if _, _, err := session.Load("brand-new-session"); err != nil {
		t.Errorf("brand-new-session was not created on disk: %v", err)
	}
	found := false
	for _, item := range m.list.Items() {
		if it, ok := item.(sessionItem); ok && it.s.Name == "brand-new-session" {
			found = true
		}
	}
	if !found {
		t.Error("brand-new-session doesn't appear in the list after creation")
	}
}

// TestTUIModelNewSessionWatchingFalseShowsWarning confirms the "started
// but click-watching isn't available" path (e.g. no Accessibility
// permission) surfaces as a status the user actually sees, rather than
// silently reporting success.
func TestTUIModelNewSessionWatchingFalseShowsWarning(t *testing.T) {
	withTestHome(t)
	orig := startRecording
	startRecording = func(name string, highlightClicks, autoCaption bool) (string, bool, string, error) {
		_, dir, err := session.Create(name)
		return dir, false, "no Accessibility permission", err
	}
	defer func() { startRecording = orig }()

	m, err := newTUIModel()
	if err != nil {
		t.Fatalf("newTUIModel: %v", err)
	}
	m.startInput(inputNewSession)
	m.textInput.SetValue("manual-only")

	m.submitInput()
	if !m.statusIsError {
		t.Error("submitInput with watching=false: statusIsError = false, want true (should surface the warning)")
	}
	if !strings.Contains(m.status, "no Accessibility permission") {
		t.Errorf("status = %q, want it to include the warning text", m.status)
	}
	if _, _, err := session.Load("manual-only"); err != nil {
		t.Errorf("manual-only session should still have been created despite watching=false: %v", err)
	}
}

func TestTUIModelNewSessionEmptyNameRejected(t *testing.T) {
	withTestHome(t)
	m, err := newTUIModel()
	if err != nil {
		t.Fatalf("newTUIModel: %v", err)
	}
	m.startInput(inputNewSession)
	m.textInput.SetValue("   ")

	m.submitInput()
	if !m.statusIsError {
		t.Error("submitInput with a blank name: statusIsError = false, want true")
	}
}

// TestTUIModelRenameFlow confirms the prompt pre-fills with the current
// name (so a user can position the cursor and edit rather than retyping
// the whole thing) and that submitting actually renames on disk.
func TestTUIModelRenameFlow(t *testing.T) {
	withTestHome(t)
	s, dir, err := session.Create("old-name")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Stop(dir); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	m, err := newTUIModel()
	if err != nil {
		t.Fatalf("newTUIModel: %v", err)
	}
	m.list.Select(0)

	m.startInput(inputRenameSession)
	if m.inputPurpose != inputRenameSession {
		t.Fatalf("startInput(inputRenameSession): inputPurpose = %v, want inputRenameSession", m.inputPurpose)
	}
	if got := m.textInput.Value(); got != "old-name" {
		t.Fatalf("rename prompt pre-filled with %q, want %q", got, "old-name")
	}
	m.textInput.SetValue("new-name")

	m.submitInput()
	if m.statusIsError {
		t.Fatalf("submitInput (rename): %s", m.status)
	}

	if _, _, err := session.Load("old-name"); err == nil {
		t.Error("old-name still exists on disk after rename")
	}
	if _, _, err := session.Load("new-name"); err != nil {
		t.Errorf("new-name doesn't exist on disk after rename: %v", err)
	}
}

func TestTUIModelStopRecording(t *testing.T) {
	withTestHome(t)
	if _, _, err := session.Create("live-one"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	m, err := newTUIModel()
	if err != nil {
		t.Fatalf("newTUIModel: %v", err)
	}
	m.list.Select(0)

	m.stopRecording()
	if m.statusIsError {
		t.Fatalf("stopRecording: %s", m.status)
	}

	s, _, err := session.Load("live-one")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.StoppedAt == nil {
		t.Error("live-one still shows as recording after stopRecording()")
	}
}

func TestTUIModelStopRecordingOnStoppedSessionSetsErrorStatus(t *testing.T) {
	withTestHome(t)
	s, dir, err := session.Create("already-stopped")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Stop(dir); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	m, err := newTUIModel()
	if err != nil {
		t.Fatalf("newTUIModel: %v", err)
	}
	m.list.Select(0)

	m.stopRecording()
	if !m.statusIsError {
		t.Errorf("stopRecording() on an already-stopped session: statusIsError = false, want true (status: %q)", m.status)
	}
}
