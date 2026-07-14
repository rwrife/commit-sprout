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
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/rwrife/commit-sprout/internal/gitstat"
	"github.com/rwrife/commit-sprout/internal/plant"
	"github.com/rwrife/commit-sprout/internal/render"
	"github.com/rwrife/commit-sprout/internal/season"
	"github.com/rwrife/commit-sprout/internal/species"
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

// speciesFlag backs the --species flag: the chosen plant species (fern /
// cactus / bonsai / sunflower). Empty means "unset", in which case the saved
// default from state (or the built-in default species) is used. When set to a
// valid name it both renders as that species and is remembered as the new
// default for future flag-less runs.
var speciesFlag string

// seasonFlag backs the --season flag: force a specific seasonal dressing
// (winter / spring / summer / autumn) instead of deriving it from the current
// date. Empty means "derive from now". It is cosmetic only and never persisted.
var seasonFlag string

// noSeason backs the --no-season flag, which disables seasonal dressing
// entirely for a neutral look.
var noSeason bool

// holidaySkin backs the --holiday flag, an opt-in that layers gentle holiday
// decorations on top of the base season. Off by default so no surprise
// decorations appear beyond the base season.
var holidaySkin bool

// noPests backs the --no-pests flag, which disables the cosmetic pest overlay
// (aphids/weeds spawned by revert commits) entirely for people who prefer a
// plant that never shows negative signals. It only suppresses the visual +
// caption; it never affects stage, streak, or health.
var noPests bool

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
	species   species.Kind
	season    season.Season
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

	// Resolve the species: an explicit --species wins, otherwise fall back to
	// the saved default, otherwise the built-in default. An unrecognized flag
	// value is reported so a typo is not silently ignored.
	sp := species.Default
	if saved, ok := species.Parse(persisted.Species); ok {
		sp = saved
	}
	if speciesFlag != "" {
		chosen, ok := species.Parse(speciesFlag)
		if !ok {
			return pipelineResult{}, fmt.Errorf("unknown species %q (choose one of: %s)",
				speciesFlag, strings.Join(species.Names(), ", "))
		}
		sp = chosen
		// Remember the newly chosen species as the default for later runs.
		persisted.Species = sp.String()
	}

	// Species can widen how long the plant tolerates a dry spell (the cactus
	// twist). Growth is unaffected; only wilt thresholds move.
	tune := plant.DefaultHealthTuning()
	if st, ok := species.TuningFor(sp); ok {
		tune = plant.HealthTuning{
			ThirstyAfterDays: st.ThirstyAfterDays,
			WiltingAfterDays: st.WiltingAfterDays,
		}
	}

	ps := plant.ComputeWith(act, persisted.Plant(now, act.LastCommit), now, tune)

	// --no-pests suppresses the cosmetic pest overlay without touching any of
	// the growth/health signals: pests are purely visual, so zeroing the count
	// here is the whole opt-out.
	if noPests {
		ps.Pests = 0
	}

	// Resolve the cosmetic season: --no-season disables it, an explicit
	// --season forces one (reporting typos), otherwise derive it from now.
	// This is purely ambient and never persisted.
	sea := season.FromTime(now)
	if seasonFlag != "" {
		chosen, ok := season.Parse(seasonFlag)
		if !ok {
			return pipelineResult{}, fmt.Errorf("unknown season %q (choose one of: %s)",
				seasonFlag, strings.Join(season.Names(), ", "))
		}
		sea = chosen
	}
	if noSeason {
		sea = season.None
	}

	return pipelineResult{
		activity:  act,
		persisted: persisted,
		statePath: statePath,
		pathErr:   pathErr,
		plant:     ps,
		now:       now,
		species:   sp,
		season:    sea,
	}, nil
}

// recomputeWith re-runs the pure plant computation for an already-read pipeline
// result but with a different persisted State (e.g. after watering). It reuses
// the same activity and reference clock so only the state-derived inputs (like
// grace days) change, giving callers an updated PlantState without touching git
// again.
func recomputeWith(r pipelineResult, updated store.State) plant.PlantState {
	return plant.Compute(r.activity, updated.Plant(r.now, r.activity.LastCommit), r.now)
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
		Species:    r.species,
		Season:     r.season,
		Holiday:    holidaySkin,
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

	// --species picks the plant's art set (and, for the cactus, its drought
	// tolerance). A chosen species is remembered as the default for later runs.
	rootCmd.PersistentFlags().StringVar(&speciesFlag, "species", "",
		"plant species: "+strings.Join(species.Names(), " / ")+" (remembered as default)")

	// --season forces a cosmetic season; --no-season disables seasonal dressing;
	// --holiday opts into gentle holiday skins. All are ambient only.
	rootCmd.PersistentFlags().StringVar(&seasonFlag, "season", "",
		"force seasonal dressing: "+strings.Join(season.Names(), " / ")+" (default: derived from date)")
	rootCmd.PersistentFlags().BoolVar(&noSeason, "no-season", false, "disable seasonal dressing (neutral look)")
	rootCmd.PersistentFlags().BoolVar(&holidaySkin, "holiday", false, "opt into gentle holiday skins layered on the season")

	// --no-pests disables the cosmetic pest overlay (reverts spawn aphids you
	// clear with a clean commit). Visual/moral only; never affects growth.
	rootCmd.PersistentFlags().BoolVar(&noPests, "no-pests", false, "disable cosmetic pests (aphids/weeds from reverts)")

	// Hidden M2 bridge: dump parsed git activity. Removed once M3+ consume it.
	rootCmd.Flags().BoolVar(&showActivity, "activity", false, "print parsed git activity (debug)")
	_ = rootCmd.Flags().MarkHidden("activity")

	// Subcommands (M6): status + prompt ride the shared pipeline.
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(promptCmd)
	rootCmd.AddCommand(waterCmd)
	rootCmd.AddCommand(bragCmd)
}
