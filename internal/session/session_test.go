package session

import "testing"

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
