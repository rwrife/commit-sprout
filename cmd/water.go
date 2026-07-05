// Water subcommand: buy the plant a grace day so a deliberate gap (a weekend,
// PTO, a conference) doesn't wilt it. Watering is the honest escape hatch to
// the wilt mechanic -- it shifts the health thresholds back by a bounded number
// of idle days without ever touching the growth stage or the streak count, so
// it can nudge the plant's *health* but can never fake *progress*.
//
// Guardrails (all enforced in internal/store.Water) keep it from being gamed:
//
//   - Idempotent per day: watering twice on the same date is a no-op.
//   - Capped: at most plant.MaxGraceDays of grace can be in effect at once.
//   - Self-resetting: a fresh commit prunes prior waterings, so grace only ever
//     covers the current dry spell and cannot be banked across commits.
package cmd

import (
	"errors"
	"fmt"

	"github.com/rwrife/commit-sprout/internal/gitstat"
	"github.com/rwrife/commit-sprout/internal/store"
	"github.com/spf13/cobra"
)

// waterCmd implements `commit-sprout water`.
var waterCmd = &cobra.Command{
	Use:   "water",
	Short: "Buy the plant one grace day before it wilts (for weekends / PTO)",
	Long: `Record a "watering" for today, buying the plant one extra idle day before
it turns thirsty and, later, wilts. Use it for deliberate gaps -- a weekend, a
day off, a conference -- when you know you won't commit but don't want the plant
to droop.

Watering is intentionally hard to abuse: it is limited to one grace day per
calendar day, capped at a small maximum at any one time, and a fresh commit
clears it. It only affects the plant's health, never its growth stage or streak,
so it can keep a plant perky through a planned break without faking that you
shipped anything.`,
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWater(cmd)
	},
}

// runWater loads state, applies a watering scoped to the current dry spell, and
// persists the result (unless --no-save). It prints what happened and the plant
// state after watering.
func runWater(cmd *cobra.Command) error {
	r, err := runPipeline()
	if err != nil {
		if errors.Is(err, gitstat.ErrNotARepo) {
			return notARepoMessage(cmd)
		}
		return err
	}

	out := cmd.OutOrStdout()

	updated, res := r.persisted.Water(r.now, r.activity.LastCommit)

	if !res.Applied {
		// Nothing to persist (state is unchanged), so just report why.
		fmt.Fprintf(out, "watering can: %s.\n", res.Reason)
		fmt.Fprintf(out, "grace:   %s\n", graceSummary(res.Grace))
		fmt.Fprintf(out, "health:  %s\n", statusWilt(r.plant))
		return nil
	}

	// Recompute the plant with the new grace in effect so the post-water
	// summary reflects reality.
	wateredPlant := recomputeWith(r, updated)

	fmt.Fprintln(out, "💧 Watered. The plant can coast one more day.")
	fmt.Fprintf(out, "grace:   %s\n", graceSummary(res.Grace))
	fmt.Fprintf(out, "health:  %s\n", statusWilt(wateredPlant))

	if noSave {
		fmt.Fprintln(out,
			"note: --no-save is set, so this watering was not persisted "+
				"(it won't count on the next run).")
		return nil
	}

	if serr := store.Save(r.statePath, updated); serr != nil {
		return fmt.Errorf("water: saving state: %w", serr)
	}
	return nil
}

// graceSummary phrases the effective grace-day count for the status block.
func graceSummary(grace int) string {
	switch {
	case grace <= 0:
		return "none"
	case grace == 1:
		return "1 grace day in effect"
	default:
		return fmt.Sprintf("%d grace days in effect", grace)
	}
}
