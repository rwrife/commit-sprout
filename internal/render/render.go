// Package render turns plant state into ASCII frames and prompt glyphs.
//
// Art lives here as data so the rest of the program stays pure: Frame selects a
// frame by (stage x health) and wraps it in a small caption block. A color path
// is available via lipgloss (Options.Color), with a clean plain-ASCII fallback
// that is used automatically for --no-color, NO_COLOR, and non-TTY output.
//
// Frame is deterministic given its inputs (including the reference time carried
// on PlantState via DaysSinceCommit and Streak), which keeps snapshot tests
// stable.
package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/rwrife/commit-sprout/internal/plant"
)

// seedling is the original M1 hard-coded plant: a tiny sprout in a pot. It is
// retained for backward compatibility (and the M1 stability test) and is the
// same art used for the healthy Seed frame.
const seedling = `   \|/
    |
   _|_
  (   )
   '-'`

// Options controls how a frame is rendered.
type Options struct {
	// Color enables the lipgloss color path. When false (the default, and
	// what callers pick for --no-color / NO_COLOR / piped output) rendering
	// is pure plain ASCII with zero styling overhead.
	Color bool

	// Now is the reference time used to phrase the "last commit" caption
	// line (e.g. "today", "3 days ago"). The zero value falls back to
	// time.Now at call time so tests can pin it for stable snapshots.
	Now time.Time

	// LastCommit is the timestamp of the most recent commit, used for the
	// caption. The zero value means "no commits yet" and is rendered as a
	// friendly seed-planting nudge.
	LastCommit time.Time
}

// Seedling returns the hard-coded ASCII seedling frame (the healthy Seed art).
//
// It predates the state machine and takes no arguments; new code should prefer
// Frame, which selects art for any stage/health combination. Kept for
// compatibility and as a stable, dependency-free entry point.
func Seedling() string {
	return seedling
}

// Frame renders a full plant: the ASCII art for ps.Stage/ps.Health followed by
// a caption block (stage, streak, last commit / days-until-wilt). The art
// visibly differs across stages and wilt states, satisfying the M4 "done when".
//
// Rendering is deterministic given ps and opt. When opt.Color is false the
// output is plain ASCII suitable for pipes, files, and CI snapshots.
func Frame(ps plant.PlantState, opt Options) string {
	art := artFor(ps.Stage, ps.Health)
	caption := caption(ps, opt)

	var b strings.Builder
	if opt.Color {
		b.WriteString(colorizeArt(art, ps))
		b.WriteString("\n\n")
		b.WriteString(colorizeCaption(caption, ps))
	} else {
		b.WriteString(art)
		b.WriteString("\n\n")
		b.WriteString(caption)
	}
	return b.String()
}

// PromptGlyph returns a compact one-line "glyph stage" suitable for a shell
// prompt or tmux status line. It is intentionally tiny and never multi-line.
// (M6 polishes prompt mode; this is the render-side primitive it will use.)
func PromptGlyph(ps plant.PlantState) string {
	return fmt.Sprintf("%s %s", glyphFor(ps.Stage, ps.Health), ps.Stage)
}

// glyphFor maps a stage/health to a single display rune for prompt mode.
func glyphFor(stage plant.Stage, health plant.Health) string {
	if health == plant.Wilting {
		return "\u2620" // skull-ish "this plant is in trouble"
	}
	switch stage {
	case plant.Seed:
		return "."
	case plant.Sprout:
		return "\u0264" // small hooked sprout
	case plant.Leafy:
		return "\u0644" // leafy stalk
	case plant.Tall:
		return "\u04ce" // tall stem
	case plant.Blooming:
		return "\u2740" // flower
	default:
		return "?"
	}
}

// caption builds the plain-text caption block under the art. It is a stable
// contract used by snapshot tests, so keep wording changes deliberate.
func caption(ps plant.PlantState, opt Options) string {
	lines := []string{
		fmt.Sprintf("stage:  %s (%s)", ps.Stage, ps.Health),
		fmt.Sprintf("streak: %s", streakPhrase(ps.Streak)),
		fmt.Sprintf("last:   %s", lastCommitPhrase(ps, opt)),
	}
	if hint := wiltHint(ps); hint != "" {
		lines = append(lines, "        "+hint)
	}
	if ps.Mood != "" {
		lines = append(lines, "")
		lines = append(lines, ps.Mood)
	}
	return strings.Join(lines, "\n")
}

// streakPhrase renders a streak count with correct pluralization.
func streakPhrase(streak int) string {
	switch {
	case streak <= 0:
		return "no active streak"
	case streak == 1:
		return "1 day"
	default:
		return fmt.Sprintf("%d days", streak)
	}
}

// lastCommitPhrase describes when the last commit landed, in whole days.
func lastCommitPhrase(ps plant.PlantState, opt Options) string {
	if ps.DaysSinceCommit < 0 || opt.LastCommit.IsZero() {
		return "no commits yet \u2014 plant a seed with your first commit"
	}

	now := opt.Now
	if now.IsZero() {
		now = time.Now()
	}
	stamp := opt.LastCommit.Format("2006-01-02")

	switch ps.DaysSinceCommit {
	case 0:
		return fmt.Sprintf("today (%s)", stamp)
	case 1:
		return fmt.Sprintf("yesterday (%s)", stamp)
	default:
		return fmt.Sprintf("%d days ago (%s)", ps.DaysSinceCommit, stamp)
	}
}

// wiltHint returns a short "days until wilt" style nudge, or "" when healthy
// with nothing to warn about. It mirrors plant's thresholds in spirit: thirsty
// plants get a countdown, wilting plants get a water nudge.
func wiltHint(ps plant.PlantState) string {
	switch ps.Health {
	case plant.Thirsty:
		// Thirsty spans days 2-3 idle; wilting begins at day 4. So the
		// days remaining before wilt is (4 - DaysSinceCommit), floored at 1.
		remaining := 4 - ps.DaysSinceCommit
		if remaining < 1 {
			remaining = 1
		}
		if remaining == 1 {
			return "needs a commit today or it starts wilting"
		}
		return fmt.Sprintf("%d days until it wilts", remaining)
	case plant.Wilting:
		return "wilting \u2014 commit to revive it"
	default:
		return ""
	}
}

// artFor selects the ASCII frame for a stage/health combination. Wilting and
// (to a lesser degree) thirsty stages get drooped variants so the plant's
// condition is visible, not just captioned. Healthy stages use the upright art.
func artFor(stage plant.Stage, health plant.Health) string {
	byHealth, ok := frames[stage]
	if !ok {
		byHealth = frames[plant.Seed]
	}
	if art, ok := byHealth[health]; ok {
		return art
	}
	// Fall back to the healthy frame if a specific variant is missing.
	return byHealth[plant.Healthy]
}

// frames holds every stage's art keyed by health. Each frame is a fixed-size
// ASCII block; upright for Healthy, mild lean for Thirsty, and a clear droop
// for Wilting. Frames are data so new stages/variants are easy to add or edit.
var frames = map[plant.Stage]map[plant.Health]string{
	plant.Seed: {
		plant.Healthy: seedling,
		plant.Thirsty: "   .|.\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: "   .,.\n" +
			"    .\n" +
			"   _._\n" +
			"  (   )\n" +
			"   '-'",
	},
	plant.Sprout: {
		plant.Healthy: "    ,\n" +
			"   /|\n" +
			"  ' |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Thirsty: "    .\n" +
			"   \\|\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: "\n" +
			"   \\,\n" +
			"    `.\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
	},
	plant.Leafy: {
		plant.Healthy: "   \\ | /\n" +
			"    \\|/\n" +
			"   --+--\n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Thirsty: "   \\ | \n" +
			"    \\| \n" +
			"   --+  \n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Wilting: "   . . .\n" +
			"    \\ / \n" +
			"     +._ \n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
	},
	plant.Tall: {
		plant.Healthy: "   \\\\|//\n" +
			"    \\|/\n" +
			"   --+--\n" +
			"    /|\\\n" +
			"     |\n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Thirsty: "   \\\\| \n" +
			"    \\| \n" +
			"   --+- \n" +
			"    /| \n" +
			"     |\n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Wilting: "   . . .\n" +
			"    \\|/ \n" +
			"   ._+   \n" +
			"     \\\n" +
			"     |\n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
	},
	plant.Blooming: {
		plant.Healthy: "  @ \\|/ @\n" +
			"   \\\\|//\n" +
			"  @--+--@\n" +
			"    /|\\\n" +
			"     |\n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Thirsty: "  @ \\| \n" +
			"   \\\\| \n" +
			"  @--+- \n" +
			"    /| \n" +
			"     |\n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Wilting: "  * . . \n" +
			"   \\\\|/ \n" +
			"  .--+   \n" +
			"    /\\\n" +
			"     |\n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
	},
}

// --- color path (lipgloss) ---------------------------------------------------
//
// The color path is only taken when Options.Color is true. It is deliberately
// simple: a foliage color chosen by health, a neutral pot, and a dimmed
// caption. lipgloss itself also honors NO_COLOR / terminal profile, so this is
// a second layer of safety on top of the caller's plain-path decision.

var (
	healthyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))  // green
	thirstyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))  // yellow
	wiltingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))  // red
	bloomStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("13")) // magenta accents
	captionStyle = lipgloss.NewStyle().Faint(true)
)

// colorizeArt applies a foliage color to the whole frame based on health, with
// a small bloom accent for a healthy blooming plant.
func colorizeArt(art string, ps plant.PlantState) string {
	style := healthyStyle
	switch ps.Health {
	case plant.Thirsty:
		style = thirstyStyle
	case plant.Wilting:
		style = wiltingStyle
	default:
		if ps.Stage == plant.Blooming {
			style = bloomStyle
		}
	}
	return style.Render(art)
}

// colorizeCaption dims the caption block so the plant stays the focal point.
func colorizeCaption(caption string, _ plant.PlantState) string {
	return captionStyle.Render(caption)
}
