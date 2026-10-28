package session

import (
	"fmt"
	"sync"
	"testing"
)

// Before loadForUpdate's lock, N concurrent appends each loaded the same
// version of session.json, added one step, and saved — so later saves
// silently overwrote earlier ones and most steps vanished. Separate
// goroutines open separate lock file descriptors, which flock treats
// exactly like separate processes (click daemon vs. CLI vs. editor).
func TestConcurrentAppendStepLosesNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("race"); err != nil {
		t.Fatal(err)
	}

	const n = 40
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := AppendStep("race", fmt.Sprintf("note %d", i), "", "", nil); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("AppendStep: %v", err)
	}

	s, _, err := Load("race")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Steps) != n {
		t.Fatalf("got %d steps after %d concurrent appends — updates were lost", len(s.Steps), n)
	}
	seen := make(map[int]bool, n)
	for _, step := range s.Steps {
		if seen[step.Index] {
			t.Fatalf("duplicate step index %d", step.Index)
		}
		seen[step.Index] = true
	}
}

// A plain truncate-then-write save let a concurrent Load read an empty
// or half-written file ("unexpected end of JSON input") — exactly what
// CI caught in TestPolishCaptionAsyncUpdatesNoteWhenConfigured. With
// atomic rename, readers only ever see complete versions.
func TestConcurrentReadsNeverSeeTornFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("torn"); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendStep("torn", "start", "", "", nil); err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := EditNote("torn", 1, fmt.Sprintf("edit %d with some padding to make writes non-trivial", i)); err != nil {
				t.Errorf("EditNote: %v", err)
				return
			}
		}
	}()

	for i := 0; i < 500; i++ {
		if _, _, err := Load("torn"); err != nil {
			close(stop)
			wg.Wait()
			t.Fatalf("Load during concurrent writes failed on read %d: %v", i, err)
		}
	}
	close(stop)
	wg.Wait()
}
