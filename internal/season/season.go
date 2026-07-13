// Package season derives a purely cosmetic "time of year" for the plant.
//
// A Season is a function of a reference time.Time only — no I/O, no persisted
// state — so it is trivially testable and never affects the plant's actual
// stage/health/streak. Render consults a season to pick a palette and light
// ASCII decoration (falling leaves, snowflakes, blossoms), all of which degrade
// gracefully to plain ASCII under --no-color.
package season

import (
	"strings"
	"time"
)

// Season is a coarse time-of-year bucket used purely for cosmetic dressing.
type Season int

const (
	// None disables seasonal dressing entirely (the neutral look). It is the
	// zero value so a Season{} means "no season".
	None Season = iota
	Winter
	Spring
	Summer
	Autumn
)

// String returns the lowercase season name (or "none").
func (s Season) String() string {
	switch s {
	case Winter:
		return "winter"
	case Spring:
		return "spring"
	case Summer:
		return "summer"
	case Autumn:
		return "autumn"
	default:
		return "none"
	}
}

// Names lists the selectable season names (excluding "none"), in calendar order
// starting from winter. Handy for flag help and validation messages.
func Names() []string {
	return []string{"winter", "spring", "summer", "autumn"}
}

// Parse maps a name to a Season. It accepts the canonical names plus a couple
// of friendly aliases ("fall" for autumn). The bool is false for an
// unrecognized name so callers can report a typo rather than silently default.
// "none"/"off" parse to None (true).
func Parse(name string) (Season, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "winter":
		return Winter, true
	case "spring":
		return Spring, true
	case "summer":
		return Summer, true
	case "autumn", "fall":
		return Autumn, true
	case "none", "off", "":
		return None, true
	default:
		return None, false
	}
}

// FromTime derives the Season from a reference time using meteorological-ish
// month boundaries (Northern Hemisphere):
//
//	Dec, Jan, Feb -> Winter
//	Mar, Apr, May -> Spring
//	Jun, Jul, Aug -> Summer
//	Sep, Oct, Nov -> Autumn
//
// It is pure and total: a zero time.Time still yields a valid season (its month
// is January -> Winter). It never returns None; None is reserved for the
// explicit "disable dressing" path, not for a date-derived season.
func FromTime(t time.Time) Season {
	switch t.Month() {
	case time.December, time.January, time.February:
		return Winter
	case time.March, time.April, time.May:
		return Spring
	case time.June, time.July, time.August:
		return Summer
	default: // September, October, November
		return Autumn
	}
}
