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
	"github.com/rwrife/commit-sprout/internal/store"
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

// noSave backs the --no-save flag, which renders without persisting state.
// Handy for read-only prompt/status contexts or when you don't want a run to
// update the remembered plant.
var noSave bool

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

// renderPlant runs the full pipeline for the current repo: load remembered
// state, read git activity, compute the plant state, render a framed ASCII
// plant to stdout, then persist the updated memory back to disk.
//
// Persistence is best-effort and never blocks rendering: a load failure falls
// back to a fresh plant, and a save failure is reported on stderr but does not
// fail the command (so a read-only home dir or full disk still shows the
// plant). The --no-save flag skips the write entirely.
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

	// Load remembered state (highest stage, best streak). A read error degrades
	// to a fresh plant but is surfaced on stderr for visibility.
	statePath, pathErr := store.DefaultPath()
	persisted := store.DefaultState()
	if pathErr == nil {
		if loaded, lerr := store.Load(statePath); lerr != nil {
			fmt.Fprintln(os.Stderr, "commit-sprout: warning:", lerr)
		} else {
			persisted = loaded
		}
	}

	now := time.Now()
	ps := plant.Compute(act, persisted.Plant(), now)

	frame := render.Frame(ps, render.Options{
		Color:      useColor(cmd),
		Now:        now,
		LastCommit: act.LastCommit,
	})
	if _, perr := fmt.Fprintln(out, frame); perr != nil {
		return perr
	}

	// Persist the updated memory. Best-effort: warn but don't fail the command.
	if !noSave && pathErr == nil {
		updated := persisted.FromPlant(ps, "", act.LastCommit)
		if serr := store.Save(statePath, updated); serr != nil {
			fmt.Fprintln(os.Stderr, "commit-sprout: warning:", serr)
		}
	}
	return nil
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

	// --no-save renders without persisting state (read-only run).
	rootCmd.Flags().BoolVar(&noSave, "no-save", false, "do not persist plant state for this run")

	// Hidden M2 bridge: dump parsed git activity. Removed once M3+ consume it.
	rootCmd.Flags().BoolVar(&showActivity, "activity", false, "print parsed git activity (debug)")
	_ = rootCmd.Flags().MarkHidden("activity")
}
