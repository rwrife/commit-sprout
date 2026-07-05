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
	if line := statusGrace(ps); line != "" {
		fmt.Fprintf(out, "grace:   %s\n", line)
	}
	fmt.Fprintf(out, "health:  %s\n", statusWilt(ps))
	if when := nextWiltDate(ps, r.activity.LastCommit, r.now); when != "" {
		fmt.Fprintf(out, "wilts:   %s\n", when)
	}

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

// statusGrace reports how many watering-can grace days are currently propping
// the plant up, or "" when none are in effect (so the line is omitted). It
// makes the escape-hatch visible without cluttering the common no-grace case.
func statusGrace(ps plant.PlantState) string {
	switch {
	case ps.GraceDays <= 0:
		return ""
	case ps.GraceDays == 1:
		return "1 day (watered)"
	default:
		return fmt.Sprintf("%d days (watered)", ps.GraceDays)
	}
}

// effectiveIdle is the idle-day count the health thresholds actually see: raw
// days since the last commit, rewound by any watering-can grace, floored at 0.
// It is the single source of truth shared by statusWilt and nextWiltDate so the
// text and the date can never disagree.
func effectiveIdle(ps plant.PlantState) int {
	d := ps.DaysSinceCommit - ps.GraceDays
	if d < 0 {
		d = 0
	}
	return d
}

// statusWilt turns the health state into a one-line, days-until-wilt nudge that
// mirrors the render caption's spirit. Healthy plants report how many idle days
// they can still absorb before turning thirsty. Watering-can grace is folded in
// via effectiveIdle, so a watered plant honestly reports the extra runway it
// bought rather than the raw calendar gap.
func statusWilt(ps plant.PlantState) string {
	switch ps.Health {
	case plant.Wilting:
		return "wilting \u2014 commit to revive it"
	case plant.Thirsty:
		// Thirsty spans effective days 2-3 idle; wilting begins at day 4.
		remaining := 4 - effectiveIdle(ps)
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
		cushion := 2 - effectiveIdle(ps)
		if cushion < 1 {
			cushion = 1
		}
		if cushion == 1 {
			return "healthy \u2014 commit today to stay ahead of thirst"
		}
		return fmt.Sprintf("healthy \u2014 %d days of cushion before it gets thirsty", cushion)
	}
}

// nextWiltDate estimates the calendar date on which the plant will begin to
// wilt if no further commits (or waterings) land, accounting for any grace
// already in effect. It returns "" when there is nothing to project (no commits
// yet) or when the plant is already wilting. Wilting begins once effective idle
// days exceed wiltingAfterDays (3), i.e. on the 4th effective idle day.
func nextWiltDate(ps plant.PlantState, last time.Time, now time.Time) string {
	if ps.DaysSinceCommit < 0 || last.IsZero() {
		return ""
	}
	if ps.Health == plant.Wilting {
		return ""
	}
	if now.IsZero() {
		now = time.Now()
	}
	// Days from now until the first wilting day: the plant wilts when
	// effectiveIdle reaches 4, so it has (4 - effectiveIdle) safe days left,
	// and wilting lands the day after the last safe day.
	daysLeft := 4 - effectiveIdle(ps)
	if daysLeft < 1 {
		daysLeft = 1
	}
	wiltDay := now.AddDate(0, 0, daysLeft)
	return wiltDay.Format("2006-01-02")
}
