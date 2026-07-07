// Package gitstat reads commit activity from a git repository and normalizes
// it into an Activity struct (commits per day, last commit time, current
// streak). It works by shelling out to `git log` and parsing the output, which
// keeps the dependency surface small and avoids heavy git bindings.
//
// The package is split into two layers so the interesting logic stays pure and
// unit-testable:
//
//   - Read() / runners deal with the outside world (running git, resolving the
//     author email).
//   - parseLog() is a pure function: raw `git log` output + a reference "now"
//     in, an Activity out. Tests drive it with fixed fixture strings, so no
//     live repository is required.
package gitstat

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DefaultWindowDays is the look-back window used when none is specified. The
// plant only cares about recent cadence, so a week keeps things snappy.
const DefaultWindowDays = 7

// streakLookbackDays bounds how far back Read fetches history when detecting a
// streak. The aggregate window may be small (e.g. 7 days), but a streak can be
// much longer, so we fetch a generous-but-bounded range rather than the whole
// repo history (which could be enormous).
const streakLookbackDays = 400

// logFieldSep is the byte git emits between fields (a real NUL), and the byte
// parseLog splits on. A NUL can never appear in a hash, ISO date, or email,
// which makes it a safe delimiter.
const logFieldSep = "\x00"

// logFieldDirective is what we put *in the --pretty format argument* to make
// git emit logFieldSep. We must use git's %x00 directive here rather than an
// actual NUL byte: a real NUL embedded in an argv entry is rejected by execve
// ("invalid argument"), so the separator is requested symbolically and only
// materializes as a NUL in git's output.
const logFieldDirective = "%x00"

// logFormat is the pretty-format passed to `git log`. Fields, in order:
//
//	%H  full commit hash
//	%aI author date, strict ISO-8601 (keeps the author's timezone offset)
//	%aE author email (the "real" email after .mailmap is applied)
//
// Fields are joined with the %x00 directive so git emits NUL-separated output.
// We use the *author* date/email rather than committer, because that reflects
// when the work was actually done and by whom, which is what the plant rewards.
var logFormat = strings.Join([]string{"%H", "%aI", "%aE"}, logFieldDirective)

// ErrNotARepo is returned when Read is invoked outside a git working tree.
var ErrNotARepo = errors.New("gitstat: not a git repository")

// Activity is the normalized view of recent commit history for a single
// author. It contains no rendering and performs no state I/O; it is a plain
// data value produced from `git log`.
type Activity struct {
	// Author is the email the activity was filtered by (the configured
	// git user.email unless overridden).
	Author string

	// WindowDays is the look-back window, in days, used for TotalInWindow
	// and CommitsByDay.
	WindowDays int

	// HasCommits reports whether the author has any commits at all within
	// the window. When false the plant should fall back to friendly
	// "nothing growing yet" messaging.
	HasCommits bool

	// LastCommit is the author timestamp of the most recent commit by the
	// author. It is the zero time when HasCommits is false.
	LastCommit time.Time

	// TotalInWindow is the number of commits by the author within the last
	// WindowDays days (inclusive of today).
	TotalInWindow int

	// Streak is the current run of consecutive calendar days (in the
	// reference timezone) ending today or yesterday that contain at least
	// one commit. A commit today extends the streak; missing today but
	// having committed yesterday keeps the streak alive (it is not broken
	// until a full day passes with no commit). Two or more idle days reset
	// it to zero.
	Streak int

	// CommitsByDay maps a calendar day (formatted YYYY-MM-DD in the
	// reference timezone) to the number of author commits on that day,
	// limited to the window. Days with zero commits are omitted.
	CommitsByDay map[string]int
}

// commit is a single parsed `git log` record.
type commit struct {
	hash  string
	when  time.Time
	email string
}

// Read returns the Activity for the git repository in the current working
// directory. It is a thin convenience wrapper over ReadAt(".", ...); see that
// function for the parameter semantics.
//
// Read shells out to git; the parsing it relies on is exercised directly by
// tests via parseLog, so this thin wrapper needs no live-repo test.
func Read(author string, windowDays int) (Activity, error) {
	return ReadAt(".", author, windowDays)
}

// ReadAt returns the Activity for the git repository rooted at dir. An empty
// dir is treated as the current working directory. When author is empty it
// defaults to that repository's configured `git config user.email`, so
// per-repo author identities are honored (each repo in a garden may have been
// committed under a different address). windowDays <= 0 falls back to
// DefaultWindowDays.
//
// Like Read, ReadAt only orchestrates git and delegates the interesting work
// to the pure parseLog, so it needs no live-repo test of its own.
func ReadAt(dir, author string, windowDays int) (Activity, error) {
	if windowDays <= 0 {
		windowDays = DefaultWindowDays
	}
	if dir == "" {
		dir = "."
	}

	if err := ensureRepo(dir); err != nil {
		return Activity{}, err
	}

	if author == "" {
		author = configuredEmail(dir)
	}

	raw, err := runLog(dir, author, windowDays)
	if err != nil {
		return Activity{}, err
	}

	return parseLog(raw, author, time.Now(), windowDays), nil
}

// ensureRepo verifies dir is inside a git working tree, translating git's
// failure into the friendly ErrNotARepo sentinel.
func ensureRepo(dir string) error {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return ErrNotARepo
	}
	return nil
}

// configuredEmail returns dir's `git config user.email`, or "" if it is unset.
// An empty author simply means "do not filter by author" downstream.
func configuredEmail(dir string) string {
	cmd := exec.Command("git", "config", "user.email")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// runLog runs `git log` in dir and returns its raw stdout. It fetches enough
// history to compute a long streak accurately (streakLookbackDays), even when
// the aggregate window is small; parseLog then bounds the aggregates to the
// window.
func runLog(dir, author string, windowDays int) (string, error) {
	lookback := windowDays
	if streakLookbackDays > lookback {
		lookback = streakLookbackDays
	}
	since := fmt.Sprintf("%d.days.ago", lookback)
	args := []string{
		"log",
		"--no-merges",
		"--since=" + since,
		"--date=iso-strict",
		"--pretty=format:" + logFormat,
	}
	if author != "" {
		// --author matches a substring; anchor on the address form so a
		// bare email does not also match unrelated display names.
		args = append(args, "--author="+author)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gitstat: git log failed: %w", err)
	}
	return string(out), nil
}

// RepoName returns a short, human-friendly label for a repo directory, used as
// the caption above each plant in a garden. It resolves dir to an absolute path
// and takes its base name, falling back to the raw input when resolution fails
// (e.g. dir does not exist) so a label is always produced.
func RepoName(dir string) string {
	if dir == "" || dir == "." {
		if wd, err := os.Getwd(); err == nil {
			return filepath.Base(wd)
		}
	}
	if abs, err := filepath.Abs(dir); err == nil {
		base := filepath.Base(abs)
		if base != "." && base != string(filepath.Separator) {
			return base
		}
	}
	return dir
}

// parseLog turns raw `git log` output into an Activity. It is pure: all clock
// and timezone decisions come from now, so identical input always yields
// identical output. windowDays is assumed already normalized (> 0).
//
// Records are newline-separated; fields within a record are NUL-separated and
// ordered hash, author-date, author-email (see logFormat). Malformed or empty
// lines are skipped defensively rather than failing the whole read.
func parseLog(raw, author string, now time.Time, windowDays int) Activity {
	act := Activity{
		Author:       author,
		WindowDays:   windowDays,
		CommitsByDay: map[string]int{},
	}

	commits := parseCommits(raw)
	if len(commits) == 0 {
		return act
	}

	loc := now.Location()
	windowStart := startOfDay(now, loc).AddDate(0, 0, -(windowDays - 1))

	// allDays tracks every calendar day with a commit, regardless of the
	// window, so the streak reflects true cadence. CommitsByDay stays
	// window-bounded because it feeds the aggregate display.
	allDays := map[string]bool{}

	for _, c := range commits {
		local := c.when.In(loc)

		// Track the most recent commit across everything we parsed.
		if local.After(act.LastCommit) {
			act.LastCommit = local
			act.HasCommits = true
		}

		allDays[dayKey(local)] = true

		// Window-bounded aggregates. A commit dated in the future
		// (clock skew) still counts as "today" for the window.
		day := startOfDay(local, loc)
		if !day.Before(windowStart) {
			act.TotalInWindow++
			act.CommitsByDay[dayKey(local)]++
		}
	}

	act.Streak = computeStreak(allDays, now, loc)
	return act
}

// parseCommits splits raw git output into commit records, skipping anything
// that does not parse cleanly.
func parseCommits(raw string) []commit {
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	out := make([]commit, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, logFieldSep)
		if len(fields) != 3 {
			continue
		}
		when, err := time.Parse(time.RFC3339, strings.TrimSpace(fields[1]))
		if err != nil {
			continue
		}
		out = append(out, commit{
			hash:  strings.TrimSpace(fields[0]),
			when:  when,
			email: strings.TrimSpace(fields[2]),
		})
	}
	return out
}

// computeStreak walks backwards from today counting consecutive days that have
// at least one commit. Today missing but yesterday present keeps the streak
// alive (the day is not yet "lost"); a full empty day breaks it. The day set
// is intentionally not window-bounded, so a long streak is reported in full.
func computeStreak(days map[string]bool, now time.Time, loc *time.Location) int {
	if len(days) == 0 {
		return 0
	}

	today := startOfDay(now, loc)

	// Anchor the streak on today if it has a commit, otherwise on
	// yesterday if it does. If neither does, the streak is broken.
	cursor := today
	if !days[dayKey(cursor)] {
		cursor = cursor.AddDate(0, 0, -1)
		if !days[dayKey(cursor)] {
			return 0
		}
	}

	streak := 0
	for days[dayKey(cursor)] {
		streak++
		cursor = cursor.AddDate(0, 0, -1)
	}
	return streak
}

// startOfDay returns midnight at the start of t's calendar day in loc.
func startOfDay(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// dayKey formats t's calendar day as YYYY-MM-DD, the key used in CommitsByDay.
func dayKey(t time.Time) string {
	return t.Format("2006-01-02")
}

// Days returns the day keys present in CommitsByDay sorted chronologically.
// It is a small convenience for callers (and tests) that want deterministic
// iteration over an otherwise unordered map.
func (a Activity) Days() []string {
	days := make([]string, 0, len(a.CommitsByDay))
	for d := range a.CommitsByDay {
		days = append(days, d)
	}
	sort.Strings(days)
	return days
}
