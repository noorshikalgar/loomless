package engine

import (
	"testing"

	"github.com/noorshikalgar/loomless/internal/session"
)

// TestStopRecordingNamedDoesNotDisturbOtherCurrentSession is the reason
// StopRecordingNamed exists at all: the TUI needs to stop a specific
// selected session regardless of which one happens to be the single
// global "current" pointer. This exercises the real engine-level
// function (not just session.Session.Stop in isolation, which
// internal/session/session_test.go already covers) against two sessions
// created the normal way, without needing a real click-daemon subprocess
// — stopClickDaemon/stopIdleWatchdog are no-ops when no PID file exists
// for a session that was never actually recording via the daemon, which
// is exactly session.Create's state here.
func TestStopRecordingNamedDoesNotDisturbOtherCurrentSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, _, err := session.Create("session-a"); err != nil {
		t.Fatalf("Create(session-a): %v", err)
	}
	if _, _, err := session.Create("session-b"); err != nil { // becomes "current"
		t.Fatalf("Create(session-b): %v", err)
	}

	stopped, err := StopRecordingNamed("session-a")
	if err != nil {
		t.Fatalf("StopRecordingNamed(session-a): %v", err)
	}
	if stopped.Name != "session-a" || stopped.StoppedAt == nil {
		t.Errorf("StopRecordingNamed(session-a) = %+v, want session-a marked stopped", stopped)
	}

	cur, _, err := session.Current()
	if err != nil {
		t.Fatalf("session.Current() after stopping a non-current session: %v", err)
	}
	if cur.Name != "session-b" {
		t.Errorf("Current().Name = %q after StopRecordingNamed(session-a), want unaffected session-b", cur.Name)
	}

	// StopRecording (the "current" one) should still work normally
	// afterward — confirms the fix in session.Session.Stop didn't break
	// the original, still-current-pointer-based path.
	if _, err := StopRecording(); err != nil {
		t.Errorf("StopRecording() for the still-current session-b: %v", err)
	}
}
