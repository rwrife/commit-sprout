package season

import (
	"testing"
	"time"
)

func TestFromTimeMonthMapping(t *testing.T) {
	cases := []struct {
		month time.Month
		want  Season
	}{
		{time.December, Winter},
		{time.January, Winter},
		{time.February, Winter},
		{time.March, Spring},
		{time.April, Spring},
		{time.May, Spring},
		{time.June, Summer},
		{time.July, Summer},
		{time.August, Summer},
		{time.September, Autumn},
		{time.October, Autumn},
		{time.November, Autumn},
	}
	for _, c := range cases {
		got := FromTime(time.Date(2026, c.month, 15, 12, 0, 0, 0, time.UTC))
		if got != c.want {
			t.Errorf("FromTime(%s) = %v, want %v", c.month, got, c.want)
		}
	}
}

// TestFromTimeBoundaries pins the exact month boundaries where a season flips.
func TestFromTimeBoundaries(t *testing.T) {
	cases := []struct {
		name string
		t    time.Time
		want Season
	}{
		{"last of winter (Feb 28)", time.Date(2026, time.February, 28, 23, 59, 0, 0, time.UTC), Winter},
		{"first of spring (Mar 1)", time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), Spring},
		{"last of spring (May 31)", time.Date(2026, time.May, 31, 23, 59, 0, 0, time.UTC), Spring},
		{"first of summer (Jun 1)", time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC), Summer},
		{"last of summer (Aug 31)", time.Date(2026, time.August, 31, 23, 59, 0, 0, time.UTC), Summer},
		{"first of autumn (Sep 1)", time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC), Autumn},
		{"last of autumn (Nov 30)", time.Date(2026, time.November, 30, 23, 59, 0, 0, time.UTC), Autumn},
		{"first of winter (Dec 1)", time.Date(2026, time.December, 1, 0, 0, 0, 0, time.UTC), Winter},
	}
	for _, c := range cases {
		if got := FromTime(c.t); got != c.want {
			t.Errorf("%s: FromTime = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestFromTimeZeroTimeTotal ensures the zero time is handled (January -> Winter)
// rather than panicking or returning None.
func TestFromTimeZeroTimeTotal(t *testing.T) {
	if got := FromTime(time.Time{}); got != Winter {
		t.Errorf("FromTime(zero) = %v, want Winter", got)
	}
}

// TestFromTimeNeverNone guards the invariant that a date-derived season is
// always a real season, never the disable sentinel.
func TestFromTimeNeverNone(t *testing.T) {
	for m := time.January; m <= time.December; m++ {
		if FromTime(time.Date(2026, m, 15, 0, 0, 0, 0, time.UTC)) == None {
			t.Fatalf("FromTime(%s) returned None", m)
		}
	}
}

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Season
		ok   bool
	}{
		{"winter", Winter, true},
		{"Spring", Spring, true},
		{"  SUMMER ", Summer, true},
		{"autumn", Autumn, true},
		{"fall", Autumn, true},
		{"none", None, true},
		{"off", None, true},
		{"", None, true},
		{"bogus", None, false},
	}
	for _, c := range cases {
		got, ok := Parse(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("Parse(%q) = (%v, %v), want (%v, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestStringRoundTrip(t *testing.T) {
	for _, s := range []Season{Winter, Spring, Summer, Autumn, None} {
		got, ok := Parse(s.String())
		if !ok || got != s {
			t.Errorf("round trip %v -> %q -> (%v, %v)", s, s.String(), got, ok)
		}
	}
}

func TestNames(t *testing.T) {
	names := Names()
	if len(names) != 4 {
		t.Fatalf("Names() len = %d, want 4", len(names))
	}
	for _, n := range names {
		if _, ok := Parse(n); !ok {
			t.Errorf("Names() contains unparseable %q", n)
		}
	}
}
