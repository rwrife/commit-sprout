package cmd

import (
	"testing"
	"time"

	"github.com/rwrife/commit-sprout/internal/plant"
)

// TestStreakCount covers pluralization and the zero/negative "none" case.
func TestStreakCount(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{-3, "none"},
		{0, "none"},
		{1, "1 day"},
		{2, "2 days"},
		{14, "14 days"},
	}
	for _, c := range cases {
		if got := streakCount(c.in); got != c.want {
			t.Errorf("streakCount(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestStatusStreak verifies the "(best: N)" suffix only appears when the
// remembered best actually exceeds the current streak.
func TestStatusStreak(t *testing.T) {
	cases := []struct {
		name          string
		current, best int
		want          string
	}{
		{"no best beats current", 6, 6, "6 days"},
		{"best below current", 6, 3, "6 days"},
		{"best above current", 6, 11, "6 days (best: 11 days)"},
		{"fresh plant", 0, 0, "none"},
		{"best from a dead streak", 0, 9, "none (best: 9 days)"},
	}
	for _, c := range cases {
		if got := statusStreak(c.current, c.best); got != c.want {
			t.Errorf("%s: statusStreak(%d,%d) = %q, want %q",
				c.name, c.current, c.best, got, c.want)
		}
	}
}

// TestStatusLastCommit covers the no-commit sentinel and the today/yesterday/N
// phrasing with a pinned reference time for stability.
func TestStatusLastCommit(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	day := func(d int) time.Time { return now.AddDate(0, 0, -d) }

	cases := []struct {
		name string
		ps   plant.PlantState
		last time.Time
		want string
	}{
		{
			name: "no commits",
			ps:   plant.PlantState{DaysSinceCommit: -1},
			last: time.Time{},
			want: "no commits yet \u2014 plant a seed with your first commit",
		},
		{
			name: "today",
			ps:   plant.PlantState{DaysSinceCommit: 0},
			last: day(0),
			want: "today (2026-07-04)",
		},
		{
			name: "yesterday",
			ps:   plant.PlantState{DaysSinceCommit: 1},
			last: day(1),
			want: "yesterday (2026-07-03)",
		},
		{
			name: "several days",
			ps:   plant.PlantState{DaysSinceCommit: 5},
			last: day(5),
			want: "5 days ago (2026-06-29)",
		},
	}
	for _, c := range cases {
		if got := statusLastCommit(c.ps, c.last, now); got != c.want {
			t.Errorf("%s: statusLastCommit = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestStatusWilt checks the days-until-wilt nudge across every health state,
// mirroring plant's thresholds (thirsty days 2-3 idle, wilting day 4+).
func TestStatusWilt(t *testing.T) {
	cases := []struct {
		name string
		ps   plant.PlantState
		want string
	}{
		{
			name: "no commits is plainly healthy",
			ps:   plant.PlantState{Health: plant.Healthy, DaysSinceCommit: -1},
			want: "healthy",
		},
		{
			name: "committed today has two days cushion",
			ps:   plant.PlantState{Health: plant.Healthy, DaysSinceCommit: 0},
			want: "healthy \u2014 2 days of cushion before it gets thirsty",
		},
		{
			name: "committed yesterday nudges to commit today",
			ps:   plant.PlantState{Health: plant.Healthy, DaysSinceCommit: 1},
			want: "healthy \u2014 commit today to stay ahead of thirst",
		},
		{
			name: "thirsty day 2 has two days until wilt",
			ps:   plant.PlantState{Health: plant.Thirsty, DaysSinceCommit: 2},
			want: "thirsty \u2014 2 days until it wilts",
		},
		{
			name: "thirsty day 3 is last-chance",
			ps:   plant.PlantState{Health: plant.Thirsty, DaysSinceCommit: 3},
			want: "thirsty \u2014 commit today or it starts wilting",
		},
		{
			name: "wilting asks for water",
			ps:   plant.PlantState{Health: plant.Wilting, DaysSinceCommit: 6},
			want: "wilting \u2014 commit to revive it",
		},
	}
	for _, c := range cases {
		if got := statusWilt(c.ps); got != c.want {
			t.Errorf("%s: statusWilt = %q, want %q", c.name, got, c.want)
		}
	}
}
