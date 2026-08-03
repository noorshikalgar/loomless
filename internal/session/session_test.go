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
