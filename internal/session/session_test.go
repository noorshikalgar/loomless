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
// "loomless record stop"/"loomless step" would currently target keeps working
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
	created, _, err := AddAnnotation("freehand", step.Index, AnnotationShape{X: 999, Y: 999, W: 999, H: 999, Kind: "freehand", Points: points})
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
	created, _, err := AddAnnotation("rect", step.Index, AnnotationShape{X: 5, Y: 15, W: 100, H: 60, Kind: "rect"})
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
	created, _, err := AddAnnotation("movefreehand", step.Index, AnnotationShape{Kind: "freehand", Points: []Point{{X: 0, Y: 0}, {X: 10, Y: 10}}})
	if err != nil {
		t.Fatalf("AddAnnotation: %v", err)
	}

	moved := []Point{{X: 100, Y: 100}, {X: 110, Y: 110}}
	s, err := UpdateAnnotation("movefreehand", step.Index, created.ID, created.Number, AnnotationShape{Kind: "freehand", Points: moved})
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
	created, _, err := AddAnnotation("rectmove", step.Index, AnnotationShape{X: 0, Y: 0, W: 10, H: 10, Kind: "rect"})
	if err != nil {
		t.Fatalf("AddAnnotation: %v", err)
	}
	s, err := UpdateAnnotation("rectmove", step.Index, created.ID, created.Number, AnnotationShape{X: 5, Y: 5, W: 20, H: 20, Kind: "rect"})
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

// TestInsertContentStepLandsAtTheRequestedPosition confirms the new
// step's index appears in DisplayOrder at exactly the requested slot —
// not just appended to Steps, which is the mistake that would leave it
// invisible in the editor (see VisibleSteps' comment on why every
// consumer must go through DisplayOrder).
func TestInsertContentStepLandsAtTheRequestedPosition(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("insertme"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	a, err := AppendStep("insertme", "a", "", "", nil)
	if err != nil {
		t.Fatalf("AppendStep a: %v", err)
	}
	b, err := AppendStep("insertme", "b", "", "", nil)
	if err != nil {
		t.Fatalf("AppendStep b: %v", err)
	}

	s, newIndex, err := InsertContentStep("insertme", 1)
	if err != nil {
		t.Fatalf("InsertContentStep: %v", err)
	}
	want := []int{a.Index, newIndex, b.Index}
	if len(s.DisplayOrder) != 3 {
		t.Fatalf("DisplayOrder = %v, want length 3", s.DisplayOrder)
	}
	for i, idx := range want {
		if s.DisplayOrder[i] != idx {
			t.Errorf("DisplayOrder[%d] = %d, want %d (full order %v)", i, s.DisplayOrder[i], idx, s.DisplayOrder)
		}
	}

	var newStep *Step
	for i := range s.Steps {
		if s.Steps[i].Index == newIndex {
			newStep = &s.Steps[i]
		}
	}
	if newStep == nil {
		t.Fatalf("new step %d not found in Steps", newIndex)
	}
	if !newStep.IsContent {
		t.Error("inserted step IsContent = false, want true")
	}
	if newStep.Note != "" || newStep.ImageFile != "" || newStep.Command != "" {
		t.Errorf("inserted step should be empty, got %+v", newStep)
	}
}

// TestInsertContentStepClampsOutOfRangePosition confirms a position
// beyond either end of the list is clamped to the nearest valid slot
// rather than rejected — the frontend always derives position from its
// own current rendering of the list, so there's no realistic "invalid"
// value to guard against, only ends of the list.
func TestInsertContentStepClampsOutOfRangePosition(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("clampme"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	a, err := AppendStep("clampme", "a", "", "", nil)
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}

	s, newIndex, err := InsertContentStep("clampme", -5)
	if err != nil {
		t.Fatalf("InsertContentStep(-5): %v", err)
	}
	if len(s.DisplayOrder) != 2 || s.DisplayOrder[0] != newIndex || s.DisplayOrder[1] != a.Index {
		t.Errorf("DisplayOrder after negative position = %v, want [%d %d]", s.DisplayOrder, newIndex, a.Index)
	}

	s, newIndex2, err := InsertContentStep("clampme", 999)
	if err != nil {
		t.Fatalf("InsertContentStep(999): %v", err)
	}
	last := len(s.DisplayOrder) - 1
	if s.DisplayOrder[last] != newIndex2 {
		t.Errorf("DisplayOrder after huge position = %v, want last element %d", s.DisplayOrder, newIndex2)
	}
}

// TestInsertContentStepWithNoPriorDisplayOrderSynthesizesOneFirst
// confirms a session that has never been explicitly reordered (so
// DisplayOrder is still empty/nil on disk) still gets a correctly
// ordered, fully-materialized DisplayOrder after an insert — not just
// the two-element list you'd get from naively appending to a nil slice.
func TestInsertContentStepWithNoPriorDisplayOrderSynthesizesOneFirst(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("synthme"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	a, err := AppendStep("synthme", "a", "", "", nil)
	if err != nil {
		t.Fatalf("AppendStep a: %v", err)
	}
	b, err := AppendStep("synthme", "b", "", "", nil)
	if err != nil {
		t.Fatalf("AppendStep b: %v", err)
	}

	s, newIndex, err := InsertContentStep("synthme", 2)
	if err != nil {
		t.Fatalf("InsertContentStep: %v", err)
	}
	want := []int{a.Index, b.Index, newIndex}
	if len(s.DisplayOrder) != len(want) {
		t.Fatalf("DisplayOrder = %v, want %v", s.DisplayOrder, want)
	}
	for i, idx := range want {
		if s.DisplayOrder[i] != idx {
			t.Errorf("DisplayOrder[%d] = %d, want %d", i, s.DisplayOrder[i], idx)
		}
	}
}

// TestAnnotationIsAnnotatedNilMeansLegacyTrue confirms an annotation
// loaded from a session.json written before the Annotated field existed
// (Annotated == nil, since the JSON key was simply never present)
// still reports itself as annotated — the whole point of using *bool
// instead of bool is that every annotation saved under the old rules
// (where a badge was always shown) must keep showing one, not silently
// lose it the moment this field was added.
func TestAnnotationIsAnnotatedNilMeansLegacyTrue(t *testing.T) {
	legacy := Annotation{ID: 1, Number: 1, X: 0, Y: 0, W: 10, H: 10}
	if !legacy.IsAnnotated() {
		t.Error("Annotation with Annotated == nil (legacy data) reports IsAnnotated() == false, want true")
	}

	on := true
	if annotated := (Annotation{Annotated: &on}); !annotated.IsAnnotated() {
		t.Error("Annotation with Annotated == &true reports IsAnnotated() == false, want true")
	}
	off := false
	if unannotated := (Annotation{Annotated: &off}); unannotated.IsAnnotated() {
		t.Error("Annotation with Annotated == &false reports IsAnnotated() == true, want false")
	}
}

// TestAddAnnotationRespectsShapeAnnotated confirms a newly created
// shape's badge visibility comes from exactly what the caller asked
// for (decision #30: a shape defaults to NOT annotated unless the
// caller explicitly says otherwise — e.g. the click daemon's automatic
// ring, which always wants one) — not silently defaulted true the way
// every annotation used to be before this feature existed.
func TestAddAnnotationRespectsShapeAnnotated(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("shapeflag"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	step, err := AppendStep("shapeflag", "note", "", "", nil)
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}

	unannotated, _, err := AddAnnotation("shapeflag", step.Index, AnnotationShape{X: 0, Y: 0, W: 10, H: 10, Kind: "rect", Annotated: false})
	if err != nil {
		t.Fatalf("AddAnnotation (unannotated): %v", err)
	}
	if unannotated.IsAnnotated() {
		t.Error("shape created with Annotated: false reports IsAnnotated() == true")
	}
	if unannotated.Annotated == nil {
		t.Error("shape created with Annotated: false has a nil Annotated pointer, want an explicit &false (nil is reserved for legacy data)")
	}

	annotated, _, err := AddAnnotation("shapeflag", step.Index, AnnotationShape{X: 0, Y: 0, W: 10, H: 10, Kind: "rect", Annotated: true})
	if err != nil {
		t.Fatalf("AddAnnotation (annotated): %v", err)
	}
	if !annotated.IsAnnotated() {
		t.Error("shape created with Annotated: true reports IsAnnotated() == false")
	}
}

// TestUpdateAnnotationTogglesAnnotated confirms the annotated flag
// round-trips through an update the same way position/color/kind do —
// this is the mechanism the editor's per-shape "add annotation" toggle
// button relies on.
func TestUpdateAnnotationTogglesAnnotated(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("toggleflag"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	step, err := AppendStep("toggleflag", "note", "", "", nil)
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}
	created, _, err := AddAnnotation("toggleflag", step.Index, AnnotationShape{X: 0, Y: 0, W: 10, H: 10, Kind: "rect", Annotated: false})
	if err != nil {
		t.Fatalf("AddAnnotation: %v", err)
	}

	s, err := UpdateAnnotation("toggleflag", step.Index, created.ID, 5, AnnotationShape{X: 0, Y: 0, W: 10, H: 10, Kind: "rect", Annotated: true})
	if err != nil {
		t.Fatalf("UpdateAnnotation: %v", err)
	}
	got := s.Steps[0].Annotations[0]
	if !got.IsAnnotated() || got.Number != 5 {
		t.Errorf("after toggling on: IsAnnotated()=%v Number=%d, want true 5", got.IsAnnotated(), got.Number)
	}

	reloaded, _, err := Load("toggleflag")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reloaded.Steps[0].Annotations[0].IsAnnotated() {
		t.Error("reloaded annotation lost its annotated=true state — not persisted")
	}
}

// TestAddAnnotationShapeKindsPersist confirms Kind round-trips through
// save/reload for every new shape kind, not just rect/freehand —
// covers the actual new surface decision #30 added (circle/line/arrow).
func TestAddAnnotationShapeKindsPersist(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("kinds"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	step, err := AppendStep("kinds", "note", "", "", nil)
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}

	if _, _, err := AddAnnotation("kinds", step.Index, AnnotationShape{X: 0, Y: 0, W: 40, H: 30, Kind: "circle"}); err != nil {
		t.Fatalf("AddAnnotation circle: %v", err)
	}
	if _, _, err := AddAnnotation("kinds", step.Index, AnnotationShape{Kind: "line", Points: []Point{{X: 0, Y: 0}, {X: 50, Y: 50}}}); err != nil {
		t.Fatalf("AddAnnotation line: %v", err)
	}
	if _, _, err := AddAnnotation("kinds", step.Index, AnnotationShape{Kind: "arrow", Points: []Point{{X: 0, Y: 0}, {X: 50, Y: 50}}}); err != nil {
		t.Fatalf("AddAnnotation arrow: %v", err)
	}

	reloaded, _, err := Load("kinds")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	anns := reloaded.Steps[0].Annotations
	if len(anns) != 3 {
		t.Fatalf("got %d annotations, want 3", len(anns))
	}
	wantKinds := []string{"circle", "line", "arrow"}
	for i, want := range wantKinds {
		if anns[i].Kind != want {
			t.Errorf("annotation %d Kind = %q, want %q", i, anns[i].Kind, want)
		}
	}
	if anns[1].W != 50 || anns[1].H != 50 {
		t.Errorf("line annotation bounding box = (%v,%v), want derived (50,50) from its two points", anns[1].W, anns[1].H)
	}
}

func TestEditHeaderStylePersistsAndRoundTrips(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("headerstyle"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	step, err := AppendStep("headerstyle", "note", "App", "Title", nil)
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}

	for _, style := range []string{"app", "title", "hidden", ""} {
		s, err := EditHeaderStyle("headerstyle", step.Index, style)
		if err != nil {
			t.Fatalf("EditHeaderStyle(%q): %v", style, err)
		}
		if s.Steps[0].HeaderStyle != style {
			t.Errorf("HeaderStyle = %q, want %q", s.Steps[0].HeaderStyle, style)
		}
		reloaded, _, err := Load("headerstyle")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if reloaded.Steps[0].HeaderStyle != style {
			t.Errorf("reloaded HeaderStyle = %q, want %q", reloaded.Steps[0].HeaderStyle, style)
		}
		// The underlying captured data must never be touched by a
		// display-only setting — switching styles back and forth has
		// to always have the same original App/Title to show again.
		if reloaded.Steps[0].WindowApp != "App" || reloaded.Steps[0].WindowTitle != "Title" {
			t.Errorf("EditHeaderStyle(%q) altered WindowApp/WindowTitle: got %q/%q, want unchanged App/Title", style, reloaded.Steps[0].WindowApp, reloaded.Steps[0].WindowTitle)
		}
	}
}

func TestEditHeaderStyleRejectsUnknownValue(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("badstyle"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	step, err := AppendStep("badstyle", "note", "App", "Title", nil)
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}
	if _, err := EditHeaderStyle("badstyle", step.Index, "bogus"); err == nil {
		t.Error("EditHeaderStyle with an unrecognized style succeeded, want an error")
	}
}
