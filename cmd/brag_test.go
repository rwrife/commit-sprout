package cmd

import (
	"testing"
	"time"

	"github.com/rwrife/commit-sprout/internal/gitstat"
	"github.com/rwrife/commit-sprout/internal/plant"
)

// act builds a minimal Activity for brag tests. A non-zero LastCommit + true
// HasCommits mirror what the pipeline produces for a repo with history.
func act(total, window int, hasCommits bool) gitstat.Activity {
	a := gitstat.Activity{
		TotalInWindow: total,
		WindowDays:    window,
		HasCommits:    hasCommits,
	}
	if hasCommits {
		a.LastCommit = time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	}
	return a
}

// TestBragLine is the core table-driven check for the pure brag formatter over
// (Activity, PlantState, rememberedPeak, bestStreak).
func TestBragLine(t *testing.T) {
	cases := []struct {
		name string
		act  gitstat.Activity
		ps   plant.PlantState
		peak plant.Stage
		best int
		want string
	}{
		{
			name: "no commits ever",
			act:  act(0, 7, false),
			ps:   plant.PlantState{Stage: plant.Seed, Health: plant.Healthy, DaysSinceCommit: -1},
			peak: plant.Seed,
			want: "Nothing to brag about yet \u2014 plant a seed with your first commit. \U0001F331",
		},
		{
			name: "grew sprout to leafy",
			act:  act(14, 7, true),
			ps:   plant.PlantState{Stage: plant.Leafy, Health: plant.Healthy, Streak: 5, DaysSinceCommit: 0},
			peak: plant.Sprout,
			want: "This week your plant grew sprout\u2192leafy on a 5-day streak (14 commits in 7 days). \U0001F331",
		},
		{
			name: "held steady at leafy",
			act:  act(9, 7, true),
			ps:   plant.PlantState{Stage: plant.Leafy, Health: plant.Healthy, Streak: 4, DaysSinceCommit: 0},
			peak: plant.Leafy,
			want: "This week your plant held steady at leafy on a 4-day streak (9 commits in 7 days). \U0001F331",
		},
		{
			name: "held steady single commit and single streak day",
			act:  act(1, 7, true),
			ps:   plant.PlantState{Stage: plant.Sprout, Health: plant.Healthy, Streak: 1, DaysSinceCommit: 0},
			peak: plant.Sprout,
			want: "This week your plant held steady at sprout on a 1-day streak (1 commit in 7 days). \U0001F331",
		},
		{
			name: "healthy with no active streak",
			act:  act(2, 7, true),
			ps:   plant.PlantState{Stage: plant.Sprout, Health: plant.Healthy, Streak: 0, DaysSinceCommit: 1},
			peak: plant.Sprout,
			want: "This week your plant held steady at sprout on no active streak (2 commits in 7 days). \U0001F331",
		},
		{
			name: "thirsty with best streak tail",
			act:  act(3, 7, true),
			ps:   plant.PlantState{Stage: plant.Leafy, Health: plant.Thirsty, Streak: 0, DaysSinceCommit: 3},
			peak: plant.Leafy,
			best: 6,
			want: "Your plant is thirsty after 3 days quiet \u2014 a commit today keeps it green. (best streak: 6 days)",
		},
		{
			name: "thirsty with no streak history omits tail",
			act:  act(1, 7, true),
			ps:   plant.PlantState{Stage: plant.Sprout, Health: plant.Thirsty, Streak: 0, DaysSinceCommit: 2},
			peak: plant.Sprout,
			best: 0,
			want: "Your plant is thirsty after 2 days quiet \u2014 a commit today keeps it green. ",
		},
		{
			name: "wilting brags honestly",
			act:  act(0, 7, true),
			ps:   plant.PlantState{Stage: plant.Sprout, Health: plant.Wilting, Streak: 0, DaysSinceCommit: 5},
			peak: plant.Tall,
			best: 9,
			want: "Your plant is wilting after 5 days quiet \u2014 time to revive it with a commit. \U0001F940",
		},
		{
			name: "wilting after a single quiet day pluralizes correctly",
			act:  act(0, 7, true),
			ps:   plant.PlantState{Stage: plant.Sprout, Health: plant.Wilting, Streak: 0, DaysSinceCommit: 1},
			peak: plant.Sprout,
			want: "Your plant is wilting after 1 day quiet \u2014 time to revive it with a commit. \U0001F940",
		},
		{
			name: "grew to blooming from tall",
			act:  act(30, 7, true),
			ps:   plant.PlantState{Stage: plant.Blooming, Health: plant.Healthy, Streak: 14, DaysSinceCommit: 0},
			peak: plant.Tall,
			want: "This week your plant grew tall\u2192blooming on a 14-day streak (30 commits in 7 days). \U0001F331",
		},
		{
			name: "zero window falls back to default window days",
			act:  gitstat.Activity{TotalInWindow: 4, WindowDays: 0, HasCommits: true, LastCommit: time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)},
			ps:   plant.PlantState{Stage: plant.Sprout, Health: plant.Healthy, Streak: 2, DaysSinceCommit: 0},
			peak: plant.Sprout,
			want: "This week your plant held steady at sprout on a 2-day streak (4 commits in 7 days). \U0001F331",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := bragLine(c.act, c.ps, c.peak, c.best)
			if got != c.want {
				t.Errorf("bragLine mismatch:\n got  %q\n want %q", got, c.want)
			}
		})
	}
}

// TestBragCommits covers singular/plural of the commit-count clause.
func TestBragCommits(t *testing.T) {
	cases := []struct {
		n, window int
		want      string
	}{
		{0, 7, "0 commits in 7 days"},
		{1, 7, "1 commit in 7 days"},
		{14, 7, "14 commits in 7 days"},
		{3, 30, "3 commits in 30 days"},
	}
	for _, c := range cases {
		if got := bragCommits(c.n, c.window); got != c.want {
			t.Errorf("bragCommits(%d,%d) = %q, want %q", c.n, c.window, got, c.want)
		}
	}
}

// TestBragStreak covers the streak clause including the zero-streak wording.
func TestBragStreak(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{-1, "no active streak"},
		{0, "no active streak"},
		{1, "a 1-day streak"},
		{5, "a 5-day streak"},
	}
	for _, c := range cases {
		if got := bragStreak(c.in); got != c.want {
			t.Errorf("bragStreak(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestBragStreakTail verifies the best-streak tail uses the larger of current
// and best and stays empty when there is nothing to boast about.
func TestBragStreakTail(t *testing.T) {
	cases := []struct {
		name          string
		current, best int
		want          string
	}{
		{"nothing yet", 0, 0, ""},
		{"best drives it", 0, 6, "(best streak: 6 days)"},
		{"current beats stale best", 4, 2, "(best streak: 4 days)"},
		{"single day", 0, 1, "(best streak: 1 day)"},
	}
	for _, c := range cases {
		if got := bragStreakTail(c.current, c.best); got != c.want {
			t.Errorf("%s: bragStreakTail(%d,%d) = %q, want %q", c.name, c.current, c.best, got, c.want)
		}
	}
}

// TestBragQuietDays covers day pluralization for the dry-spell phrasing.
func TestBragQuietDays(t *testing.T) {
	if got := bragQuietDays(1); got != "1 day" {
		t.Errorf("bragQuietDays(1) = %q, want %q", got, "1 day")
	}
	if got := bragQuietDays(4); got != "4 days" {
		t.Errorf("bragQuietDays(4) = %q, want %q", got, "4 days")
	}
}
