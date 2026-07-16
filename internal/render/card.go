// SVG card rendering: turn the current plant into a self-contained, embeddable
// SVG so people can drop a commit-sprout badge into their README. The card
// mirrors the terminal look — the same per-stage/health ASCII art as monospace
// <tspan> rows plus a stage/streak caption — so the SVG and the shell view are
// recognizably the same plant, just screenshot-free and shareable.
//
// The SVG is dependency-free (no external fonts/assets, no <image>): it uses a
// generic monospace font family and inlines every glyph as text, so it renders
// identically wherever it is embedded (GitHub, a static site, a raw viewer).
package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/rwrife/commit-sprout/internal/plant"
	"github.com/rwrife/commit-sprout/internal/species"
)

// CardOptions controls how a plant is rendered to an SVG card.
type CardOptions struct {
	// Color enables the themed color path: foliage takes the same
	// health-driven color used in the terminal render (green / yellow / red,
	// with a magenta bloom accent). When false, the whole card is mono
	// (a single ink color on a light background), mirroring --no-color.
	Color bool

	// Species selects which art set to draw. The zero value is the default
	// species, matching the terminal render.
	Species species.Kind

	// Now is the reference time used to phrase the caption's last-commit line
	// ("today", "3 days ago"). The zero value falls back to time.Now.
	Now time.Time

	// LastCommit is the timestamp of the most recent commit, used for the
	// caption. The zero value renders as the friendly "no commits yet" nudge.
	LastCommit time.Time
}

// SVG glyph metrics. These are chosen so a generic monospace font lays out
// cleanly: cellW/cellH are the advance width and line height in user units, and
// the viewBox is sized from the art/caption dimensions plus padding.
const (
	cardCellW    = 9.6  // monospace advance width per column
	cardCellH    = 18.0 // line height per row
	cardPadX     = 16.0 // left/right padding
	cardPadY     = 16.0 // top/bottom padding
	cardFontPx   = 15.0 // art font size
	cardCapPx    = 12.0 // caption font size
	cardCapCellH = 15.0 // caption line height
)

// Card palette (SVG-friendly hex, distinct from the terminal's ANSI indices but
// chosen to read the same: healthy green, thirsty amber, wilting red, bloom
// magenta, dim caption ink).
const (
	cardBG         = "#fbfdf7"
	cardBorder     = "#d7e2c8"
	cardInk        = "#2f3a2a"
	cardCaptionInk = "#6b7566"
	cardHealthyHex = "#2e7d32"
	cardThirstyHex = "#b8860b"
	cardWiltingHex = "#c0392b"
	cardBloomHex   = "#a83fa8"
)

// cardArtColor picks the foliage color for the color path, mirroring
// colorizeArt's health/bloom logic.
func cardArtColor(ps plant.PlantState) string {
	switch ps.Health {
	case plant.Thirsty:
		return cardThirstyHex
	case plant.Wilting:
		return cardWiltingHex
	default:
		if ps.Stage == plant.Blooming {
			return cardBloomHex
		}
		return cardHealthyHex
	}
}

// Card renders the plant state to a complete, standalone SVG document string.
//
// The art rows mirror the terminal frame's plain art (with pests overlaid), and
// the caption reuses the same stage/streak/last-commit block. Color honors the
// theme when opt.Color is true; otherwise the card is mono. The output is a full
// <?xml?>-prefixed <svg> document suitable for writing to a .svg file or
// embedding via <img>.
func Card(ps plant.PlantState, opt CardOptions) string {
	// Reuse the exact art the terminal would draw (plain path + pests) so the
	// card and shell views are the same plant.
	art := withPests(species.ArtFor(opt.Species, ps.Stage, ps.Health), ps)
	artRows := strings.Split(art, "\n")

	// Reuse the shared caption block for parity with the terminal render. We
	// pass a plain Options so the caption phrasing (last-commit "today"/"N days
	// ago", wilt/pest hints, mood) matches exactly.
	cap := caption(ps, Options{Now: opt.Now, LastCommit: opt.LastCommit})
	capRows := strings.Split(cap, "\n")

	// Dimensions: width is the widest of art / caption; height is stacked rows
	// plus padding and a small gap between the art and caption blocks.
	artCols := maxRuneWidth(artRows)
	capCols := maxRuneWidth(capRows)
	cols := artCols
	if capCols > cols {
		cols = capCols
	}

	artH := float64(len(artRows)) * cardCellH
	gap := 10.0
	capH := float64(len(capRows)) * cardCapCellH
	width := cardPadX*2 + float64(cols)*cardCellW
	height := cardPadY*2 + artH + gap + capH

	artFill := cardInk
	if opt.Color {
		artFill = cardArtColor(ps)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?>`+"\n")
	fmt.Fprintf(&b,
		`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" `+
			`viewBox="0 0 %.0f %.0f" role="img" aria-label="commit-sprout plant: %s, %s">`+"\n",
		width, height, width, height,
		svgEscape(ps.Stage.String()), svgEscape(ps.Health.String()))

	// Background + subtle border so the badge reads as a card on any page.
	fmt.Fprintf(&b,
		`  <rect x="0.5" y="0.5" width="%.0f" height="%.0f" rx="8" `+
			`fill="%s" stroke="%s"/>`+"\n",
		width-1, height-1, cardBG, cardBorder)

	// Art block: one <text> with a <tspan> per row so columns line up under a
	// monospace font. x is reset per row and dy advances the line.
	artBaseY := cardPadY + cardFontPx
	fmt.Fprintf(&b,
		`  <text font-family="ui-monospace,'Cascadia Code','DejaVu Sans Mono',Consolas,monospace" `+
			`font-size="%.0f" fill="%s" xml:space="preserve">`+"\n",
		cardFontPx, artFill)
	for i, row := range artRows {
		y := artBaseY + float64(i)*cardCellH
		fmt.Fprintf(&b, `    <tspan x="%.1f" y="%.1f">%s</tspan>`+"\n",
			cardPadX, y, svgEscape(padRow(row)))
	}
	fmt.Fprintf(&b, "  </text>\n")

	// Caption block: dimmed ink, smaller font, below the art.
	capBaseY := cardPadY + artH + gap + cardCapPx
	fmt.Fprintf(&b,
		`  <text font-family="ui-monospace,'Cascadia Code','DejaVu Sans Mono',Consolas,monospace" `+
			`font-size="%.0f" fill="%s" xml:space="preserve">`+"\n",
		cardCapPx, cardCaptionInk)
	for i, row := range capRows {
		y := capBaseY + float64(i)*cardCapCellH
		fmt.Fprintf(&b, `    <tspan x="%.1f" y="%.1f">%s</tspan>`+"\n",
			cardPadX, y, svgEscape(row))
	}
	fmt.Fprintf(&b, "  </text>\n")

	fmt.Fprintf(&b, "</svg>\n")
	return b.String()
}

// padRow renders a blank art row as a single space so its <tspan> is non-empty
// (some renderers collapse empty tspans, which would drop the visual line).
func padRow(row string) string {
	if row == "" {
		return " "
	}
	return row
}

// maxRuneWidth returns the widest row in runes, used to size the viewBox.
func maxRuneWidth(rows []string) int {
	max := 0
	for _, r := range rows {
		if n := len([]rune(r)); n > max {
			max = n
		}
	}
	return max
}

// svgEscape escapes the five XML special characters so arbitrary art/caption
// text (including the '&', '<', '>' that can appear in glyphs) stays valid XML.
func svgEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(s)
}
