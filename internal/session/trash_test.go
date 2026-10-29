package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrashThenRestoreRoundTrips(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, dir, err := Create("doomed_name")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddStep(dir, "a note", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(dir); err != nil {
		t.Fatal(err)
	}

	id, err := Trash("doomed_name")
	if err != nil {
		t.Fatalf("Trash: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("session directory still in sessions/ after Trash")
	}
	list, _ := List()
	if len(list) != 0 {
		t.Fatalf("List() returned %d sessions after trashing the only one", len(list))
	}
	if _, _, err := Current(); err == nil {
		t.Fatal("current-session pointer still set to a trashed session")
	}

	name, err := Restore(id)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if name != "doomed_name" {
		t.Fatalf("Restore returned name %q, want doomed_name (underscore in name must survive the split)", name)
	}
	restored, _, err := Load("doomed_name")
	if err != nil {
		t.Fatalf("Load after Restore: %v", err)
	}
	if len(restored.Steps) != 1 || restored.Steps[0].Note != "a note" {
		t.Fatal("restored session lost its data")
	}
}

func TestTrashRefusesRecordingSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Create("live"); err != nil {
		t.Fatal(err)
	}
	if _, err := Trash("live"); err == nil || !strings.Contains(err.Error(), "still recording") {
		t.Fatalf("Trash(recording session) err = %v, want a still-recording refusal", err)
	}
}

func TestRestoreRefusesToOverwriteExistingSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, dir, _ := Create("dup")
	s.Stop(dir)
	id, err := Trash("dup")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Create("dup"); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(id); err == nil {
		t.Fatal("Restore overwrote a session created after the original was trashed")
	}
}

func TestRestoreRejectsTraversalIDs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, id := range []string{"../x__1", "a/b__1", "", "noseparator"} {
		if _, err := Restore(id); err == nil {
			t.Errorf("Restore(%q) succeeded, want error", id)
		}
	}
	if _, err := os.Stat(filepath.Join(home, "x")); err == nil {
		t.Fatal("Restore escaped the sessions directory")
	}
}
