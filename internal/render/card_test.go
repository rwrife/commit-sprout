package render

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/rwrife/commit-sprout/internal/plant"
	"github.com/rwrife/commit-sprout/internal/species"
)

// knownCardState returns a fixed, healthy blooming plant so the card output is
// deterministic for assertions.
func knownCardState() plant.PlantState {
	return plant.PlantState{
		Stage:           plant.Blooming,
		Health:          plant.Healthy,
		Streak:          7,
		DaysSinceCommit: 0,
	}
}

func TestCard_ValidSVGRootAndGlyphRows(t *testing.T) {
	ps := knownCardState()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	svg := Card(ps, CardOptions{
		Color:      false,
		Species:    species.Default,
		Now:        now,
		LastCommit: now,
	})

	// It must be well-formed XML with an <svg> root element.
	dec := xml.NewDecoder(strings.NewReader(svg))
	var rootLocal string
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok {
			rootLocal = se.Name.Local
			break
		}
	}
	if rootLocal != "svg" {
		t.Fatalf("expected <svg> root element, got %q; svg was:\n%s", rootLocal, svg)
	}

	// Fully parse to ensure the whole document is valid XML (no unescaped
	// glyphs, balanced tags).
	if err := xml.Unmarshal([]byte(svg), new(interface{})); err != nil {
		t.Fatalf("card SVG is not valid XML: %v\n%s", err, svg)
	}

	// The art rows must mirror the plain terminal art for this known state:
	// one <tspan> per art line, in order.
	art := species.ArtFor(species.Default, ps.Stage, ps.Health)
	for _, row := range strings.Split(art, "\n") {
		want := svgEscape(padRow(row))
		if !strings.Contains(svg, ">"+want+"</tspan>") {
			t.Errorf("expected art glyph row %q as a tspan in SVG output", row)
		}
	}

	// The caption block carries the stage/streak so the badge is self-describing.
	if !strings.Contains(svg, "stage:  blooming (healthy)") {
		t.Errorf("expected stage caption in card; svg:\n%s", svg)
	}
	if !strings.Contains(svg, "streak:") {
		t.Errorf("expected streak caption in card")
	}
}

func TestCard_ColorHonorsHealthTheme(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name   string
		ps     plant.PlantState
		wantHx string
	}{
		{"healthy", plant.PlantState{Stage: plant.Leafy, Health: plant.Healthy, DaysSinceCommit: 0}, cardHealthyHex},
		{"thirsty", plant.PlantState{Stage: plant.Leafy, Health: plant.Thirsty, DaysSinceCommit: 2}, cardThirstyHex},
		{"wilting", plant.PlantState{Stage: plant.Leafy, Health: plant.Wilting, DaysSinceCommit: 6}, cardWiltingHex},
		{"bloom", plant.PlantState{Stage: plant.Blooming, Health: plant.Healthy, DaysSinceCommit: 0}, cardBloomHex},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svg := Card(tc.ps, CardOptions{Color: true, Now: now, LastCommit: now})
			if !strings.Contains(svg, `fill="`+tc.wantHx+`"`) {
				t.Errorf("expected foliage fill %s for %s state", tc.wantHx, tc.name)
			}
		})
	}
}

func TestCard_NoColorIsMono(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	svg := Card(plant.PlantState{Stage: plant.Blooming, Health: plant.Healthy, DaysSinceCommit: 0},
		CardOptions{Color: false, Now: now, LastCommit: now})

	// The mono path must not emit any of the themed foliage colors.
	for _, hex := range []string{cardHealthyHex, cardThirstyHex, cardWiltingHex, cardBloomHex} {
		if strings.Contains(svg, `font-size="15" fill="`+hex+`"`) {
			t.Errorf("mono card should not use themed foliage color %s", hex)
		}
	}
	// Art should use the neutral ink instead.
	if !strings.Contains(svg, `font-size="15" fill="`+cardInk+`"`) {
		t.Errorf("mono card should use neutral ink %s for art", cardInk)
	}
}
