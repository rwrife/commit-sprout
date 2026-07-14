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
	"github.com/rwrife/commit-sprout/internal/season"
	"github.com/rwrife/commit-sprout/internal/species"
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

	// Width is the terminal width (in columns) available for multi-plant
	// garden layout. It is only consulted by Garden; single-plant Frame
	// ignores it. Zero or negative means "unknown", and Garden falls back to
	// DefaultGardenWidth.
	Width int

	// Species selects which art set to render. The zero value is the default
	// species (fern), so callers that never set it get the historical art.
	Species species.Kind

	// Season selects the cosmetic seasonal dressing (palette + decoration).
	// The zero value (season.None) is a clean no-op that renders the neutral
	// look, so existing callers are unaffected. Seasonal dressing never
	// changes stage/health/streak — it is purely ambient.
	Season season.Season

	// Holiday enables gentle holiday skins layered on top of the base season.
	// It is opt-in and off by default: no surprise decorations appear beyond
	// the base season unless a caller explicitly sets this.
	Holiday bool
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
	art := species.ArtFor(opt.Species, ps.Stage, ps.Health)
	// Overlay pests on the raw art first so the (single-block) color pass wraps
	// cleanly around the whole plant; injecting glyphs after colorizeArt would
	// risk splitting its ANSI sequences across lines.
	art = withPests(art, ps)
	caption := caption(ps, opt)

	var b strings.Builder
	if opt.Color {
		b.WriteString(dressArt(colorizeArt(art, ps), opt.Season, true, opt.Holiday))
		b.WriteString("\n\n")
		b.WriteString(colorizeCaption(caption, ps))
	} else {
		b.WriteString(dressArt(art, opt.Season, false, opt.Holiday))
		b.WriteString("\n\n")
		b.WriteString(caption)
	}
	return b.String()
}

// pestGlyph is the ASCII rune used to draw a single pest (aphid/weed) on the
// plant. It is a plain, widely-available character so it renders identically
// under --no-color and on minimal terminals.
const pestGlyph = '%'

// withPests overlays ps.Pests pest glyphs onto the raw (uncolored) plant art.
// Pests are placed on the foliage: the glyph replaces a leading space on each
// of the first few non-blank art lines so the pests sit *on* the plant rather
// than floating beside it, degrading cleanly if the art is narrow. It is called
// before any color pass so a single-block colorizer wraps the whole plant
// without its ANSI sequences being split across lines. Zero pests is a no-op,
// returning the art untouched so pest-free plants are byte-for-byte unchanged.
func withPests(art string, ps plant.PlantState) string {
	if ps.Pests <= 0 {
		return art
	}

	glyph := string(pestGlyph)

	lines := strings.Split(art, "\n")
	placed := 0
	for i := 0; i < len(lines) && placed < ps.Pests; i++ {
		ln := lines[i]
		if strings.TrimSpace(ln) == "" {
			continue // skip blank lines; keep pests on actual foliage
		}
		// Replace the first space with a pest glyph so the glyph overlays the
		// plant's margin without widening the block. If the line has no leading
		// space (art flush-left), prepend instead as a graceful fallback.
		if idx := strings.IndexByte(ln, ' '); idx >= 0 {
			lines[i] = ln[:idx] + glyph + ln[idx+1:]
		} else {
			lines[i] = glyph + ln
		}
		placed++
	}
	return strings.Join(lines, "\n")
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
	if hint := pestHint(ps); hint != "" {
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

// pestHint returns a short note when the plant has cosmetic pests, explaining
// what they are and how to clear them, or "" when the plant is pest-free.
func pestHint(ps plant.PlantState) string {
	if ps.Pests <= 0 {
		return ""
	}
	noun := "pest"
	if ps.Pests != 1 {
		noun = "pests"
	}
	return fmt.Sprintf("%d %s (from reverts) \u2014 a clean commit today clears them", ps.Pests, noun)
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

// --- garden (multi-repo windowsill) -----------------------------------------
//
// Garden renders several plants side by side as a row (a "windowsill"), each
// under its repo label with a compact one-plant caption. It reuses the same
// per-stage/health art as the single-plant Frame so a garden plant looks
// identical to its solo view, just laid out in a grid.

// gardenGutter is the number of spaces placed between adjacent plants in a row.
const gardenGutter = 3

// DefaultGardenWidth is the terminal width assumed when a caller does not know
// the real one (Options.Width <= 0). 80 columns is the classic safe default and
// keeps the row from sprawling when output is piped or width detection fails.
const DefaultGardenWidth = 80

// GardenPlant is one entry in a garden: a repo's resolved plant state plus the
// label (typically the repo directory name) to caption it with.
type GardenPlant struct {
	// Label is the short name shown above the plant (e.g. the repo dir name).
	Label string

	// State is the computed plant to render for this repo.
	State plant.PlantState

	// LastCommit is the repo's most recent commit time, used for this
	// plant's compact caption. The zero value renders as "no commits yet".
	LastCommit time.Time

	// Species selects this plant's art set. The zero value is the default
	// species (fern), so existing callers are unaffected.
	Species species.Kind
}

// Garden renders plants as one or more rows of side-by-side cells, wrapping to
// the next row when the next cell would exceed opt.Width (falling back to
// DefaultGardenWidth when opt.Width <= 0). It is deterministic given its inputs.
//
// An empty plants slice yields a friendly "empty windowsill" line rather than a
// blank string, so callers can print the result unconditionally.
func Garden(plants []GardenPlant, opt Options) string {
	if len(plants) == 0 {
		return "No repos in the garden yet \u2014 add some with `commit-sprout garden --add <path>`\n" +
			"or pass repo paths: `commit-sprout garden <path> <path>`."
	}

	width := opt.Width
	if width <= 0 {
		width = DefaultGardenWidth
	}

	cells := make([]string, len(plants))
	for i, p := range plants {
		cells[i] = gardenCell(p, opt)
	}

	rows := packRows(cells, width)
	blocks := make([]string, len(rows))
	for i, row := range rows {
		blocks[i] = joinSideBySide(row, gardenGutter)
	}
	return strings.Join(blocks, "\n\n")
}

// gardenCell builds the full text block for a single garden plant: a centered
// label, the stage/health art, and a compact two-line caption. The block is a
// rectangle (all lines padded to equal width) so cells align cleanly when set
// beside one another.
func gardenCell(p GardenPlant, opt Options) string {
	art := species.ArtFor(p.Species, p.State.Stage, p.State.Health)
	art = withPests(art, p.State)
	if opt.Color {
		art = colorizeArt(art, p.State)
	}

	caption := gardenCaption(p, opt)
	if opt.Color {
		caption = captionStyle.Render(caption)
	}

	label := gardenLabel(p.Label)
	if opt.Color {
		label = captionStyle.Render(label)
	}

	block := label + "\n" + art + "\n" + caption
	return padBlock(block)
}

// gardenLabel formats the repo name shown above a garden plant, truncating
// overly long names so a single cell cannot dominate the row width.
func gardenLabel(name string) string {
	const max = 16
	name = strings.TrimSpace(name)
	if name == "" {
		name = "(repo)"
	}
	r := []rune(name)
	if len(r) > max {
		name = string(r[:max-1]) + "\u2026"
	}
	return name
}

// gardenCaption is the compact per-plant caption used in a garden: stage/health
// on one line and a short last-commit phrase on the next. It intentionally
// omits the mood and multi-line hints that the full single-plant Frame shows,
// keeping each cell narrow.
func gardenCaption(p GardenPlant, opt Options) string {
	opt.LastCommit = p.LastCommit
	last := lastCommitPhrase(p.State, opt)
	if len(last) > 22 {
		// The parenthetical date is the least essential part in a tight
		// cell; drop it when the phrase runs long.
		if idx := strings.Index(last, " ("); idx > 0 {
			last = last[:idx]
		}
	}
	return fmt.Sprintf("%s/%s\n%s", p.State.Stage, p.State.Health, last)
}

// packRows greedily groups cells into rows so that each row's total rendered
// width (cells plus gutters) stays within width. A single cell wider than width
// still occupies its own row rather than being dropped. Order is preserved.
func packRows(cells []string, width int) [][]string {
	var rows [][]string
	var cur []string
	curWidth := 0

	for _, c := range cells {
		cw := blockWidth(c)
		add := cw
		if len(cur) > 0 {
			add += gardenGutter
		}
		if len(cur) > 0 && curWidth+add > width {
			rows = append(rows, cur)
			cur = nil
			curWidth = 0
			add = cw
		}
		cur = append(cur, c)
		curWidth += add
	}
	if len(cur) > 0 {
		rows = append(rows, cur)
	}
	return rows
}

// joinSideBySide places rectangular text blocks next to each other separated by
// a gutter of spaces, top-aligned. Blocks of unequal height are bottom-padded
// with blank lines so every block contributes the same number of rows.
func joinSideBySide(blocks []string, gutter int) string {
	if len(blocks) == 0 {
		return ""
	}
	if len(blocks) == 1 {
		return blocks[0]
	}

	split := make([][]string, len(blocks))
	height := 0
	for i, b := range blocks {
		lines := strings.Split(b, "\n")
		split[i] = lines
		if len(lines) > height {
			height = len(lines)
		}
	}

	sep := strings.Repeat(" ", gutter)
	var out strings.Builder
	for row := 0; row < height; row++ {
		for i, lines := range split {
			if i > 0 {
				out.WriteString(sep)
			}
			if row < len(lines) {
				out.WriteString(lines[row])
			} else {
				// Pad missing lines to this block's width so later
				// columns still line up.
				out.WriteString(strings.Repeat(" ", blockWidth(blocks[i])))
			}
		}
		if row < height-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// padBlock right-pads every line of a block to the block's maximum line width,
// turning ragged text into a clean rectangle so it tiles correctly beside other
// blocks. Width is measured in printable columns (see visibleWidth) so ANSI
// color codes and wide runes do not skew the padding.
func padBlock(block string) string {
	lines := strings.Split(block, "\n")
	w := 0
	for _, ln := range lines {
		if vw := visibleWidth(ln); vw > w {
			w = vw
		}
	}
	for i, ln := range lines {
		if pad := w - visibleWidth(ln); pad > 0 {
			lines[i] = ln + strings.Repeat(" ", pad)
		}
	}
	return strings.Join(lines, "\n")
}

// blockWidth returns the maximum printable width across a block's lines.
func blockWidth(block string) int {
	w := 0
	for _, ln := range strings.Split(block, "\n") {
		if vw := visibleWidth(ln); vw > w {
			w = vw
		}
	}
	return w
}

// visibleWidth returns the number of printable columns in s, ignoring ANSI SGR
// escape sequences (e.g. lipgloss color codes) so colored and plain cells pad
// to the same width. It counts runes rather than bytes; the plant art is ASCII
// plus a handful of BMP symbols, all single-width, so a rune count is accurate
// here without pulling in a full east-asian-width dependency.
//
// It recognizes CSI sequences of the form ESC '[' ... final, where the final
// byte is in the range 0x40..0x7E. The '[' introducer immediately after ESC is
// consumed rather than treated as a terminator, so short codes like "\x1b[2m"
// are skipped in full.
func visibleWidth(s string) int {
	n := 0
	// esc state: 0 = normal, 1 = just saw ESC, 2 = inside CSI params.
	esc := 0
	for _, r := range s {
		switch esc {
		case 1:
			if r == '[' {
				esc = 2 // CSI; consume until a final byte
			} else {
				esc = 0 // a non-CSI escape; stop skipping
			}
			continue
		case 2:
			// Parameter/intermediate bytes are 0x20..0x3F; the sequence
			// ends at the first final byte in 0x40..0x7E.
			if r >= '@' && r <= '~' {
				esc = 0
			}
			continue
		}
		if r == '\x1b' {
			esc = 1
			continue
		}
		n++
	}
	return n
}
