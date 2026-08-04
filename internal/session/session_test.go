package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if sessions, err := List(); err != nil || len(sessions) != 0 {
		t.Fatalf("List() with no sessions dir yet = %v, %v; want (nil, nil)", sessions, err)
	}

	if _, _, err := Create("alpha"); err != nil {
		t.Fatalf("Create(alpha): %v", err)
	}
	time.Sleep(2 * time.Millisecond) // guarantee a distinct, later StartedAt
	if _, _, err := Create("beta"); err != nil {
		t.Fatalf("Create(beta): %v", err)
	}

	sessions, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("List() = %d sessions, want 2", len(sessions))
	}
	if sessions[0].Name != "beta" || sessions[1].Name != "alpha" {
		t.Errorf("List() order = [%s, %s], want [beta, alpha] (most recently started first)", sessions[0].Name, sessions[1].Name)
	}
}

// TestListSkipsCorruptedSession confirms one unreadable session directory
// doesn't hide every other session from the list — a corrupted or
// mid-write session.json shouldn't make the whole TUI/CLI listing fail.
func TestListSkipsCorruptedSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, _, err := Create("good"); err != nil {
		t.Fatalf("Create(good): %v", err)
	}
	root, err := Root()
	if err != nil {
		t.Fatalf("Root: %v", err)
	}
	badDir := filepath.Join(root, "sessions", "bad")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatalf("mkdir bad session dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "session.json"), []byte("not valid json"), 0o644); err != nil {
		t.Fatalf("write corrupted session.json: %v", err)
	}

	sessions, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Name != "good" {
		t.Errorf("List() = %v, want just [good] (corrupted session skipped, not fatal)", sessions)
	}
}

// TestStopOnlyClearsMatchingCurrent confirms stopping session A never
// wipes out session B's "current" pointer just because B happens to be
// the live one — the exact bug that would bite the TUI's forthcoming
// by-name stop action if Stop() cleared "current" unconditionally.
func TestStopOnlyClearsMatchingCurrent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	sa, dirA, err := Create("session-a")
	if err != nil {
		t.Fatalf("Create(session-a): %v", err)
	}
	if _, _, err := Create("session-b"); err != nil { // becomes "current"
		t.Fatalf("Create(session-b): %v", err)
	}

	// session-a is stopped while session-b is still "current".
	if err := sa.Stop(dirA); err != nil {
		t.Fatalf("Stop(session-a): %v", err)
	}

	cur, _, err := Current()
	if err != nil {
		t.Fatalf("Current() after stopping a non-current session: %v", err)
	}
	if cur.Name != "session-b" {
		t.Errorf("Current().Name = %q, want %q (unaffected by stopping a different session)", cur.Name, "session-b")
	}
}

// TestStopClearsOwnCurrent is the normal case: stopping the session that
// actually is "current" still clears the pointer, same as before.
func TestStopClearsOwnCurrent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	s, dir, err := Create("only-session")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Stop(dir); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if _, _, err := Current(); err == nil {
		t.Errorf("Current() succeeded after stopping the session that was current, want an error")
	}
}

func TestRenameMovesDirectoryAndUpdatesName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	s, dir, err := Create("old-name")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Stop(dir); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	renamed, err := Rename("old-name", "new-name")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.Name != "new-name" {
		t.Errorf("Rename result Name = %q, want %q", renamed.Name, "new-name")
	}

	if _, _, err := Load("old-name"); err == nil {
		t.Error("Load(old-name) succeeded after rename, want it gone")
	}
	loaded, _, err := Load("new-name")
	if err != nil {
		t.Fatalf("Load(new-name) after rename: %v", err)
	}
	if loaded.Name != "new-name" {
		t.Errorf("Load(new-name).Name = %q, want %q (session.json itself must reflect the rename, not just the directory)", loaded.Name, "new-name")
	}
}

// TestRenameRejectsStillRecordingSession is the load-bearing case: the
// click daemon holds the session name it started with in memory for its
// whole lifetime and has no way to learn it changed mid-recording, so
// renaming while StoppedAt is nil must be refused, not silently corrupt
// the in-progress recording.
func TestRenameRejectsStillRecordingSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, _, err := Create("still-recording"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := Rename("still-recording", "renamed"); err == nil {
		t.Error("Rename succeeded on a still-recording session, want an error")
	}
	if _, _, err := Load("still-recording"); err != nil {
		t.Errorf("session directory moved despite the rejected rename: %v", err)
	}
}

func TestRenameToExistingNameFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	sa, dirA, err := Create("session-a")
	if err != nil {
		t.Fatalf("Create(session-a): %v", err)
	}
	if err := sa.Stop(dirA); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if _, _, err := Create("session-b"); err != nil {
		t.Fatalf("Create(session-b): %v", err)
	}

	if _, err := Rename("session-a", "session-b"); err == nil {
		t.Error("Rename to an already-existing name succeeded, want an error")
	}
	if _, _, err := Load("session-a"); err != nil {
		t.Errorf("session-a directory was moved despite the rejected rename: %v", err)
	}
}

func TestRenameToSameNameIsANoOp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Deliberately still recording (StoppedAt == nil) — renaming to the
	// same name must succeed even then, since nothing about the daemon's
	// in-memory name actually changes.
	if _, _, err := Create("same-name"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	s, err := Rename("same-name", "same-name")
	if err != nil {
		t.Fatalf("Rename to the same name: %v", err)
	}
	if s.Name != "same-name" {
		t.Errorf("Rename to the same name returned Name = %q, want %q", s.Name, "same-name")
	}
}

// TestRenameFollowsCurrentPointer confirms renaming the session that
// "story record stop"/"story step" would currently target keeps working
// after the rename — otherwise a rename would silently orphan the
// "current" pointer at a name that no longer exists on disk.
func TestRenameFollowsCurrentPointer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	s, dir, err := Create("current-one")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Stop(dir); err != nil { // stop first so Rename is allowed; Create() already made it "current"
		t.Fatalf("Stop: %v", err)
	}

	// Re-establish it as "current" the same way Create does, since
	// Stop() just cleared it (this test wants to verify the rename
	// follows an EXISTING current pointer, so restore that precondition
	// directly rather than via the daemon machinery Stop() normally
	// implies).
	if err := setCurrent("current-one"); err != nil {
		t.Fatalf("setCurrent: %v", err)
	}

	if _, err := Rename("current-one", "renamed-current"); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	cur, _, err := Current()
	if err != nil {
		t.Fatalf("Current() after renaming the current session: %v", err)
	}
	if cur.Name != "renamed-current" {
		t.Errorf("Current().Name = %q after rename, want %q", cur.Name, "renamed-current")
	}
}

func TestSetCropAndClearCrop(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, _, err := Create("cropme"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	step, err := AppendStep("cropme", "note", "App", "Title", nil)
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}

	s, err := SetCrop("cropme", step.Index, 10, 20, 100, 50)
	if err != nil {
		t.Fatalf("SetCrop: %v", err)
	}
	got := s.Steps[0].Crop
	if got == nil || got.X != 10 || got.Y != 20 || got.W != 100 || got.H != 50 {
		t.Fatalf("SetCrop result Crop = %+v, want {10 20 100 50}", got)
	}

	// Reload from disk to confirm it actually persisted, not just held
	// in the returned in-memory Session.
	reloaded, _, err := Load("cropme")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.Steps[0].Crop == nil || *reloaded.Steps[0].Crop != *got {
		t.Errorf("reloaded Crop = %+v, want %+v", reloaded.Steps[0].Crop, got)
	}

	s, err = ClearCrop("cropme", step.Index)
	if err != nil {
		t.Fatalf("ClearCrop: %v", err)
	}
	if s.Steps[0].Crop != nil {
		t.Errorf("after ClearCrop, Crop = %+v, want nil", s.Steps[0].Crop)
	}
}

func TestSetCropRejectsNonPositiveSize(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("badcrop"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	step, err := AppendStep("badcrop", "note", "App", "Title", nil)
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}
	if _, err := SetCrop("badcrop", step.Index, 0, 0, 0, 50); err == nil {
		t.Error("SetCrop with zero width succeeded, want an error")
	}
}

// TestClearCropOnUncroppedStepIsANoOp confirms clearing a crop that was
// never set isn't treated as a mistake worth rejecting.
func TestClearCropOnUncroppedStepIsANoOp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("nocrop"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	step, err := AppendStep("nocrop", "note", "App", "Title", nil)
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}
	if _, err := ClearCrop("nocrop", step.Index); err != nil {
		t.Errorf("ClearCrop on a step with no crop: %v, want no error", err)
	}
}

func TestBoundingBox(t *testing.T) {
	cases := []struct {
		name       string
		points     []Point
		x, y, w, h float64
	}{
		{"empty", nil, 0, 0, 0, 0},
		{"single point", []Point{{X: 5, Y: 10}}, 5, 10, 0, 0},
		{"diagonal", []Point{{X: 0, Y: 0}, {X: 10, Y: 20}}, 0, 0, 10, 20},
		{"unordered points", []Point{{X: 30, Y: 5}, {X: 10, Y: 40}, {X: 20, Y: 15}}, 10, 5, 20, 35},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x, y, w, h := boundingBox(tc.points)
			if x != tc.x || y != tc.y || w != tc.w || h != tc.h {
				t.Errorf("boundingBox(%v) = (%v,%v,%v,%v), want (%v,%v,%v,%v)", tc.points, x, y, w, h, tc.x, tc.y, tc.w, tc.h)
			}
		})
	}
}

// TestAddAnnotationFreehandComputesBoundingBox confirms a freehand
// annotation's stored X/Y/W/H always matches its Points, even though the
// caller passed different (garbage, in this test) x/y/w/h values —
// AddAnnotation must trust the path over whatever bounding box the
// caller thought it was.
func TestAddAnnotationFreehandComputesBoundingBox(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("freehand"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	step, err := AppendStep("freehand", "note", "App", "Title", nil)
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}

	points := []Point{{X: 10, Y: 10}, {X: 50, Y: 30}, {X: 20, Y: 60}}
	created, _, err := AddAnnotation("freehand", step.Index, 999, 999, 999, 999, "", points)
	if err != nil {
		t.Fatalf("AddAnnotation: %v", err)
	}
	if created.X != 10 || created.Y != 10 || created.W != 40 || created.H != 50 {
		t.Errorf("created bounding box = (%v,%v,%v,%v), want (10,10,40,50) — derived from Points, not the caller's x/y/w/h", created.X, created.Y, created.W, created.H)
	}
	if len(created.Points) != 3 {
		t.Errorf("created.Points has %d points, want 3", len(created.Points))
	}
}

func TestAddAnnotationRectangleUnaffectedByFreehandChange(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("rect"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	step, err := AppendStep("rect", "note", "App", "Title", nil)
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}
	created, _, err := AddAnnotation("rect", step.Index, 5, 15, 100, 60, "", nil)
	if err != nil {
		t.Fatalf("AddAnnotation: %v", err)
	}
	if created.X != 5 || created.Y != 15 || created.W != 100 || created.H != 60 {
		t.Errorf("rectangle annotation = (%v,%v,%v,%v), want the caller's own (5,15,100,60) unchanged", created.X, created.Y, created.W, created.H)
	}
	if created.Points != nil {
		t.Errorf("rectangle annotation has non-nil Points = %v, want nil", created.Points)
	}
}

// TestUpdateAnnotationFreehandMoveRecomputesBoundingBox confirms moving
// a freehand stroke (sending a new Points array) recomputes X/Y/W/H from
// the new path — the two must never be allowed to drift apart.
func TestUpdateAnnotationFreehandMoveRecomputesBoundingBox(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("movefreehand"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	step, err := AppendStep("movefreehand", "note", "App", "Title", nil)
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}
	created, _, err := AddAnnotation("movefreehand", step.Index, 0, 0, 0, 0, "", []Point{{X: 0, Y: 0}, {X: 10, Y: 10}})
	if err != nil {
		t.Fatalf("AddAnnotation: %v", err)
	}

	moved := []Point{{X: 100, Y: 100}, {X: 110, Y: 110}}
	s, err := UpdateAnnotation("movefreehand", step.Index, created.ID, 0, 0, 0, 0, created.Number, "", moved)
	if err != nil {
		t.Fatalf("UpdateAnnotation: %v", err)
	}
	got := s.Steps[0].Annotations[0]
	if got.X != 100 || got.Y != 100 || got.W != 10 || got.H != 10 {
		t.Errorf("moved bounding box = (%v,%v,%v,%v), want (100,100,10,10)", got.X, got.Y, got.W, got.H)
	}
}

// TestUpdateAnnotationRectangleDoesNotTouchPoints confirms a plain
// rectangle move/resize (points == nil) leaves Points alone — the nil
// vs. non-nil distinction is how UpdateAnnotation tells "just resize the
// rectangle" apart from "replace the freehand path."
func TestUpdateAnnotationRectangleDoesNotTouchPoints(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("rectmove"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	step, err := AppendStep("rectmove", "note", "App", "Title", nil)
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}
	created, _, err := AddAnnotation("rectmove", step.Index, 0, 0, 10, 10, "", nil)
	if err != nil {
		t.Fatalf("AddAnnotation: %v", err)
	}
	s, err := UpdateAnnotation("rectmove", step.Index, created.ID, 5, 5, 20, 20, created.Number, "", nil)
	if err != nil {
		t.Fatalf("UpdateAnnotation: %v", err)
	}
	got := s.Steps[0].Annotations[0]
	if got.X != 5 || got.Y != 5 || got.W != 20 || got.H != 20 {
		t.Errorf("resized rectangle = (%v,%v,%v,%v), want (5,5,20,20)", got.X, got.Y, got.W, got.H)
	}
	if got.Points != nil {
		t.Errorf("rectangle update set Points = %v, want still nil", got.Points)
	}
}

func TestVisibleSteps(t *testing.T) {
	base := func() Session {
		return Session{
			Steps: []Step{
				{Index: 1, Note: "one"},
				{Index: 2, Note: "two"},
				{Index: 3, Note: "three"},
			},
		}
	}

	t.Run("natural order by default", func(t *testing.T) {
		s := base()
		got := s.VisibleSteps()
		wantIndexes := []int{1, 2, 3}
		assertIndexes(t, got, wantIndexes)
	})

	t.Run("reordered via DisplayOrder", func(t *testing.T) {
		s := base()
		s.DisplayOrder = []int{3, 1, 2}
		got := s.VisibleSteps()
		assertIndexes(t, got, []int{3, 1, 2})
	})

	t.Run("hidden steps excluded", func(t *testing.T) {
		s := base()
		s.Hidden = []int{2}
		got := s.VisibleSteps()
		assertIndexes(t, got, []int{1, 3})
	})

	t.Run("reordered and hidden together", func(t *testing.T) {
		s := base()
		s.DisplayOrder = []int{3, 1, 2}
		s.Hidden = []int{1}
		got := s.VisibleSteps()
		assertIndexes(t, got, []int{3, 2})
	})

	t.Run("all hidden yields empty, not a crash", func(t *testing.T) {
		s := base()
		s.Hidden = []int{1, 2, 3}
		got := s.VisibleSteps()
		if len(got) != 0 {
			t.Fatalf("VisibleSteps() = %v, want empty", got)
		}
	})

	t.Run("stale DisplayOrder referencing a step index that no longer exists is skipped, not a crash", func(t *testing.T) {
		s := base()
		s.DisplayOrder = []int{1, 99, 2}
		got := s.VisibleSteps()
		assertIndexes(t, got, []int{1, 2})
	})

	t.Run("no steps at all", func(t *testing.T) {
		s := Session{}
		got := s.VisibleSteps()
		if len(got) != 0 {
			t.Fatalf("VisibleSteps() = %v, want empty", got)
		}
	})
}

func assertIndexes(t *testing.T, steps []Step, want []int) {
	t.Helper()
	if len(steps) != len(want) {
		t.Fatalf("got %d steps, want %d (steps=%v)", len(steps), len(want), steps)
	}
	for i, step := range steps {
		if step.Index != want[i] {
			t.Errorf("position %d: got index %d, want %d", i, step.Index, want[i])
		}
	}
}
