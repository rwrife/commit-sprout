package store

import (
	"os"
	"path/filepath"
	"testing"
)

// abs is a tiny helper turning a relative path into the absolute form the store
// normalizes to, so tests can assert against exactly what gets persisted.
func abs(t *testing.T, p string) string {
	t.Helper()
	a, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("filepath.Abs(%q): %v", p, err)
	}
	return a
}

func TestAddGardenReposAbsolutizesAndDedups(t *testing.T) {
	s := DefaultState()
	s, added := s.AddGardenRepos("./a", "./b", "./a") // duplicate ./a
	if added != 2 {
		t.Fatalf("added = %d; want 2", added)
	}
	want := []string{abs(t, "./a"), abs(t, "./b")}
	if len(s.GardenRepos) != len(want) {
		t.Fatalf("GardenRepos = %v; want %v", s.GardenRepos, want)
	}
	for i := range want {
		if s.GardenRepos[i] != want[i] {
			t.Errorf("GardenRepos[%d] = %q; want %q", i, s.GardenRepos[i], want[i])
		}
	}
}

func TestAddGardenReposReportsOnlyNew(t *testing.T) {
	s := DefaultState()
	s, _ = s.AddGardenRepos("./a", "./b")
	s2, added := s.AddGardenRepos("./b", "./c") // b already present
	if added != 1 {
		t.Errorf("added = %d; want 1 (only ./c is new)", added)
	}
	if len(s2.GardenRepos) != 3 {
		t.Errorf("GardenRepos len = %d; want 3", len(s2.GardenRepos))
	}
}

func TestAddGardenReposSkipsBlanks(t *testing.T) {
	s := DefaultState()
	s, added := s.AddGardenRepos("  ", "", "./real")
	if added != 1 {
		t.Fatalf("added = %d; want 1", added)
	}
	if len(s.GardenRepos) != 1 || s.GardenRepos[0] != abs(t, "./real") {
		t.Errorf("GardenRepos = %v; want [%s]", s.GardenRepos, abs(t, "./real"))
	}
}

func TestRemoveGardenReposByAnyForm(t *testing.T) {
	s := DefaultState()
	s, _ = s.AddGardenRepos("./a", "./b", "./c")
	// Remove using a relative path; it should match the stored absolute form.
	s2, removed := s.RemoveGardenRepos("./b")
	if removed != 1 {
		t.Fatalf("removed = %d; want 1", removed)
	}
	for _, p := range s2.GardenRepos {
		if p == abs(t, "./b") {
			t.Errorf("./b still present after removal: %v", s2.GardenRepos)
		}
	}
	if len(s2.GardenRepos) != 2 {
		t.Errorf("GardenRepos len = %d; want 2", len(s2.GardenRepos))
	}
}

func TestRemoveGardenReposNoMatchIsZero(t *testing.T) {
	s := DefaultState()
	s, _ = s.AddGardenRepos("./a")
	_, removed := s.RemoveGardenRepos("./nope")
	if removed != 0 {
		t.Errorf("removed = %d; want 0 for a path not in the set", removed)
	}
}

// TestGardenReposSurviveRoundTrip ensures the saved set persists through
// Save/Load unchanged (absolute, ordered, de-duplicated).
func TestGardenReposSurviveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s := DefaultState()
	s, _ = s.AddGardenRepos("./x", "./y")
	if err := Save(path, s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.GardenRepos) != 2 {
		t.Fatalf("loaded GardenRepos = %v; want 2 entries", loaded.GardenRepos)
	}
	if loaded.GardenRepos[0] != abs(t, "./x") || loaded.GardenRepos[1] != abs(t, "./y") {
		t.Errorf("round-trip GardenRepos = %v; want [%s %s]",
			loaded.GardenRepos, abs(t, "./x"), abs(t, "./y"))
	}
}

// TestGardenReposAdditiveToExistingState checks backward compatibility: a state
// file with no garden_repos key still loads fine and starts with an empty set.
func TestGardenReposAdditiveToExistingState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	// Minimal legacy-ish document with no garden_repos field.
	if err := os.WriteFile(path, []byte(`{"version":1,"highest_stage":"leafy","best_streak":4}`), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.GardenRepos != nil {
		t.Errorf("expected nil GardenRepos for legacy file; got %v", loaded.GardenRepos)
	}
	if loaded.HighestStage != "leafy" {
		t.Errorf("legacy highest_stage lost: %q", loaded.HighestStage)
	}
}
