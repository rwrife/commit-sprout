// Package render turns plant state into ASCII frames and prompt glyphs.
//
// For M1 it exposes a single hard-coded seedling. Later milestones (M4) extend
// this with art for every stage/health combination plus a color path via
// lipgloss. Art is kept here as data so the rest of the program stays pure.
package render

// seedling is the M1 hard-coded plant: a tiny sprout in a pot.
const seedling = `   \|/
    |
   _|_
  (   )
   '-'`

// Seedling returns the hard-coded ASCII seedling frame.
//
// This is the M1 placeholder render. It intentionally takes no arguments; once
// the plant state machine (M3) lands, a Frame(PlantState) function will select
// the correct art based on stage and health.
func Seedling() string {
	return seedling
}
