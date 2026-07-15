package plant

import (
	"testing"
	"time"

	"github.com/rwrife/commit-sprout/internal/gitstat"
)

// fixedNow is a stable reference clock. UTC keeps the day-boundary math in the
// fixtures obvious.
var fixedNow = time.Date(2026, time.June, 30, 12, 0, 0, 0, time.UTC)

// commitDaysAgo returns a timestamp N whole days before fixedNow (still at
// noon, so it's unambiguously that calendar day).
func commitDaysAgo(n int) time.Time {
	return fixedNow.AddDate(0, 0, -n)
}

// act builds an Activity with the fields the state machine actually reads.
// hasCommits is derived from whether a last-commit day is provided.
func act(streak, total, lastDaysAgo int, hasCommits bool) gitstat.Activity {
	a := gitstat.Activity{
		Author:        "me@example.com",
		WindowDays:    7,
		HasCommits:    hasCommits,
		TotalInWindow: total,
		Streak:        streak,
	}
	if hasCommits {
		a.LastCommit = commitDaysAgo(lastDaysAgo)
	}
	return a
}

func TestComputeStageByStreak(t *testing.T) {
	cases := []struct {
		name   string
		streak int
		total  int
		want   Stage
	}{
		{"no streak, no commits -> seed", 0, 0, Seed},
		{"one day -> sprout", 1, 1, Sprout},
		{"two days -> still sprout", 2, 2, Sprout},
		{"three days -> leafy", 3, 3, Leafy},
		{"six days -> leafy", 6, 6, Leafy},
		{"seven days -> tall", 7, 7, Tall},
		{"thirteen days -> tall", 13, 20, Tall},
		{"fourteen days -> blooming", 14, 30, Blooming},
		{"long streak -> blooming", 40, 60, Blooming},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Commit today so health is Healthy and doesn't interfere
			// with the pure stage-by-streak check.
			a := act(tc.streak, tc.total, 0, tc.streak > 0 || tc.total > 0)
			got := Compute(a, State{}, fixedNow)
			if got.Stage != tc.want {
				t.Errorf("Stage = %v; want %v", got.Stage, tc.want)
			}
		})
	}
}

func TestBusyDayShortcutSprouts(t *testing.T) {
	// No streak yet (streak 0) but a burst of commits today should still
	// count as a sprout rather than a dead seed.
	a := act(0, busyDayCommits, 0, true)
	got := Compute(a, State{}, fixedNow)
	if got.Stage != Sprout {
		t.Errorf("busy day: Stage = %v; want Sprout", got.Stage)
	}

	// Just under the busy-day threshold with no streak stays a seed.
	a2 := act(0, busyDayCommits-1, 0, true)
	got2 := Compute(a2, State{}, fixedNow)
	if got2.Stage != Seed {
		t.Errorf("below busy-day threshold: Stage = %v; want Seed", got2.Stage)
	}
}

func TestHealthByRecency(t *testing.T) {
	cases := []struct {
		name     string
		lastDays int
		want     Health
	}{
		{"today -> healthy", 0, Healthy},
		{"yesterday -> healthy", 1, Healthy},
		{"two days -> thirsty", 2, Thirsty},
		{"three days -> thirsty", 3, Thirsty},
		{"four days -> wilting", 4, Wilting},
		{"ten days -> wilting", 10, Wilting},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Give it an established streak so it's a real plant; the
			// streak value itself doesn't affect health.
			a := act(7, 10, tc.lastDays, true)
			got := Compute(a, State{}, fixedNow)
			if got.Health != tc.want {
				t.Errorf("Health = %v; want %v (days=%d)", got.Health, tc.want, tc.lastDays)
			}
			if got.DaysSinceCommit != tc.lastDays {
				t.Errorf("DaysSinceCommit = %d; want %d", got.DaysSinceCommit, tc.lastDays)
			}
		})
	}
}

func TestNoCommitsIsHealthySeed(t *testing.T) {
	a := act(0, 0, 0, false)
	got := Compute(a, State{}, fixedNow)
	if got.Stage != Seed {
		t.Errorf("Stage = %v; want Seed", got.Stage)
	}
	if got.Health != Healthy {
		t.Errorf("Health = %v; want Healthy (nothing to wilt yet)", got.Health)
	}
	if got.DaysSinceCommit != -1 {
		t.Errorf("DaysSinceCommit = %d; want -1 for no commits", got.DaysSinceCommit)
	}
}

func TestMemoryFloorHoldsWhileHealthy(t *testing.T) {
	// The plant once reached Tall. Today activity is weak (short streak ->
	// live Sprout) but the last commit was yesterday, so it's Healthy. A
	// mature plant shouldn't collapse to a sprout on one slow day: it holds
	// its remembered peak.
	a := act(1, 1, 1, true)
	st := State{HighestStage: Tall}
	got := Compute(a, st, fixedNow)
	if got.Stage != Tall {
		t.Errorf("Stage = %v; want Tall (held by memory while healthy)", got.Stage)
	}
	if got.UpdatedHighestStage != Tall {
		t.Errorf("UpdatedHighestStage = %v; want Tall", got.UpdatedHighestStage)
	}
}

func TestMemoryFloorSlipsOneStageWhenWilting(t *testing.T) {
	// Once wilting (4+ idle days), a remembered Tall plant slips exactly one
	// stage to Leafy -- neglect becomes visible, but it doesn't crater.
	a := act(0, 0, 4, true)
	st := State{HighestStage: Tall}
	got := Compute(a, st, fixedNow)
	if got.Health != Wilting {
		t.Fatalf("precondition: Health = %v; want Wilting", got.Health)
	}
	if got.Stage != Leafy {
		t.Errorf("Stage = %v; want Leafy (Tall slipped one while wilting)", got.Stage)
	}
	if got.UpdatedHighestStage != Tall {
		t.Errorf("UpdatedHighestStage = %v; want Tall (peak remembered)", got.UpdatedHighestStage)
	}
}

func TestWiltingNeverFallsBelowSprout(t *testing.T) {
	// A remembered Sprout that is now wilting must not drop to Seed: anything
	// that ever grew keeps at least a sprout.
	a := act(0, 0, 9, true)
	st := State{HighestStage: Sprout}
	got := Compute(a, st, fixedNow)
	if got.Stage != Sprout {
		t.Errorf("Stage = %v; want Sprout (floor while wilting)", got.Stage)
	}
}

func TestLiveGrowthBeatsStaleMemory(t *testing.T) {
	// Memory says Sprout, but today's activity is a booming 14-day streak.
	// Live growth should win and the peak should update to Blooming.
	a := act(14, 30, 0, true)
	st := State{HighestStage: Sprout}
	got := Compute(a, st, fixedNow)
	if got.Stage != Blooming {
		t.Errorf("Stage = %v; want Blooming (live growth exceeds memory)", got.Stage)
	}
	if got.UpdatedHighestStage != Blooming {
		t.Errorf("UpdatedHighestStage = %v; want Blooming", got.UpdatedHighestStage)
	}
}

func TestMoodPrioritizesHealthProblems(t *testing.T) {
	// Wilting mood should mention dryness regardless of stage.
	a := act(0, 0, 5, true)
	got := Compute(a, State{HighestStage: Blooming}, fixedNow)
	if got.Health != Wilting {
		t.Fatalf("precondition Health = %v; want Wilting", got.Health)
	}
	if got.Mood == "" {
		t.Fatal("Mood is empty")
	}
	if !containsAny(got.Mood, "Parched", "drooping", "dry") {
		t.Errorf("wilting Mood = %q; expected dryness language", got.Mood)
	}
}

func TestMoodCelebratesBloom(t *testing.T) {
	a := act(20, 40, 0, true)
	got := Compute(a, State{}, fixedNow)
	if got.Stage != Blooming || got.Health != Healthy {
		t.Fatalf("precondition stage=%v health=%v; want Blooming/Healthy", got.Stage, got.Health)
	}
	if !containsAny(got.Mood, "bloom", "Bloom") {
		t.Errorf("blooming Mood = %q; expected bloom language", got.Mood)
	}
}

func TestDeterministic(t *testing.T) {
	a := act(5, 8, 2, true)
	st := State{HighestStage: Leafy, BestStreak: 9}
	first := Compute(a, st, fixedNow)
	for i := 0; i < 25; i++ {
		got := Compute(a, st, fixedNow)
		if got != first {
			t.Fatalf("Compute not deterministic: run %d = %+v, first = %+v", i, got, first)
		}
	}
}

func TestClockSkewFutureCommitIsToday(t *testing.T) {
	// A commit timestamped slightly in the future (clock skew) should be
	// treated as today (0 days), not a negative age.
	a := gitstat.Activity{
		HasCommits:    true,
		TotalInWindow: 1,
		Streak:        1,
		LastCommit:    fixedNow.Add(2 * time.Hour),
	}
	got := Compute(a, State{}, fixedNow)
	if got.DaysSinceCommit != 0 {
		t.Errorf("DaysSinceCommit = %d; want 0 for slight future skew", got.DaysSinceCommit)
	}
	if got.Health != Healthy {
		t.Errorf("Health = %v; want Healthy", got.Health)
	}
}

// TestGraceShiftsHealthThresholds verifies that watering-can grace days rewind
// the effective dry spell so a plant tolerates that many extra idle days before
// turning thirsty and, later, wilting.
func TestGraceShiftsHealthThresholds(t *testing.T) {
	cases := []struct {
		name     string
		lastDays int
		grace    int
		want     Health
	}{
		// Without grace, day 2 is thirsty and day 4 wilts (see TestHealthByRecency).
		{"2 idle, 1 grace -> healthy", 2, 1, Healthy},
		{"3 idle, 1 grace -> thirsty", 3, 1, Thirsty},
		{"4 idle, 1 grace -> thirsty", 4, 1, Thirsty},
		{"5 idle, 1 grace -> wilting", 5, 1, Wilting},
		{"4 idle, 2 grace -> thirsty", 4, 2, Thirsty},
		{"3 idle, 2 grace -> healthy", 3, 2, Healthy},
		{"5 idle, 2 grace -> thirsty", 5, 2, Thirsty},
		{"6 idle, 2 grace -> wilting", 6, 2, Wilting},
		// Grace is capped at MaxGraceDays; extra grace can't help further.
		{"9 idle, 5 grace (capped at 2) -> wilting", 9, 5, Wilting},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := act(7, 10, tc.lastDays, true)
			got := Compute(a, State{GraceDays: tc.grace}, fixedNow)
			if got.Health != tc.want {
				t.Errorf("Health = %v; want %v (idle=%d grace=%d)",
					got.Health, tc.want, tc.lastDays, tc.grace)
			}
		})
	}
}

// TestGraceIsClampedAndSurfaced checks that the effective grace is clamped to
// [0, MaxGraceDays] and reported back on the PlantState for status output.
func TestGraceIsClampedAndSurfaced(t *testing.T) {
	a := act(7, 10, 2, true)

	over := Compute(a, State{GraceDays: 99}, fixedNow)
	if over.GraceDays != MaxGraceDays {
		t.Errorf("over-cap GraceDays = %d; want %d", over.GraceDays, MaxGraceDays)
	}

	neg := Compute(a, State{GraceDays: -4}, fixedNow)
	if neg.GraceDays != 0 {
		t.Errorf("negative GraceDays = %d; want 0", neg.GraceDays)
	}
}

// TestGraceNeverAffectsStreak is the anti-gaming guarantee: grace changes only
// health, never the reported streak, and never lets the stage exceed the
// remembered peak.
func TestGraceNeverAffectsStreak(t *testing.T) {
	// A wilting plant (4 idle days) with an established leafy peak: grace can
	// lift it out of wilting, but must not inflate the streak number.
	a := act(3, 5, 4, true) // streak 3 => Leafy live, 4 idle days

	bare := Compute(a, State{HighestStage: Leafy}, fixedNow)
	watered := Compute(a, State{HighestStage: Leafy, GraceDays: MaxGraceDays}, fixedNow)

	if watered.Streak != bare.Streak {
		t.Errorf("grace changed Streak: %d -> %d", bare.Streak, watered.Streak)
	}
	if watered.Streak != a.Streak {
		t.Errorf("Streak = %d; want %d (raw activity)", watered.Streak, a.Streak)
	}
	// Health improves with grace: 4 idle days is Wilting bare, but with 2 grace
	// the effective idle is 2 -> Thirsty (a real improvement, one step better).
	if bare.Health != Wilting {
		t.Errorf("bare Health = %v; want Wilting", bare.Health)
	}
	if watered.Health != Thirsty {
		t.Errorf("watered Health = %v; want Thirsty", watered.Health)
	}
	// Neither stage may exceed the remembered peak: grace never fakes growth,
	// it only changes how healthy the plant looks.
	if watered.Stage > Leafy || bare.Stage > Leafy {
		t.Errorf("grace/health must not exceed remembered peak Leafy; bare=%v watered=%v",
			bare.Stage, watered.Stage)
	}
}

func TestStageAndHealthStringsStable(t *testing.T) {
	stages := map[Stage]string{
		Seed: "seed", Sprout: "sprout", Leafy: "leafy",
		Tall: "tall", Blooming: "blooming",
	}
	for s, want := range stages {
		if got := s.String(); got != want {
			t.Errorf("Stage(%d).String() = %q; want %q", s, got, want)
		}
	}
	healths := map[Health]string{
		Healthy: "healthy", Thirsty: "thirsty", Wilting: "wilting",
	}
	for h, want := range healths {
		if got := h.String(); got != want {
			t.Errorf("Health(%d).String() = %q; want %q", h, got, want)
		}
	}
}

// containsAny reports whether s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && indexOf(s, sub) >= 0 {
			return true
		}
	}
	return false
}

// indexOf is a tiny substring search to avoid importing strings just for this.
func indexOf(s, sub string) int {
	n, m := len(s), len(sub)
	if m == 0 {
		return 0
	}
	for i := 0; i+m <= n; i++ {
		if s[i:i+m] == sub {
			return i
		}
	}
	return -1
}

// pestAct builds an Activity with an explicit revert count on top of the base
// act helper, for exercising the cosmetic pest computation.
func pestAct(streak, total, lastDaysAgo int, hasCommits bool, reverts int) gitstat.Activity {
	a := act(streak, total, lastDaysAgo, hasCommits)
	a.Reverts = reverts
	return a
}

func TestPestsSpawnFromReverts(t *testing.T) {
	// Two reverts, last commit yesterday (not a clean commit today) -> two pests.
	ps := Compute(pestAct(3, 5, 1, true, 2), State{}, fixedNow)
	if ps.Pests != 2 {
		t.Errorf("Pests = %d; want 2", ps.Pests)
	}
	// Pests must never touch growth/health.
	if ps.Stage != Leafy {
		t.Errorf("Stage = %v; want leafy (pests must not change stage)", ps.Stage)
	}
}

func TestPestsClampToMax(t *testing.T) {
	// Way more reverts than the cap, last commit yesterday.
	ps := Compute(pestAct(3, 20, 1, true, 10), State{}, fixedNow)
	if ps.Pests != MaxPests {
		t.Errorf("Pests = %d; want clamp to %d", ps.Pests, MaxPests)
	}
}

func TestPestsClearOnCleanCommitToday(t *testing.T) {
	// Reverts in the window, but a clean commit landed today (total > reverts,
	// days since commit == 0) -> pests clear.
	ps := Compute(pestAct(3, 5, 0, true, 2), State{}, fixedNow)
	if ps.Pests != 0 {
		t.Errorf("Pests = %d; want 0 after a clean healthy commit today", ps.Pests)
	}
}

func TestPestsPersistWhenTodayIsAllReverts(t *testing.T) {
	// Commit today, but every commit today was a revert (total == reverts):
	// not a "clean" day, so pests stay.
	ps := Compute(pestAct(3, 2, 0, true, 2), State{}, fixedNow)
	if ps.Pests != 2 {
		t.Errorf("Pests = %d; want 2 (no clean work today to clear them)", ps.Pests)
	}
}

func TestNoRevertsNoPests(t *testing.T) {
	ps := Compute(act(3, 5, 1, true), State{}, fixedNow)
	if ps.Pests != 0 {
		t.Errorf("Pests = %d; want 0 with no reverts", ps.Pests)
	}
}
