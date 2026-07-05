// Package cmd defines the cobra command tree for commit-sprout.
//
// The root command renders the full ASCII plant for the current repo. Two
// focused subcommands ride on the same pipeline: `status` prints a compact,
// plain-text summary (streak, last commit, days-until-wilt) and `prompt`
// (also reachable as the `--prompt` flag on the root) emits a single-line
// glyph for shell prompts, tmux, and starship.
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
//
// GoReleaser sets it automatically from the git tag (see .goreleaser.yaml).
var version = "dev"

// showActivity backs the hidden --activity debug flag (M2 bridge).
var showActivity bool

// noColor backs the --no-color flag, forcing the plain-ASCII render path.
var noColor bool

// noSave backs the --no-save flag, which renders without persisting state.
// Handy for read-only prompt/status contexts or when you don't want a run to
// update the remembered plant.
var noSave bool

// promptMode backs the --prompt flag on the root command, a convenience alias
// for the `prompt` subcommand so it drops cleanly into a shell prompt string.
var promptMode bool

// rootCmd is the base command invoked as `commit-sprout` with no subcommand.
var rootCmd = &cobra.Command{
	Use:   "commit-sprout",
	Short: "A terminal pet that grows an ASCII plant from your git commits",
	Long: `commit-sprout reads your git history and turns "did I actually ship
anything?" into something you can see. Commit today and it sprouts; keep a
streak and it blooms; ghost your repo and it wilts.

Run with no arguments to render the current plant. Use "status" for a compact
text summary or "prompt" (or --prompt) for a one-line glyph you can embed in a
shell prompt, tmux, or starship.`,
	// SilenceUsage/SilenceErrors keep output clean; we print errors ourselves.
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       version,
	RunE: func(cmd *cobra.Command, args []string) error {
		if showActivity {
			return printActivity(cmd)
		}
		if promptMode {
			return renderPrompt(cmd)
		}
		return renderPlant(cmd)
	},
}

// pipelineResult bundles everything the render/status/prompt paths need out of
// one pass over the repo: the raw activity, the loaded persisted state (and its
// path), and the computed plant state as of `now`.
type pipelineResult struct {
	activity  gitstat.Activity
	persisted store.State
	statePath string
	pathErr   error
	plant     plant.PlantState
	now       time.Time
}

// runPipeline performs the shared read half of every command: read git
// activity, load remembered state, and compute the plant state. It never
// writes; callers decide whether to persist afterward via persist().
//
// It returns gitstat.ErrNotARepo unwrapped so callers can special-case the
// "not a repo" message in their own voice.
func runPipeline() (pipelineResult, error) {
	act, err := gitstat.Read("", gitstat.DefaultWindowDays)
	if err != nil {
		return pipelineResult{}, err
	}

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

	return pipelineResult{
		activity:  act,
		persisted: persisted,
		statePath: statePath,
		pathErr:   pathErr,
		plant:     ps,
		now:       now,
	}, nil
}

// persist folds the computed plant state back into the remembered file, unless
// --no-save was set or the state path could not be resolved. It is best-effort:
// a write failure is surfaced on stderr but never fails the command, so a
// read-only home dir or full disk still lets the plant render.
func persist(r pipelineResult) {
	if noSave || r.pathErr != nil {
		return
	}
	updated := r.persisted.FromPlant(r.plant, "", r.activity.LastCommit)
	if serr := store.Save(r.statePath, updated); serr != nil {
		fmt.Fprintln(os.Stderr, "commit-sprout: warning:", serr)
	}
}

// notARepoMessage prints the friendly "nothing growing here" line and returns
// any write error. It centralizes the wording shared by every command.
func notARepoMessage(cmd *cobra.Command) error {
	_, perr := fmt.Fprintln(cmd.OutOrStdout(),
		"Not a git repository \u2014 nothing growing here yet. "+
			"Run commit-sprout inside a repo.")
	return perr
}

// renderPlant runs the full pipeline for the current repo and renders a framed
// ASCII plant to stdout, then persists the updated memory back to disk.
func renderPlant(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	r, err := runPipeline()
	if err != nil {
		if errors.Is(err, gitstat.ErrNotARepo) {
			return notARepoMessage(cmd)
		}
		return err
	}

	frame := render.Frame(r.plant, render.Options{
		Color:      useColor(cmd),
		Now:        r.now,
		LastCommit: r.activity.LastCommit,
	})
	if _, perr := fmt.Fprintln(out, frame); perr != nil {
		return perr
	}

	persist(r)
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
			return notARepoMessage(cmd)
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
	rootCmd.PersistentFlags().BoolVar(&noSave, "no-save", false, "do not persist plant state for this run")

	// --prompt is a convenience alias for the `prompt` subcommand so the glyph
	// drops straight into a shell prompt string.
	rootCmd.Flags().BoolVar(&promptMode, "prompt", false, "emit a one-line glyph for shell prompt / tmux / starship")

	// Hidden M2 bridge: dump parsed git activity. Removed once M3+ consume it.
	rootCmd.Flags().BoolVar(&showActivity, "activity", false, "print parsed git activity (debug)")
	_ = rootCmd.Flags().MarkHidden("activity")

	// Subcommands (M6): status + prompt ride the shared pipeline.
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(promptCmd)
}
