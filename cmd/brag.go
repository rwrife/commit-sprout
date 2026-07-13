// Brag subcommand: a single, standup-ready line summarizing how the plant grew
// over the recent window, driven entirely by real commit data.
//
// `commit-sprout brag` is the "brag/streak logging" energy from PLAN.md made
// glanceable: it reuses the shared read pipeline (gitstat -> plant) and adds no
// new persisted state. The whole line is produced by bragLine, a pure function
// over the activity, the freshly computed plant state, and the remembered peak
// stage / best streak, so it is trivially table-testable.
//
// Output is a single line and deliberately plain (no color), matching the
// `status` conventions so it drops cleanly into a standup note, Slack message,
// or a git post-commit hook.
package cmd

import (
	"errors"
	"fmt"

	"github.com/rwrife/commit-sprout/internal/gitstat"
	"github.com/rwrife/commit-sprout/internal/plant"
	"github.com/spf13/cobra"
)

// bragCmd implements `commit-sprout brag`.
var bragCmd = &cobra.Command{
	Use:   "brag",
	Short: "Print a one-line, standup-ready summary of the plant's recent growth",
	Long: `Print a single, copy-pasteable line summarizing how the plant grew over
the recent window (default 7 days): the stage transition (or "held steady"),
the current streak, and the number of commits in the window.

The line is plain (no color) and stable, so it drops cleanly into a standup
note, a Slack message, or a git post-commit hook. It reuses the same read
pipeline as every other command and adds no new state.`,
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return renderBrag(cmd)
	},
}

// renderBrag runs the shared pipeline and prints the brag line, then persists
// the updated plant memory like the other read commands (honoring --no-save).
func renderBrag(cmd *cobra.Command) error {
	r, err := runPipeline()
	if err != nil {
		if errors.Is(err, gitstat.ErrNotARepo) {
			return notARepoMessage(cmd)
		}
		return err
	}

	line := bragLine(r.activity, r.plant, plant.ParseStage(r.persisted.HighestStage), r.persisted.BestStreak)
	if _, perr := fmt.Fprintln(cmd.OutOrStdout(), line); perr != nil {
		return perr
	}

	persist(r)
	return nil
}

// bragLine is the pure formatter behind `brag`: given the window activity, the
// computed plant state, the previously-remembered peak stage, and the best
// streak, it returns a single standup-ready sentence. It performs no I/O and is
// deterministic, so it is exercised entirely with table-driven tests.
//
// The phrasing has four shapes:
//   - no commits ever  -> a friendly "plant a seed" nudge
//   - wilting/thirsty  -> an honest "revive it" line (dry spells don't brag)
//   - grew this window  -> "grew sprout -> leafy on an N-day streak (M commits)"
//   - held steady       -> "held steady at leafy on an N-day streak (M commits)"
func bragLine(act gitstat.Activity, ps plant.PlantState, rememberedPeak plant.Stage, bestStreak int) string {
	// Nothing to brag about yet: no commits at all in the repo.
	if !act.HasCommits || ps.DaysSinceCommit < 0 {
		return "Nothing to brag about yet \u2014 plant a seed with your first commit. \U0001F331"
	}

	window := act.WindowDays
	if window <= 0 {
		window = gitstat.DefaultWindowDays
	}
	commits := bragCommits(act.TotalInWindow, window)

	// Honest brag for a plant that has gone quiet. A wilting or thirsty plant
	// shouldn't crow about growth -- it should nudge you to revive it.
	switch ps.Health {
	case plant.Wilting:
		return fmt.Sprintf(
			"Your plant is wilting after %s quiet \u2014 time to revive it with a commit. \U0001F940",
			bragQuietDays(ps.DaysSinceCommit))
	case plant.Thirsty:
		return fmt.Sprintf(
			"Your plant is thirsty after %s quiet \u2014 a commit today keeps it green. %s",
			bragQuietDays(ps.DaysSinceCommit), bragStreakTail(ps.Streak, bestStreak))
	}

	// Healthy: describe growth over the window. Compare where the plant sits
	// now (live Stage) against the peak it had remembered before this run.
	streakPhrase := bragStreak(ps.Streak)
	if ps.Stage > rememberedPeak {
		return fmt.Sprintf(
			"This week your plant grew %s\u2192%s on %s (%s). \U0001F331",
			rememberedPeak, ps.Stage, streakPhrase, commits)
	}
	return fmt.Sprintf(
		"This week your plant held steady at %s on %s (%s). \U0001F331",
		ps.Stage, streakPhrase, commits)
}

// bragCommits pluralizes the window commit count together with the window
// length, e.g. "14 commits in 7 days" or "1 commit in 7 days".
func bragCommits(n, window int) string {
	noun := "commits"
	if n == 1 {
		noun = "commit"
	}
	return fmt.Sprintf("%d %s in %d days", n, noun, window)
}

// bragStreak renders the streak clause used in a growing/steady brag, e.g.
// "a 5-day streak" or, for a zero streak, "no active streak".
func bragStreak(streak int) string {
	switch {
	case streak <= 0:
		return "no active streak"
	case streak == 1:
		return "a 1-day streak"
	default:
		return fmt.Sprintf("a %d-day streak", streak)
	}
}

// bragStreakTail is the optional streak sentence appended to a thirsty brag. It
// leans on the best streak so a lapsed plant still carries a little pride, and
// stays quiet ("") when there is nothing worth mentioning.
func bragStreakTail(current, best int) string {
	peak := best
	if current > peak {
		peak = current
	}
	if peak <= 0 {
		return ""
	}
	if peak == 1 {
		return "(best streak: 1 day)"
	}
	return fmt.Sprintf("(best streak: %d days)", peak)
}

// bragQuietDays pluralizes the idle-day count for the thirsty/wilting phrasing,
// e.g. "1 day" or "4 days". It assumes a non-negative day count (the no-commit
// case is handled earlier).
func bragQuietDays(days int) string {
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}
