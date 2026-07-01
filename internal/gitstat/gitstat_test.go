package gitstat

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// fixedNow is a stable reference clock used across tests. Using UTC keeps the
// day-boundary math obvious in the fixtures below.
var fixedNow = time.Date(2026, time.June, 30, 12, 0, 0, 0, time.UTC)

// rec builds one raw `git log` record in the same shape logFormat produces:
// hash, author-date (RFC3339), email — joined by NUL.
func rec(hash, isoDate, email string) string {
	return strings.Join([]string{hash, isoDate, email}, logFieldSep)
}

// logFixture joins records with newlines, mimicking `git log` stdout.
func logFixture(records ...string) string {
	return strings.Join(records, "\n")
}

func TestParseLogEmptyMeansNoActivity(t *testing.T) {
	act := parseLog("", "me@example.com", fixedNow, DefaultWindowDays)

	if act.HasCommits {
		t.Errorf("HasCommits = true; want false for empty log")
	}
	if !act.LastCommit.IsZero() {
		t.Errorf("LastCommit = %v; want zero time", act.LastCommit)
	}
	if act.TotalInWindow != 0 {
		t.Errorf("TotalInWindow = %d; want 0", act.TotalInWindow)
	}
	if act.Streak != 0 {
		t.Errorf("Streak = %d; want 0", act.Streak)
	}
	if len(act.CommitsByDay) != 0 {
		t.Errorf("CommitsByDay = %v; want empty", act.CommitsByDay)
	}
	if act.Author != "me@example.com" {
		t.Errorf("Author = %q; want preserved", act.Author)
	}
	if act.WindowDays != DefaultWindowDays {
		t.Errorf("WindowDays = %d; want %d", act.WindowDays, DefaultWindowDays)
	}
}

func TestParseLogCountsAndLastCommit(t *testing.T) {
	raw := logFixture(
		rec("a1", "2026-06-30T09:30:00Z", "me@example.com"),
		rec("a2", "2026-06-30T08:00:00Z", "me@example.com"),
		rec("b1", "2026-06-29T18:00:00Z", "me@example.com"),
		rec("c1", "2026-06-28T11:00:00Z", "me@example.com"),
	)

	act := parseLog(raw, "me@example.com", fixedNow, DefaultWindowDays)

	if !act.HasCommits {
		t.Fatal("HasCommits = false; want true")
	}
	if act.TotalInWindow != 4 {
		t.Errorf("TotalInWindow = %d; want 4", act.TotalInWindow)
	}
	wantLast := time.Date(2026, time.June, 30, 9, 30, 0, 0, time.UTC)
	if !act.LastCommit.Equal(wantLast) {
		t.Errorf("LastCommit = %v; want %v", act.LastCommit, wantLast)
	}

	wantByDay := map[string]int{
		"2026-06-30": 2,
		"2026-06-29": 1,
		"2026-06-28": 1,
	}
	if !reflect.DeepEqual(act.CommitsByDay, wantByDay) {
		t.Errorf("CommitsByDay = %v; want %v", act.CommitsByDay, wantByDay)
	}
}

func TestParseLogStreakIncludesToday(t *testing.T) {
	// Commits today, yesterday, and the day before -> streak of 3.
	raw := logFixture(
		rec("a1", "2026-06-30T09:00:00Z", "me@example.com"),
		rec("b1", "2026-06-29T09:00:00Z", "me@example.com"),
		rec("c1", "2026-06-28T09:00:00Z", "me@example.com"),
	)

	act := parseLog(raw, "me@example.com", fixedNow, DefaultWindowDays)

	if act.Streak != 3 {
		t.Errorf("Streak = %d; want 3", act.Streak)
	}
}

func TestParseLogStreakSurvivesMissingToday(t *testing.T) {
	// Nothing today, but yesterday + the day before -> streak stays at 2
	// (today is not yet "lost").
	raw := logFixture(
		rec("b1", "2026-06-29T09:00:00Z", "me@example.com"),
		rec("c1", "2026-06-28T09:00:00Z", "me@example.com"),
	)

	act := parseLog(raw, "me@example.com", fixedNow, DefaultWindowDays)

	if act.Streak != 2 {
		t.Errorf("Streak = %d; want 2", act.Streak)
	}
}

func TestParseLogStreakBreaksOnFullEmptyDay(t *testing.T) {
	// Last commit was two days ago (28th); both today and yesterday are
	// empty -> streak broken.
	raw := logFixture(
		rec("c1", "2026-06-28T09:00:00Z", "me@example.com"),
		rec("c2", "2026-06-27T09:00:00Z", "me@example.com"),
	)

	act := parseLog(raw, "me@example.com", fixedNow, DefaultWindowDays)

	if act.Streak != 0 {
		t.Errorf("Streak = %d; want 0 (gap of a full day breaks streak)", act.Streak)
	}
	// The historical activity should still be reflected in the aggregates.
	if act.TotalInWindow != 2 {
		t.Errorf("TotalInWindow = %d; want 2", act.TotalInWindow)
	}
}

func TestParseLogStreakStopsAtGap(t *testing.T) {
	// Today + yesterday present, then a gap, then older commits. Streak
	// should count only the contiguous run (2), not the older block.
	raw := logFixture(
		rec("a1", "2026-06-30T09:00:00Z", "me@example.com"),
		rec("b1", "2026-06-29T09:00:00Z", "me@example.com"),
		// gap on the 28th
		rec("d1", "2026-06-27T09:00:00Z", "me@example.com"),
	)

	act := parseLog(raw, "me@example.com", fixedNow, DefaultWindowDays)

	if act.Streak != 2 {
		t.Errorf("Streak = %d; want 2 (streak stops at the 28th gap)", act.Streak)
	}
}

func TestParseLogMultipleCommitsSameDayCountOnceForStreak(t *testing.T) {
	raw := logFixture(
		rec("a1", "2026-06-30T18:00:00Z", "me@example.com"),
		rec("a2", "2026-06-30T09:00:00Z", "me@example.com"),
		rec("a3", "2026-06-30T08:00:00Z", "me@example.com"),
		rec("b1", "2026-06-29T09:00:00Z", "me@example.com"),
	)

	act := parseLog(raw, "me@example.com", fixedNow, DefaultWindowDays)

	if act.Streak != 2 {
		t.Errorf("Streak = %d; want 2", act.Streak)
	}
	if got := act.CommitsByDay["2026-06-30"]; got != 3 {
		t.Errorf("CommitsByDay[today] = %d; want 3", got)
	}
}

func TestParseLogExcludesCommitsOutsideWindow(t *testing.T) {
	// With a 3-day window and now=June 30, the window covers the 28th-30th.
	// The commit on the 25th must be excluded from window aggregates, but
	// may still set LastCommit only if it is the newest (it is not here).
	raw := logFixture(
		rec("a1", "2026-06-30T09:00:00Z", "me@example.com"),
		rec("old", "2026-06-25T09:00:00Z", "me@example.com"),
	)

	act := parseLog(raw, "me@example.com", fixedNow, 3)

	if act.TotalInWindow != 1 {
		t.Errorf("TotalInWindow = %d; want 1 (old commit excluded)", act.TotalInWindow)
	}
	if _, ok := act.CommitsByDay["2026-06-25"]; ok {
		t.Errorf("CommitsByDay unexpectedly contains out-of-window day")
	}
	if got := len(act.CommitsByDay); got != 1 {
		t.Errorf("len(CommitsByDay) = %d; want 1", got)
	}
}

func TestParseLogSkipsMalformedLines(t *testing.T) {
	raw := strings.Join([]string{
		rec("a1", "2026-06-30T09:00:00Z", "me@example.com"),
		"this line has no separators and bad data",
		rec("bad", "not-a-date", "me@example.com"),
		"", // blank line
		rec("b1", "2026-06-29T09:00:00Z", "me@example.com"),
	}, "\n")

	act := parseLog(raw, "me@example.com", fixedNow, DefaultWindowDays)

	if act.TotalInWindow != 2 {
		t.Errorf("TotalInWindow = %d; want 2 (malformed lines skipped)", act.TotalInWindow)
	}
	if act.Streak != 2 {
		t.Errorf("Streak = %d; want 2", act.Streak)
	}
}

func TestParseLogRespectsTimezone(t *testing.T) {
	// A commit timestamped 2026-06-30T01:00:00+02:00 is 2026-06-29T23:00Z.
	// When the reference clock is in UTC, that commit belongs to the 29th.
	raw := logFixture(
		rec("tz", "2026-06-30T01:00:00+02:00", "me@example.com"),
	)

	act := parseLog(raw, "me@example.com", fixedNow, DefaultWindowDays)

	if got := act.CommitsByDay["2026-06-29"]; got != 1 {
		t.Errorf("CommitsByDay[2026-06-29] = %d; want 1 (commit normalized to UTC day)", got)
	}
	if _, ok := act.CommitsByDay["2026-06-30"]; ok {
		t.Errorf("commit incorrectly attributed to 2026-06-30")
	}
}

func TestParseLogTrailingNewlineHandled(t *testing.T) {
	// `git log` output may or may not end in a newline; both must parse the
	// same.
	body := logFixture(
		rec("a1", "2026-06-30T09:00:00Z", "me@example.com"),
		rec("b1", "2026-06-29T09:00:00Z", "me@example.com"),
	)

	withNL := parseLog(body+"\n", "me@example.com", fixedNow, DefaultWindowDays)
	withoutNL := parseLog(body, "me@example.com", fixedNow, DefaultWindowDays)

	if withNL.TotalInWindow != withoutNL.TotalInWindow {
		t.Errorf("trailing newline changed count: %d vs %d",
			withNL.TotalInWindow, withoutNL.TotalInWindow)
	}
	if withNL.TotalInWindow != 2 {
		t.Errorf("TotalInWindow = %d; want 2", withNL.TotalInWindow)
	}
}

func TestActivityDaysSorted(t *testing.T) {
	raw := logFixture(
		rec("a1", "2026-06-28T09:00:00Z", "me@example.com"),
		rec("b1", "2026-06-30T09:00:00Z", "me@example.com"),
		rec("c1", "2026-06-29T09:00:00Z", "me@example.com"),
	)

	act := parseLog(raw, "me@example.com", fixedNow, DefaultWindowDays)
	got := act.Days()
	want := []string{"2026-06-28", "2026-06-29", "2026-06-30"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Days() = %v; want %v", got, want)
	}
}

// TestParseLogLargeWindowStreak guards a longer contiguous streak across a
// window boundary: streak counting is not bounded by the window, only the
// aggregates are.
func TestParseLogLargeWindowStreak(t *testing.T) {
	// Build 10 consecutive days of commits ending today, but only a 3-day
	// window. Streak should be 10; TotalInWindow should be 3.
	var records []string
	for i := 0; i < 10; i++ {
		day := fixedNow.AddDate(0, 0, -i).Format("2006-01-02")
		records = append(records, rec(
			fmt.Sprintf("h%d", i),
			day+"T09:00:00Z",
			"me@example.com",
		))
	}
	raw := logFixture(records...)

	act := parseLog(raw, "me@example.com", fixedNow, 3)

	if act.Streak != 10 {
		t.Errorf("Streak = %d; want 10 (streak ignores window)", act.Streak)
	}
	if act.TotalInWindow != 3 {
		t.Errorf("TotalInWindow = %d; want 3", act.TotalInWindow)
	}
}
