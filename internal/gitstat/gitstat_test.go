package gitstat

import (
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTagMetadataWindow(t *testing.T) {
	raw := strings.Join([]string{
		"commit\x00\x002026-06-30T09:00:00Z",
		"tag\x00commit\x002026-06-24T00:00:00Z",
		"tag\x00commit\x002026-06-23T23:59:59Z",
		"tag\x00commit\x002026-06-29T23:30:00-02:00",
		"commit\x00\x002026-07-01T09:00:00Z",
		"blob\x00\x00",
		"tag\x00tree\x002026-06-30T09:00:00Z",
	}, "\n")
	base := parseLog("", "me@example.com", fixedNow, 7)
	got, err := withTags(base, raw, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"2026-06-30": 3, "2026-06-24": 1}
	if got.Tags != 4 || !reflect.DeepEqual(got.TagsByDay, want) {
		t.Fatalf("tags = %d, %v; want 4, %v", got.Tags, got.TagsByDay, want)
	}
	got.Tags, got.TagsByDay = base.Tags, base.TagsByDay
	if !reflect.DeepEqual(got, base) {
		t.Fatal("tag metadata changed commit activity")
	}
	for _, bad := range []string{"commit", "tag\x00commit\x00not-a-date"} {
		if _, err := withTags(base, bad, fixedNow); err == nil {
			t.Errorf("malformed tag metadata %q did not error", bad)
		}
	}
}

func TestRunTagsLightweightAndAnnotated(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	git("init", "--quiet")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.com",
		"-c", "commit.gpgsign=false", "commit", "--quiet", "--allow-empty", "-m", "fixture")
	git("-c", "tag.gpgsign=false", "tag", "lightweight")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.com",
		"-c", "tag.gpgsign=false", "tag", "-a", "annotated", "-m", "not read by gitstat")
	raw, err := runTags(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	act, err := withTags(Activity{WindowDays: 7}, raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if act.Tags != 2 || act.TagsByDay[now.Format("2006-01-02")] != 2 {
		t.Fatalf("real tags = %+v; want two tags today", act)
	}
	if strings.Contains(raw, "not read by gitstat") {
		t.Fatal("tag reader included tag content")
	}
	filtered, err := ReadAt(dir, "nobody@example.com", 7)
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Tags != 2 || filtered.HasCommits || filtered.TotalInWindow != 0 {
		t.Fatalf("repository tags should survive the author filter: %+v", filtered)
	}
}

// fixedNow is a stable reference clock used across tests. Using UTC keeps the
// day-boundary math obvious in the fixtures below.
var fixedNow = time.Date(2026, time.June, 30, 12, 0, 0, 0, time.UTC)

// rec builds one raw `git log` record in the same shape logFormat produces:
// hash, author-date (RFC3339), email, subject — joined by NUL. It uses a
// generic non-revert subject so existing tests exercise the common case; use
// recSubject to supply a specific subject (e.g. a revert).
func rec(hash, isoDate, email string) string {
	return recSubject(hash, isoDate, email, "work commit")
}

// recSubject builds a raw record with an explicit commit subject, letting tests
// drive revert detection and other subject-sensitive behavior.
func recSubject(hash, isoDate, email, subject string) string {
	return strings.Join([]string{hash, isoDate, email, subject}, logFieldSep)
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

func TestRepoNameBaseName(t *testing.T) {
	if got := RepoName("/home/user/code/myrepo"); got != "myrepo" {
		t.Errorf("RepoName(/home/user/code/myrepo) = %q; want myrepo", got)
	}
	if got := RepoName("/home/user/code/myrepo/"); got != "myrepo" {
		t.Errorf("RepoName with trailing slash = %q; want myrepo", got)
	}
}

func TestRepoNameEmptyFallsBackToCwdBase(t *testing.T) {
	// "." and "" resolve to the current working directory's base name; we
	// don't assert the exact value (it varies) but it must be non-empty.
	for _, in := range []string{"", "."} {
		if got := RepoName(in); got == "" {
			t.Errorf("RepoName(%q) returned empty; want the cwd base name", in)
		}
	}
}

// TestParseLogCountsRevertsInWindow verifies that revert commits (subjects that
// begin with the conventional `Revert "` prefix) are counted into Reverts, and
// only when they fall inside the aggregate window.
func TestParseLogCountsRevertsInWindow(t *testing.T) {
	raw := logFixture(
		recSubject("r1", "2026-06-30T09:00:00Z", "me@example.com", `Revert "add feature X"`),
		recSubject("a1", "2026-06-30T08:00:00Z", "me@example.com", "add feature X"),
		recSubject("r2", "2026-06-29T09:00:00Z", "me@example.com", `Revert "tweak Y"`),
		// A commit that merely mentions "revert" in prose must NOT count.
		recSubject("a2", "2026-06-28T09:00:00Z", "me@example.com", "do not revert this later"),
		// A revert well outside the 7-day window is ignored by the count.
		recSubject("r3", "2026-06-01T09:00:00Z", "me@example.com", `Revert "old change"`),
	)

	act := parseLog(raw, "me@example.com", fixedNow, DefaultWindowDays)

	if act.Reverts != 2 {
		t.Errorf("Reverts = %d; want 2 (two in-window reverts, prose 'revert' excluded, old revert out of window)", act.Reverts)
	}
	if act.TotalInWindow != 4 {
		t.Errorf("TotalInWindow = %d; want 4", act.TotalInWindow)
	}
}

// TestIsRevertSubject exercises the subject-prefix heuristic directly.
func TestIsRevertSubject(t *testing.T) {
	cases := []struct {
		subject string
		want    bool
	}{
		{`Revert "add feature"`, true},
		{`  Revert "leading spaces tolerated"`, true},
		{"Reverting a change", false},
		{"revert lowercase not matched", false},
		{"add a normal thing", false},
		{"", false},
		{`this Revert "in the middle" does not count`, false},
	}
	for _, c := range cases {
		if got := isRevertSubject(c.subject); got != c.want {
			t.Errorf("isRevertSubject(%q) = %v; want %v", c.subject, got, c.want)
		}
	}
}
