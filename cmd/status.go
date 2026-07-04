// Status subcommand: a compact, plain-text summary of the plant for the current
// repo. Where the root command draws the ASCII creature, `status` answers the
// practical questions at a glance -- what stage, how long the streak, when the
// last commit landed, and how many days remain before the plant starts to wilt.
//
// Output is deliberately plain and stable (no color, script-friendly) so it can
// be parsed or dropped into other tooling. It shares the read pipeline with the
// root command via runPipeline and, like every command, persists the plant's
// memory afterward unless --no-save is set.
package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/rwrife/commit-sprout/internal/gitstat"
	"github.com/rwrife/commit-sprout/internal/plant"
	"github.com/spf13/cobra"
)

// statusCmd implements `commit-sprout status`.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show streak, last commit, and days until the plant wilts",
	Long: `Print a compact, plain-text summary of the plant for the current repo:
its stage and health, the current streak, when the last commit landed, and how
many days remain before it starts to wilt.

The output is plain (no color) and stable, so it is easy to read at a glance or
parse from a script.`,
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return renderStatus(cmd)
	},
}

// renderStatus runs the shared pipeline and prints the status block.
func renderStatus(cmd *cobra.Command) error {
	r, err := runPipeline()
	if err != nil {
		if errors.Is(err, gitstat.ErrNotARepo) {
			return notARepoMessage(cmd)
		}
		return err
	}

	out := cmd.OutOrStdout()
	ps := r.plant

	fmt.Fprintf(out, "stage:   %s (%s)\n", ps.Stage, ps.Health)
	fmt.Fprintf(out, "streak:  %s\n", statusStreak(ps.Streak, r.persisted.BestStreak))
	fmt.Fprintf(out, "last:    %s\n", statusLastCommit(ps, r.activity.LastCommit, r.now))
	fmt.Fprintf(out, "health:  %s\n", statusWilt(ps))

	persist(r)
	return nil
}

// statusStreak renders the current streak plus the remembered best, so the
// summary carries a little brag energy without needing the full render caption.
func statusStreak(current, best int) string {
	cur := streakCount(current)
	if best > current {
		return fmt.Sprintf("%s (best: %s)", cur, streakCount(best))
	}
	return cur
}

// streakCount pluralizes a day count, matching the render package's wording.
func streakCount(n int) string {
	switch {
	case n <= 0:
		return "none"
	case n == 1:
		return "1 day"
	default:
		return fmt.Sprintf("%d days", n)
	}
}

// statusLastCommit phrases the most recent commit in whole days, with the date.
func statusLastCommit(ps plant.PlantState, last time.Time, now time.Time) string {
	if ps.DaysSinceCommit < 0 || last.IsZero() {
		return "no commits yet \u2014 plant a seed with your first commit"
	}
	if now.IsZero() {
		now = time.Now()
	}
	stamp := last.Format("2006-01-02")
	switch ps.DaysSinceCommit {
	case 0:
		return fmt.Sprintf("today (%s)", stamp)
	case 1:
		return fmt.Sprintf("yesterday (%s)", stamp)
	default:
		return fmt.Sprintf("%d days ago (%s)", ps.DaysSinceCommit, stamp)
	}
}

// statusWilt turns the health state into a one-line, days-until-wilt nudge that
// mirrors the render caption's spirit. Healthy plants report how many idle days
// they can still absorb before turning thirsty.
func statusWilt(ps plant.PlantState) string {
	switch ps.Health {
	case plant.Wilting:
		return "wilting \u2014 commit to revive it"
	case plant.Thirsty:
		// Thirsty spans days 2-3 idle; wilting begins at day 4.
		remaining := 4 - ps.DaysSinceCommit
		if remaining < 1 {
			remaining = 1
		}
		if remaining == 1 {
			return "thirsty \u2014 commit today or it starts wilting"
		}
		return fmt.Sprintf("thirsty \u2014 %d days until it wilts", remaining)
	default:
		// Healthy: report the cushion before it turns thirsty (day 2 idle).
		if ps.DaysSinceCommit < 0 {
			return "healthy"
		}
		cushion := 2 - ps.DaysSinceCommit
		if cushion < 1 {
			cushion = 1
		}
		if cushion == 1 {
			return "healthy \u2014 commit today to stay ahead of thirst"
		}
		return fmt.Sprintf("healthy \u2014 %d days of cushion before it gets thirsty", cushion)
	}
}
