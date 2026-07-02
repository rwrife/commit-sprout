// Package cmd defines the cobra command tree for commit-sprout.
//
// For M1 the root command renders a single hard-coded ASCII seedling. M2 adds
// a hidden --activity flag that prints parsed git activity via internal/gitstat
// as a debugging bridge. Later milestones wire the plant state machine,
// rendering, and persistent state into the default flow.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/rwrife/commit-sprout/internal/gitstat"
	"github.com/rwrife/commit-sprout/internal/plant"
	"github.com/rwrife/commit-sprout/internal/render"
	"github.com/spf13/cobra"
)

// version is the build version. It is overridable at build time via:
//
//	go build -ldflags "-X github.com/rwrife/commit-sprout/cmd.version=v0.1.0"
var version = "dev"

// showActivity backs the hidden --activity debug flag (M2 bridge).
var showActivity bool

// noColor backs the --no-color flag, forcing the plain-ASCII render path.
var noColor bool

// rootCmd is the base command invoked as `commit-sprout` with no subcommand.
var rootCmd = &cobra.Command{
	Use:   "commit-sprout",
	Short: "A terminal pet that grows an ASCII plant from your git commits",
	Long: `commit-sprout reads your git history and turns "did I actually ship
anything?" into something you can see. Commit today and it sprouts; keep a
streak and it blooms; ghost your repo and it wilts.

Run with no arguments to render the current plant.`,
	// SilenceUsage/SilenceErrors keep output clean; we print errors ourselves.
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       version,
	RunE: func(cmd *cobra.Command, args []string) error {
		if showActivity {
			return printActivity(cmd)
		}
		return renderPlant(cmd)
	},
}

// renderPlant runs the full pipeline for the current repo: read git activity,
// compute the plant state, and render a framed ASCII plant to stdout. This is
// the real M4 default flow, replacing the M1 hard-coded seedling.
//
// State persistence (M5) is not wired yet, so a zero plant.State is used; the
// memory floor still behaves correctly from a fresh state, and M5 will load the
// remembered highest stage/streak here.
func renderPlant(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	act, err := gitstat.Read("", gitstat.DefaultWindowDays)
	if err != nil {
		if errors.Is(err, gitstat.ErrNotARepo) {
			_, perr := fmt.Fprintln(out,
				"Not a git repository \u2014 nothing growing here yet. "+
					"Run commit-sprout inside a repo.")
			return perr
		}
		return err
	}

	now := time.Now()
	ps := plant.Compute(act, plant.State{}, now)

	frame := render.Frame(ps, render.Options{
		Color:      useColor(cmd),
		Now:        now,
		LastCommit: act.LastCommit,
	})
	_, perr := fmt.Fprintln(out, frame)
	return perr
}

// useColor decides whether to take the color render path. Color is disabled by
// --no-color, by the NO_COLOR environment variable (any value), and whenever
// stdout is not a terminal (piped/redirected), keeping output pipe-safe.
func useColor(cmd *cobra.Command) bool {
	if noColor {
		return false
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	f, ok := cmd.OutOrStdout().(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// printActivity renders the parsed Activity for the current repo. It is a
// temporary M2 debugging aid behind the hidden --activity flag; the real
// pipeline (activity -> plant -> render) arrives in later milestones.
func printActivity(cmd *cobra.Command) error {
	act, err := gitstat.Read("", gitstat.DefaultWindowDays)
	if err != nil {
		if errors.Is(err, gitstat.ErrNotARepo) {
			_, perr := fmt.Fprintln(cmd.OutOrStdout(),
				"Not a git repository \u2014 nothing growing here yet. "+
					"Run commit-sprout inside a repo.")
			return perr
		}
		return err
	}

	out := cmd.OutOrStdout()
	if !act.HasCommits {
		_, perr := fmt.Fprintf(out,
			"No commits by %s in the last %d days \u2014 plant a seed with your first commit!\n",
			authorLabel(act.Author), act.WindowDays)
		return perr
	}

	fmt.Fprintf(out, "author:   %s\n", authorLabel(act.Author))
	fmt.Fprintf(out, "window:   last %d days\n", act.WindowDays)
	fmt.Fprintf(out, "commits:  %d\n", act.TotalInWindow)
	fmt.Fprintf(out, "streak:   %d day(s)\n", act.Streak)
	fmt.Fprintf(out, "last:     %s\n", act.LastCommit.Format("2006-01-02 15:04"))
	for _, day := range act.Days() {
		fmt.Fprintf(out, "  %s  %d\n", day, act.CommitsByDay[day])
	}
	return nil
}

// authorLabel gives a friendly stand-in when no author email is configured.
func authorLabel(author string) string {
	if author == "" {
		return "(any author)"
	}
	return author
}

// Execute runs the root command and exits non-zero on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "commit-sprout:", err)
		os.Exit(1)
	}
}

func init() {
	// Match the documented interface in README/PLAN: `--version` prints version.
	rootCmd.SetVersionTemplate("commit-sprout {{.Version}}\n")

	// --no-color forces the plain-ASCII render path (also honored via NO_COLOR
	// and automatically for non-TTY output).
	rootCmd.Flags().BoolVar(&noColor, "no-color", false, "disable color output (plain ASCII)")

	// Hidden M2 bridge: dump parsed git activity. Removed once M3+ consume it.
	rootCmd.Flags().BoolVar(&showActivity, "activity", false, "print parsed git activity (debug)")
	_ = rootCmd.Flags().MarkHidden("activity")
}
