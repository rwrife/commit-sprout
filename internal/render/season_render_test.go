package render

import (
	"strings"
	"testing"
	"time"

	"github.com/rwrife/commit-sprout/internal/plant"
	"github.com/rwrife/commit-sprout/internal/season"
)

// samplePlant returns a representative plant state for dressing tests.
func samplePlant() plant.PlantState {
	return plant.PlantState{
		Stage:           plant.Leafy,
		Health:          plant.Healthy,
		Streak:          5,
		DaysSinceCommit: 0,
	}
}

// TestSeasonNoneIsNeutral confirms season.None renders identically to a frame
// with no seasonal dressing — the neutral look is a true no-op.
func TestSeasonNoneIsNeutral(t *testing.T) {
	ps := samplePlant()
	now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)

	neutral := Frame(ps, Options{Now: now, LastCommit: now})
	explicitNone := Frame(ps, Options{Now: now, LastCommit: now, Season: season.None})

	if neutral != explicitNone {
		t.Fatalf("season.None changed output:\nneutral:\n%s\nnone:\n%s", neutral, explicitNone)
	}
}

// TestSeasonPreservesPlantState asserts the cosmetic-only invariant: for every
// season the underlying plant art and full caption block are byte-identical to
// the neutral render once the decoration line is stripped. Seasonal dressing
// must never alter stage/health/streak or the caption that reports them.
func TestSeasonPreservesPlantState(t *testing.T) {
	ps := samplePlant()
	now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	neutral := Frame(ps, Options{Now: now, LastCommit: now})

	for _, s := range []season.Season{season.Winter, season.Spring, season.Summer, season.Autumn} {
		dressed := Frame(ps, Options{Now: now, LastCommit: now, Season: s})
		// A season prepends exactly one decoration line to the art; strip it
		// and the rest must equal the neutral frame.
		stripped := trimSeasonDeco(dressed)
		if stripped != neutral {
			t.Errorf("season %v altered plant body:\nneutral:\n%q\nstripped:\n%q", s, neutral, stripped)
		}
	}
}

// TestSeasonPlainNoAnsi ensures the plain (no-color) path emits no ANSI escape
// codes in the seasonal decoration, so it degrades to clean ASCII.
func TestSeasonPlainNoAnsi(t *testing.T) {
	ps := samplePlant()
	now := time.Date(2026, time.December, 25, 12, 0, 0, 0, time.UTC)

	for _, s := range []season.Season{season.Winter, season.Spring, season.Summer, season.Autumn} {
		out := Frame(ps, Options{Now: now, LastCommit: now, Season: s, Color: false})
		if strings.Contains(out, "\x1b[") {
			t.Errorf("season %v plain render contains ANSI escape: %q", s, out)
		}
	}
}

// TestSeasonAddsDecoration confirms a non-None season actually adds a visible
// decoration line (i.e. the feature does something).
func TestSeasonAddsDecoration(t *testing.T) {
	ps := samplePlant()
	now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)

	neutral := Frame(ps, Options{Now: now, LastCommit: now})
	dressed := Frame(ps, Options{Now: now, LastCommit: now, Season: season.Winter})

	if !strings.HasPrefix(dressed, seasonDecoration(season.Winter, false)) {
		t.Errorf("winter frame missing decoration prefix: %q", dressed)
	}
	if len(dressed) <= len(neutral) {
		t.Errorf("dressed frame not longer than neutral (%d vs %d)", len(dressed), len(neutral))
	}
}

// TestHolidayIsOptIn verifies holiday skins only change output when explicitly
// enabled: with Holiday=false the base season decoration is used.
func TestHolidayIsOptIn(t *testing.T) {
	ps := samplePlant()
	now := time.Date(2026, time.December, 25, 12, 0, 0, 0, time.UTC)

	base := Frame(ps, Options{Now: now, LastCommit: now, Season: season.Winter, Holiday: false})
	holiday := Frame(ps, Options{Now: now, LastCommit: now, Season: season.Winter, Holiday: true})

	if base == holiday {
		t.Errorf("holiday skin did not change winter output when opted in")
	}
	if !strings.HasPrefix(base, seasonDecoration(season.Winter, false)) {
		t.Errorf("base winter should use non-holiday decoration")
	}
}
