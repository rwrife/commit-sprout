package render

import (
	"strings"
	"testing"
	"time"

	"github.com/rwrife/commit-sprout/internal/plant"
)

// gp is a small helper to build a GardenPlant for tests.
func gp(label string, stage plant.Stage, health plant.Health, last time.Time) GardenPlant {
	return GardenPlant{
		Label:      label,
		State:      plant.PlantState{Stage: stage, Health: health, DaysSinceCommit: 0},
		LastCommit: last,
	}
}

func TestGardenEmptyIsFriendly(t *testing.T) {
	out := Garden(nil, Options{})
	if strings.TrimSpace(out) == "" {
		t.Fatal("Garden(nil) returned empty; expected a friendly hint")
	}
	if !strings.Contains(out, "garden") {
		t.Errorf("empty-garden message should mention the garden; got %q", out)
	}
}

func TestGardenSingleContainsLabelAndCaption(t *testing.T) {
	last := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	now := last
	out := Garden([]GardenPlant{
		gp("myrepo", plant.Sprout, plant.Healthy, last),
	}, Options{Now: now, Width: 80})

	for _, want := range []string{"myrepo", "sprout/healthy", "today"} {
		if !strings.Contains(out, want) {
			t.Errorf("single-plant garden missing %q; got:\n%s", want, out)
		}
	}
}

// TestGardenPacksMultipleOnWideRow verifies that several narrow cells share one
// row when the width allows, i.e. the label line has both repo names on it.
func TestGardenPacksMultipleOnWideRow(t *testing.T) {
	last := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	out := Garden([]GardenPlant{
		gp("aaa", plant.Sprout, plant.Healthy, last),
		gp("bbb", plant.Leafy, plant.Healthy, last),
	}, Options{Now: last, Width: 120})

	firstLine := strings.SplitN(out, "\n", 2)[0]
	if !strings.Contains(firstLine, "aaa") || !strings.Contains(firstLine, "bbb") {
		t.Errorf("expected both labels on the first row at width 120; got first line %q", firstLine)
	}
}

// TestGardenWrapsWhenNarrow verifies that a tight width forces cells onto
// separate rows (the second label appears on a later line, not the first).
func TestGardenWrapsWhenNarrow(t *testing.T) {
	last := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	out := Garden([]GardenPlant{
		gp("aaa", plant.Sprout, plant.Healthy, last),
		gp("bbb", plant.Leafy, plant.Healthy, last),
	}, Options{Now: last, Width: 12})

	firstLine := strings.SplitN(out, "\n", 2)[0]
	if strings.Contains(firstLine, "bbb") {
		t.Errorf("did not expect second label on the first row at width 12; got %q", firstLine)
	}
	if !strings.Contains(out, "bbb") {
		t.Errorf("second plant missing entirely from narrow garden:\n%s", out)
	}
}

// TestGardenRowsAreRectangular checks that every line in a packed row has the
// same visible width, which is what keeps columns aligned.
func TestGardenRowsAreRectangular(t *testing.T) {
	last := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	out := Garden([]GardenPlant{
		gp("aaa", plant.Blooming, plant.Healthy, last),
		gp("bb", plant.Seed, plant.Wilting, last),
	}, Options{Now: last, Width: 120})

	// Consider only the first row block (up to a blank separator line).
	rowBlock := out
	if idx := strings.Index(out, "\n\n"); idx >= 0 {
		rowBlock = out[:idx]
	}
	lines := strings.Split(rowBlock, "\n")
	width := visibleWidth(lines[0])
	for i, ln := range lines {
		if w := visibleWidth(ln); w != width {
			t.Errorf("row line %d width = %d; want %d (line %q)", i, w, width, ln)
		}
	}
}

// TestGardenNoWidthUsesDefault ensures Width<=0 falls back rather than piling
// everything onto one infinitely wide row: with a default of 80, three ~18-wide
// cells fit on one row, but the call must not panic and must include all three.
func TestGardenNoWidthUsesDefault(t *testing.T) {
	last := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	out := Garden([]GardenPlant{
		gp("one", plant.Sprout, plant.Healthy, last),
		gp("two", plant.Sprout, plant.Healthy, last),
		gp("three", plant.Sprout, plant.Healthy, last),
	}, Options{Now: last}) // Width omitted -> DefaultGardenWidth

	for _, want := range []string{"one", "two", "three"} {
		if !strings.Contains(out, want) {
			t.Errorf("garden missing %q at default width:\n%s", want, out)
		}
	}
}

// TestGardenColorPadsToSameWidthAsPlain guards the ANSI-aware width logic: a
// colored row must have the same *visible* geometry as the plain one, so color
// codes never break column alignment.
func TestGardenColorPadsToSameWidthAsPlain(t *testing.T) {
	last := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	plants := []GardenPlant{
		gp("aaa", plant.Tall, plant.Healthy, last),
		gp("bbb", plant.Sprout, plant.Thirsty, last),
	}
	plain := Garden(plants, Options{Now: last, Width: 120, Color: false})
	colored := Garden(plants, Options{Now: last, Width: 120, Color: true})

	plainLines := strings.Split(plain, "\n")
	colorLines := strings.Split(colored, "\n")
	if len(plainLines) != len(colorLines) {
		t.Fatalf("line count differs: plain=%d colored=%d", len(plainLines), len(colorLines))
	}
	for i := range plainLines {
		if pv, cv := visibleWidth(plainLines[i]), visibleWidth(colorLines[i]); pv != cv {
			t.Errorf("line %d visible width: plain=%d colored=%d", i, pv, cv)
		}
	}
}

func TestVisibleWidthIgnoresANSI(t *testing.T) {
	// "\x1b[2m" ... "\x1b[0m" wrap "hi" -> visible width 2.
	s := "\x1b[2mhi\x1b[0m"
	if w := visibleWidth(s); w != 2 {
		t.Errorf("visibleWidth(%q) = %d; want 2", s, w)
	}
	if w := visibleWidth("plain"); w != 5 {
		t.Errorf("visibleWidth(plain) = %d; want 5", w)
	}
}

func TestGardenLabelTruncatesLongNames(t *testing.T) {
	long := gardenLabel("a-really-long-repository-name-here")
	if r := []rune(long); len(r) > 16 {
		t.Errorf("gardenLabel too long: %d runes (%q)", len(r), long)
	}
	if !strings.HasSuffix(long, "\u2026") {
		t.Errorf("expected ellipsis on truncated label; got %q", long)
	}
	if got := gardenLabel(""); got != "(repo)" {
		t.Errorf("empty label = %q; want (repo)", got)
	}
}

func TestPackRowsRespectsWidth(t *testing.T) {
	// Three cells each 10 wide, gutter 3: at width 25 only two fit per row.
	cells := []string{
		strings.Repeat("x", 10),
		strings.Repeat("y", 10),
		strings.Repeat("z", 10),
	}
	rows := packRows(cells, 25)
	if len(rows) != 2 {
		t.Fatalf("packRows rows = %d; want 2", len(rows))
	}
	if len(rows[0]) != 2 || len(rows[1]) != 1 {
		t.Errorf("packRows layout = %v; want [2,1] cells", []int{len(rows[0]), len(rows[1])})
	}
}

func TestPackRowsOversizeCellGetsOwnRow(t *testing.T) {
	cells := []string{strings.Repeat("x", 100), "y"}
	rows := packRows(cells, 20)
	if len(rows) != 2 {
		t.Fatalf("oversize cell should still occupy its own row; rows=%d", len(rows))
	}
}
