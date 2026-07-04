package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rwrife/commit-sprout/internal/plant"
)

// fixedTime is a stable, non-zero timestamp used for last-commit fields.
var fixedTime = time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)

func TestDefaultState(t *testing.T) {
	got := DefaultState()
	if got.Version != SchemaVersion {
		t.Errorf("Version = %d, want %d", got.Version, SchemaVersion)
	}
	if got.HighestStage != plant.Seed.String() {
		t.Errorf("HighestStage = %q, want %q", got.HighestStage, plant.Seed.String())
	}
	if got.BestStreak != 0 {
		t.Errorf("BestStreak = %d, want 0", got.BestStreak)
	}
	if !got.LastCommitTime.IsZero() {
		t.Errorf("LastCommitTime = %v, want zero", got.LastCommitTime)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	want := State{
		Version:        SchemaVersion,
		HighestStage:   plant.Tall.String(),
		BestStreak:     9,
		LastCommitHash: "deadbeef",
		LastCommitTime: fixedTime,
	}

	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Version != want.Version {
		t.Errorf("Version = %d, want %d", got.Version, want.Version)
	}
	if got.HighestStage != want.HighestStage {
		t.Errorf("HighestStage = %q, want %q", got.HighestStage, want.HighestStage)
	}
	if got.BestStreak != want.BestStreak {
		t.Errorf("BestStreak = %d, want %d", got.BestStreak, want.BestStreak)
	}
	if got.LastCommitHash != want.LastCommitHash {
		t.Errorf("LastCommitHash = %q, want %q", got.LastCommitHash, want.LastCommitHash)
	}
	if !got.LastCommitTime.Equal(want.LastCommitTime) {
		t.Errorf("LastCommitTime = %v, want %v", got.LastCommitTime, want.LastCommitTime)
	}
}

func TestSaveStampsCurrentVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// A caller that forgets to set Version should still get a versioned file.
	if err := Save(path, State{HighestStage: plant.Sprout.String()}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Version != SchemaVersion {
		t.Errorf("Version = %d, want %d", got.Version, SchemaVersion)
	}
}

func TestLoadMissingFileReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load of missing file returned error: %v", err)
	}
	if got != DefaultState() {
		t.Errorf("Load = %+v, want DefaultState %+v", got, DefaultState())
	}
}

func TestLoadCorruptFileRecoversToDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := os.WriteFile(path, []byte("{ this is not valid json "), 0o600); err != nil {
		t.Fatalf("seed corrupt file: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load of corrupt file returned error: %v", err)
	}
	if got != DefaultState() {
		t.Errorf("Load = %+v, want DefaultState %+v", got, DefaultState())
	}
}

func TestLoadNormalizesLegacyAndGarbage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Version 0 (legacy), an unknown stage name, and a negative streak should
	// all be normalized on load.
	raw := `{"version":0,"highest_stage":"megatree","best_streak":-4}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("seed legacy file: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Version != SchemaVersion {
		t.Errorf("Version = %d, want %d", got.Version, SchemaVersion)
	}
	if got.HighestStage != plant.Seed.String() {
		t.Errorf("HighestStage = %q, want %q (unknown -> seed)", got.HighestStage, plant.Seed.String())
	}
	if got.BestStreak != 0 {
		t.Errorf("BestStreak = %d, want 0 (negative clamped)", got.BestStreak)
	}
}

func TestSaveIsAtomicLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := Save(path, DefaultState()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected exactly 1 file after Save, got %d: %v", len(entries), names)
	}
	if entries[0].Name() != "state.json" {
		t.Errorf("unexpected file name %q, want state.json", entries[0].Name())
	}
}

func TestSaveOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := Save(path, State{HighestStage: plant.Sprout.String(), BestStreak: 1}); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if err := Save(path, State{HighestStage: plant.Blooming.String(), BestStreak: 21}); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.HighestStage != plant.Blooming.String() || got.BestStreak != 21 {
		t.Errorf("after overwrite = %+v, want blooming/21", got)
	}
}

func TestSaveCreatesNestedDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "state.json")

	if err := Save(path, DefaultState()); err != nil {
		t.Fatalf("Save into nested path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at nested path: %v", err)
	}
}

func TestPlantProjection(t *testing.T) {
	s := State{
		HighestStage: plant.Leafy.String(),
		BestStreak:   5,
	}
	ps := s.Plant()
	if ps.HighestStage != plant.Leafy {
		t.Errorf("HighestStage = %v, want Leafy", ps.HighestStage)
	}
	if ps.BestStreak != 5 {
		t.Errorf("BestStreak = %d, want 5", ps.BestStreak)
	}

	// Garbage stage + negative streak degrade safely.
	bad := State{HighestStage: "???", BestStreak: -3}
	pbad := bad.Plant()
	if pbad.HighestStage != plant.Seed {
		t.Errorf("bad HighestStage = %v, want Seed", pbad.HighestStage)
	}
	if pbad.BestStreak != 0 {
		t.Errorf("bad BestStreak = %d, want 0", pbad.BestStreak)
	}
}

func TestFromPlantRatchetsAndRecordsCommit(t *testing.T) {
	prev := State{
		Version:      SchemaVersion,
		HighestStage: plant.Leafy.String(),
		BestStreak:   6,
	}

	// A run that grows further and sets a longer streak should ratchet both up
	// and record the new commit.
	ps := plant.PlantState{
		Stage:               plant.Tall,
		Streak:              8,
		UpdatedHighestStage: plant.Tall,
	}
	got := prev.FromPlant(ps, "cafef00d", fixedTime)

	if got.HighestStage != plant.Tall.String() {
		t.Errorf("HighestStage = %q, want tall", got.HighestStage)
	}
	if got.BestStreak != 8 {
		t.Errorf("BestStreak = %d, want 8", got.BestStreak)
	}
	if got.LastCommitHash != "cafef00d" || !got.LastCommitTime.Equal(fixedTime) {
		t.Errorf("commit not recorded: hash=%q time=%v", got.LastCommitHash, got.LastCommitTime)
	}
}

func TestFromPlantNeverRegresses(t *testing.T) {
	prev := State{
		Version:        SchemaVersion,
		HighestStage:   plant.Blooming.String(),
		BestStreak:     30,
		LastCommitHash: "old",
		LastCommitTime: fixedTime,
	}

	// A quiet run: computed peak lower than remembered, shorter streak, and no
	// new commit (zero time). Nothing should regress and the old commit info
	// must be preserved.
	ps := plant.PlantState{
		Stage:               plant.Tall,
		Streak:              2,
		UpdatedHighestStage: plant.Tall,
	}
	got := prev.FromPlant(ps, "", time.Time{})

	if got.HighestStage != plant.Blooming.String() {
		t.Errorf("HighestStage regressed to %q, want blooming", got.HighestStage)
	}
	if got.BestStreak != 30 {
		t.Errorf("BestStreak regressed to %d, want 30", got.BestStreak)
	}
	if got.LastCommitHash != "old" || !got.LastCommitTime.Equal(fixedTime) {
		t.Errorf("commit info lost on empty update: hash=%q time=%v", got.LastCommitHash, got.LastCommitTime)
	}
}

func TestDefaultPathXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join("/xdg/config", appDir, stateFile)
	if got != want {
		t.Errorf("DefaultPath = %q, want %q", got, want)
	}
}

func TestDefaultPathHomeFallback(t *testing.T) {
	// A non-absolute XDG value must be ignored in favor of the home fallback.
	t.Setenv("XDG_CONFIG_HOME", "relative/not/absolute")
	t.Setenv("HOME", "/home/tester")

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join("/home/tester", ".config", appDir, stateFile)
	if got != want {
		t.Errorf("DefaultPath = %q, want %q", got, want)
	}
}

// TestSavedFileIsReadableJSON guards the human-readable, indented format so the
// state file stays debuggable by hand.
func TestSavedFileIsReadableJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := Save(path, DefaultState()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	if _, ok := m["version"]; !ok {
		t.Errorf("saved JSON missing version field: %s", data)
	}
}
