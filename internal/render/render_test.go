package render

import (
	"strings"
	"testing"
	"time"

	"github.com/rwrife/commit-sprout/internal/plant"
)

func TestSeedlingNotEmpty(t *testing.T) {
	got := Seedling()
	if strings.TrimSpace(got) == "" {
		t.Fatal("Seedling() returned empty output; expected ASCII art")
	}
}

func TestSeedlingIsStable(t *testing.T) {
	// The M1 seedling is hard-coded; guard against accidental edits.
	if Seedling() != seedling {
		t.Errorf("Seedling() did not match the seedling constant")
	}
}

// TestFramesExistForEveryStageAndHealth ensures the art table is complete: a
// non-empty frame for every (stage x health) combination the state machine can
// produce. A missing entry would fall back and hide a bug, so assert directly.
func TestFramesExistForEveryStageAndHealth(t *testing.T) {
	stages := []plant.Stage{plant.Seed, plant.Sprout, plant.Leafy, plant.Tall, plant.Blooming}
	healths := []plant.Health{plant.Healthy, plant.Thirsty, plant.Wilting}

	for _, s := range stages {
		byHealth, ok := frames[s]
		if !ok {
			t.Fatalf("no frames registered for stage %s", s)
		}
		for _, h := range healths {
			art, ok := byHealth[h]
			if !ok {
				t.Errorf("stage %s missing frame for health %s", s, h)
				continue
			}
			if strings.TrimSpace(art) == "" {
				t.Errorf("stage %s / health %s frame is blank", s, h)
			}
		}
	}
}

// TestArtVariesAcrossStages is the heart of the M4 "done when": running the
// tool must show a plant that visibly differs across stages. Assert that every
// healthy stage's art is distinct from every other.
func TestArtVariesAcrossStages(t *testing.T) {
	stages := []plant.Stage{plant.Seed, plant.Sprout, plant.Leafy, plant.Tall, plant.Blooming}
	seen := map[string]plant.Stage{}
	for _, s := range stages {
		art := artFor(s, plant.Healthy)
		if prev, dup := seen[art]; dup {
			t.Errorf("stages %s and %s render identical art; they should differ", prev, s)
		}
		seen[art] = s
	}
}

// TestArtVariesAcrossHealth ensures wilt states are visible in the art, not
// only the caption: for each stage, the three health frames must be distinct.
func TestArtVariesAcrossHealth(t *testing.T) {
	stages := []plant.Stage{plant.Seed, plant.Sprout, plant.Leafy, plant.Tall, plant.Blooming}
	for _, s := range stages {
		h := artFor(s, plant.Healthy)
		th := artFor(s, plant.Thirsty)
		w := artFor(s, plant.Wilting)
		if h == th {
			t.Errorf("stage %s: healthy and thirsty art are identical", s)
		}
		if h == w {
			t.Errorf("stage %s: healthy and wilting art are identical", s)
		}
		if th == w {
			t.Errorf("stage %s: thirsty and wilting art are identical", s)
		}
	}
}

// TestPlainFrameIsColorFree verifies the plain path (Color:false) emits no ANSI
// escape sequences, so piped/CI output stays clean.
func TestPlainFrameIsColorFree(t *testing.T) {
	ps := plant.PlantState{
		Stage:           plant.Blooming,
		Health:          plant.Healthy,
		Streak:          14,
		DaysSinceCommit: 0,
		Mood:            "In full bloom. Absolute unit of a plant.",
	}
	out := Frame(ps, Options{
		Color:      false,
		Now:        time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
		LastCommit: time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC),
	})
	if strings.Contains(out, "\x1b[") {
		t.Errorf("plain Frame contained ANSI escape codes:\n%q", out)
	}
}

// TestFrameSnapshotHealthyLeafy pins the exact plain-text render of a healthy
// leafy plant so accidental art/caption drift is caught.
func TestFrameSnapshotHealthyLeafy(t *testing.T) {
	ps := plant.PlantState{
		Stage:           plant.Leafy,
		Health:          plant.Healthy,
		Streak:          5,
		DaysSinceCommit: 0,
		Mood:            "Leafing out nicely. Keep it up.",
	}
	got := Frame(ps, Options{
		Color:      false,
		Now:        time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
		LastCommit: time.Date(2026, 7, 2, 8, 30, 0, 0, time.UTC),
	})

	want := strings.Join([]string{
		`   \ | /`,
		`    \|/`,
		`   --+--`,
		`     |`,
		`    _|_`,
		`   (   )`,
		`    '-'`,
		``,
		`stage:  leafy (healthy)`,
		`streak: 5 days`,
		`last:   today (2026-07-02)`,
		``,
		`Leafing out nicely. Keep it up.`,
	}, "\n")

	if got != want {
		t.Errorf("healthy leafy snapshot mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestFrameSnapshotWiltingTall pins a wilting tall plant, including the wilt
// hint line and "days ago" phrasing.
func TestFrameSnapshotWiltingTall(t *testing.T) {
	ps := plant.PlantState{
		Stage:           plant.Tall,
		Health:          plant.Wilting,
		Streak:          0,
		DaysSinceCommit: 5,
		Mood:            "Parched. A commit would really help right now.",
	}
	got := Frame(ps, Options{
		Color:      false,
		Now:        time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
		LastCommit: time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC),
	})

	want := strings.Join([]string{
		`   . . .`,
		`    \|/ `,
		`   ._+   `,
		`     \`,
		`     |`,
		`     |`,
		`    _|_`,
		`   (   )`,
		`    '-'`,
		``,
		`stage:  tall (wilting)`,
		`streak: no active streak`,
		`last:   5 days ago (2026-06-27)`,
		`        wilting ` + "\u2014" + ` commit to revive it`,
		``,
		`Parched. A commit would really help right now.`,
	}, "\n")

	if got != want {
		t.Errorf("wilting tall snapshot mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestFrameNoCommitsCaption checks the friendly seed messaging when there is no
// commit history at all (DaysSinceCommit == -1, zero LastCommit).
func TestFrameNoCommitsCaption(t *testing.T) {
	ps := plant.PlantState{
		Stage:           plant.Seed,
		Health:          plant.Healthy,
		Streak:          0,
		DaysSinceCommit: -1,
		Mood:            "Just a seed. Commit to make it grow.",
	}
	got := Frame(ps, Options{Color: false})

	if !strings.Contains(got, "plant a seed with your first commit") {
		t.Errorf("expected first-commit nudge in caption, got:\n%s", got)
	}
	if strings.Contains(got, "days ago") {
		t.Errorf("no-commit caption should not mention days ago:\n%s", got)
	}
}

// TestWiltHintThirstyCountdown verifies the thirsty countdown phrasing across
// the thirsty band (days 2 and 3 idle).
func TestWiltHintThirstyCountdown(t *testing.T) {
	cases := []struct {
		days int
		want string
	}{
		{days: 2, want: "2 days until it wilts"},
		{days: 3, want: "needs a commit today or it starts wilting"},
	}
	for _, tc := range cases {
		ps := plant.PlantState{Stage: plant.Leafy, Health: plant.Thirsty, DaysSinceCommit: tc.days}
		if got := wiltHint(ps); got != tc.want {
			t.Errorf("wiltHint(days=%d) = %q, want %q", tc.days, got, tc.want)
		}
	}
}

// TestStreakPhrasePluralization guards the streak wording edges.
func TestStreakPhrasePluralization(t *testing.T) {
	cases := map[int]string{
		0:  "no active streak",
		1:  "1 day",
		2:  "2 days",
		14: "14 days",
	}
	for in, want := range cases {
		if got := streakPhrase(in); got != want {
			t.Errorf("streakPhrase(%d) = %q, want %q", in, got, want)
		}
	}
}

// TestPromptGlyphSingleLine ensures prompt glyphs never span multiple lines and
// include the stage name.
func TestPromptGlyphSingleLine(t *testing.T) {
	ps := plant.PlantState{Stage: plant.Blooming, Health: plant.Healthy}
	got := PromptGlyph(ps)
	if strings.Contains(got, "\n") {
		t.Errorf("PromptGlyph must be single-line, got %q", got)
	}
	if !strings.Contains(got, "blooming") {
		t.Errorf("PromptGlyph should include stage name, got %q", got)
	}
}

// TestColorFrameIsRenderable makes sure the color path produces non-empty,
// still-contains-art output without panicking. (Actual ANSI depends on the
// detected terminal profile, which is intentionally environment-driven.)
func TestColorFrameIsRenderable(t *testing.T) {
	ps := plant.PlantState{
		Stage:           plant.Sprout,
		Health:          plant.Healthy,
		Streak:          1,
		DaysSinceCommit: 0,
		Mood:            "A fresh sprout!",
	}
	out := Frame(ps, Options{
		Color:      true,
		Now:        time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
		LastCommit: time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC),
	})
	if strings.TrimSpace(out) == "" {
		t.Fatal("color Frame returned empty output")
	}
	if !strings.Contains(out, "sprout") {
		t.Errorf("color Frame should still contain caption text, got:\n%s", out)
	}
}

// TestFrameDrawsPestsPlain verifies that a plant with pests renders pest glyphs
// in the plain (no-color) path, and includes a caption hint explaining them.
func TestFrameDrawsPestsPlain(t *testing.T) {
	ps := plant.PlantState{
		Stage:  plant.Leafy,
		Health: plant.Healthy,
		Streak: 3,
		Pests:  2,
	}
	out := Frame(ps, Options{Color: false, Now: time.Now()})

	if !strings.ContainsRune(out, pestGlyph) {
		t.Errorf("Frame output missing pest glyph %q:\n%s", string(pestGlyph), out)
	}
	if strings.Count(out, string(pestGlyph)) < 2 {
		t.Errorf("expected at least 2 pest glyphs, got %d:\n%s",
			strings.Count(out, string(pestGlyph)), out)
	}
	if !strings.Contains(out, "from reverts") {
		t.Errorf("Frame output missing pest caption hint:\n%s", out)
	}
}

// TestFrameNoPestsUnchanged confirms a pest-free plant renders no pest glyphs,
// keeping historical output byte-for-byte stable.
func TestFrameNoPestsUnchanged(t *testing.T) {
	ps := plant.PlantState{Stage: plant.Leafy, Health: plant.Healthy, Streak: 3}
	with := Frame(ps, Options{Color: false, Now: time.Now()})
	if strings.ContainsRune(with, pestGlyph) {
		t.Errorf("pest-free plant unexpectedly rendered a pest glyph:\n%s", with)
	}
}

// TestWithPestsIsNoOpAtZero guards the fast path: zero pests returns the art
// unchanged.
func TestWithPestsIsNoOpAtZero(t *testing.T) {
	art := "  \\|/\n   |\n  _|_"
	if got := withPests(art, plant.PlantState{Pests: 0}); got != art {
		t.Errorf("withPests changed art at zero pests:\n%s", got)
	}
}
