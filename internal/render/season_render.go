// Package render seasonal dressing.
//
// Seasons are cosmetic only: they layer a palette and a light ASCII decoration
// (falling leaves, snowflakes, blossoms) on top of the existing frame without
// ever touching the plant's stage, health, or streak. Everything here degrades
// gracefully — under the plain path (--no-color / NO_COLOR / non-TTY) the
// decoration is drawn as plain ASCII with no styling, and season.None is a
// clean no-op that returns the neutral frame untouched.
package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rwrife/commit-sprout/internal/season"
)

// dressArt applies a season's decoration and palette to already-rendered art.
//
// For season.None it returns art unchanged (the neutral look). Otherwise it
// prepends a thin decoration line (e.g. drifting snow or falling leaves) and,
// when color is enabled, tints that decoration with a season-appropriate color.
// The base plant art keeps its own health-based coloring done by the caller;
// dressing only adds the ambient top line so the plant's own coloring is never
// overwritten.
func dressArt(art string, s season.Season, color bool, holiday bool) string {
	deco := seasonDecoration(s, holiday)
	if deco == "" {
		return art
	}
	if color {
		deco = seasonStyle(s).Render(deco)
	}
	return deco + "\n" + art
}

// seasonDecoration returns the ambient decoration line for a season, or "" when
// there is nothing to add (season.None). When holiday is true and the season
// has a gentle holiday variant, that is used instead of the base line.
//
// Decorations are intentionally a single short ASCII line so they never disturb
// the plant's own art block width in a disruptive way and remain readable in a
// plain terminal.
func seasonDecoration(s season.Season, holiday bool) string {
	switch s {
	case season.Winter:
		if holiday {
			return "  *  .  *  .  *" // a touch more sparkle for the holidays
		}
		return "  .  *  .  *  ." // drifting snow
	case season.Spring:
		if holiday {
			return "  @  .  @  .  @" // blossoms in bloom
		}
		return "  '  ,  '  ,  '" // fresh buds
	case season.Summer:
		return "  \\  |  /  |  \\" // bright sun rays
	case season.Autumn:
		if holiday {
			return "  %  .  %  .  %" // extra falling leaves
		}
		return "  ,  .  `  .  ," // falling leaves
	default:
		return ""
	}
}

// seasonStyle picks a lipgloss color for a season's decoration. It is only
// consulted on the color path; the plain path ignores it entirely.
func seasonStyle(s season.Season) lipgloss.Style {
	switch s {
	case season.Winter:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan snow
	case season.Spring:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("13")) // pink blossoms
	case season.Summer:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // bright yellow sun
	case season.Autumn:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("208")) // orange leaves
	default:
		return lipgloss.NewStyle()
	}
}

// trimSeasonDeco is a small helper used by tests to strip a leading decoration
// line from a dressed frame, leaving the underlying neutral art for comparison.
// It removes exactly one leading line when it matches a known decoration.
func trimSeasonDeco(dressed string) string {
	idx := strings.IndexByte(dressed, '\n')
	if idx < 0 {
		return dressed
	}
	return dressed[idx+1:]
}
