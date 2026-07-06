package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
	if !reflect.DeepEqual(got, DefaultState()) {
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
	if !reflect.DeepEqual(got, DefaultState()) {
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
	ps := s.Plant(fixedTime, time.Time{})
	if ps.HighestStage != plant.Leafy {
		t.Errorf("HighestStage = %v, want Leafy", ps.HighestStage)
	}
	if ps.BestStreak != 5 {
		t.Errorf("BestStreak = %d, want 5", ps.BestStreak)
	}

	// Garbage stage + negative streak degrade safely.
	bad := State{HighestStage: "???", BestStreak: -3}
	pbad := bad.Plant(fixedTime, time.Time{})
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

// --- Watering-can grace period ---------------------------------------------

// graceNow is a stable reference clock for the watering tests.
var graceNow = time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)

// day returns graceNow shifted by n days (negative = past), as a YYYY-MM-DD key.
func day(n int) string {
	return graceNow.AddDate(0, 0, n).Format("2006-01-02")
}

// commitAt returns a commit time n days before graceNow.
func commitAt(n int) time.Time {
	return graceNow.AddDate(0, 0, -n)
}

func TestEffectiveGraceScopesToDrySpell(t *testing.T) {
	cases := []struct {
		name       string
		watered    []string
		lastCommit time.Time
		want       int
	}{
		{"none", nil, commitAt(3), 0},
		{"one recent watering after commit", []string{day(-1)}, commitAt(3), 1},
		{"two waterings after commit", []string{day(-2), day(-1)}, commitAt(3), 2},
		{"capped at max", []string{day(-3), day(-2), day(-1)}, commitAt(4), plant.MaxGraceDays},
		{"watering before commit ignored", []string{day(-5)}, commitAt(2), 0},
		{"future watering ignored", []string{day(3)}, commitAt(2), 0},
		{"no commit: counts all past waterings", []string{day(-1)}, time.Time{}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := State{WateredDates: tc.watered}
			if got := s.EffectiveGrace(graceNow, tc.lastCommit); got != tc.want {
				t.Errorf("EffectiveGrace = %d; want %d", got, tc.want)
			}
		})
	}
}

func TestWaterRecordsTodayOnce(t *testing.T) {
	s := State{}
	last := commitAt(3)

	after, res := s.Water(graceNow, last)
	if !res.Applied {
		t.Fatalf("first water not applied: %q", res.Reason)
	}
	if res.Grace != 1 {
		t.Errorf("grace after first water = %d; want 1", res.Grace)
	}
	if len(after.WateredDates) != 1 || after.WateredDates[0] != day(0) {
		t.Errorf("WateredDates = %v; want [%s]", after.WateredDates, day(0))
	}

	// Watering again the same day is a no-op.
	again, res2 := after.Water(graceNow, last)
	if res2.Applied {
		t.Errorf("second same-day water should be a no-op")
	}
	if len(again.WateredDates) != 1 {
		t.Errorf("same-day water changed dates: %v", again.WateredDates)
	}
}

func TestWaterRefusesAtCap(t *testing.T) {
	// Already at the cap for the current dry spell: further watering refused.
	s := State{WateredDates: []string{day(-2), day(-1)}}
	last := commitAt(3)
	if got := s.EffectiveGrace(graceNow, last); got != plant.MaxGraceDays {
		t.Fatalf("precondition: grace = %d; want %d", got, plant.MaxGraceDays)
	}

	after, res := s.Water(graceNow, last)
	if res.Applied {
		t.Errorf("water at cap should be refused; reason=%q", res.Reason)
	}
	// No today entry should have been banked.
	for _, d := range after.WateredDates {
		if d == day(0) {
			t.Errorf("refused water still banked today's date")
		}
	}
}

func TestFreshCommitPrunesGrace(t *testing.T) {
	// Two waterings buy grace during a gap...
	s := State{WateredDates: []string{day(-2), day(-1)}}
	if got := s.EffectiveGrace(graceNow, commitAt(3)); got != plant.MaxGraceDays {
		t.Fatalf("precondition grace = %d; want %d", got, plant.MaxGraceDays)
	}

	// ...then a commit lands today. FromPlant should prune waterings on/before
	// the commit day, so grace resets for the new (zero-length) dry spell.
	ps := plant.PlantState{Stage: plant.Leafy, Streak: 4, UpdatedHighestStage: plant.Leafy}
	after := s.FromPlant(ps, "abc123", graceNow)

	if got := after.EffectiveGrace(graceNow, graceNow); got != 0 {
		t.Errorf("grace after commit = %d; want 0 (pruned). dates=%v", got, after.WateredDates)
	}
	if len(after.WateredDates) != 0 {
		t.Errorf("waterings not pruned after commit: %v", after.WateredDates)
	}
}

func TestNormalizeWateringsSortsDedupesAndDropsGarbage(t *testing.T) {
	in := State{WateredDates: []string{day(-1), day(-1), "not-a-date", day(-3), ""}}
	got := normalize(in).WateredDates
	want := []string{day(-3), day(-1)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("normalized waterings = %v; want %v", got, want)
	}
}

func TestWateringsSurviveSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	in := DefaultState()
	in.WateredDates = []string{day(-1), day(0)}
	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got.WateredDates, []string{day(-1), day(0)}) {
		t.Errorf("round-tripped waterings = %v; want %v", got.WateredDates, []string{day(-1), day(0)})
	}
}
