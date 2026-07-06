// Garden subcommand: a multi-repo "windowsill" that renders one small plant per
// repository, side by side, so you can glance at cadence across several
// projects at once. It is deliberately a read-only view -- it never persists
// per-repo plant memory -- because the single-repo commands already own that
// state and a garden is a dashboard, not another writer.
//
// Repos are resolved in priority order:
//
//  1. explicit path arguments (`commit-sprout garden ~/a ~/b`), else
//  2. a one-level scan of a parent dir (`commit-sprout garden --scan ~/code`), else
//  3. the saved garden set persisted in the state file.
//
// The saved set is managed with `--add`, `--remove`, and `--list`, which map
// onto small helpers in internal/store. Per-repo author filtering is preserved:
// each repo defaults to its own `git config user.email` unless --author pins a
// single address across the whole garden.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/rwrife/commit-sprout/internal/gitstat"
	"github.com/rwrife/commit-sprout/internal/plant"
	"github.com/rwrife/commit-sprout/internal/render"
	"github.com/rwrife/commit-sprout/internal/store"
	"github.com/spf13/cobra"
)

// Garden flag backing vars.
var (
	// gardenScan scans a parent directory one level deep for git repos.
	gardenScan string
	// gardenAdd adds the given paths (or the current dir when none) to the
	// saved garden set and exits.
	gardenAdd bool
	// gardenRemove removes the given paths from the saved garden set and exits.
	gardenRemove bool
	// gardenList prints the saved garden set and exits.
	gardenList bool
	// gardenAuthor pins a single author email across every repo, overriding
	// each repo's configured user.email. Empty means per-repo default.
	gardenAuthor string
	// gardenWindow overrides the activity look-back window (days).
	gardenWindow int
)

// gardenCmd implements `commit-sprout garden`.
var gardenCmd = &cobra.Command{
	Use:   "garden [path...]",
	Short: "Render a row of plants, one per repo (a multi-repo windowsill)",
	Long: `Show several repositories as a row of small plants, so you can see your
commit cadence across projects at a glance.

Which repos are shown is resolved in this order:
  1. any paths you pass as arguments, otherwise
  2. the directory given to --scan (scanned one level deep for git repos), otherwise
  3. your saved garden set.

Manage the saved set:
  commit-sprout garden --add ~/code/foo ~/code/bar   # remember these repos
  commit-sprout garden --add                         # remember the current repo
  commit-sprout garden --remove ~/code/bar           # forget one
  commit-sprout garden --list                        # show the saved set

Each repo keeps its own author identity by default (its git config user.email);
pass --author to score every repo against one address instead.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGarden(cmd, args)
	},
}

// runGarden dispatches the management flags first (--add/--remove/--list, which
// mutate or print the saved set and return), then renders the resolved garden.
func runGarden(cmd *cobra.Command, args []string) error {
	switch {
	case gardenList:
		return runGardenList(cmd)
	case gardenAdd:
		return runGardenAdd(cmd, args)
	case gardenRemove:
		return runGardenRemove(cmd, args)
	}
	return renderGarden(cmd, args)
}

// loadGardenState loads the persisted state (best-effort, like the main
// pipeline) so the saved garden set and the --add/--remove/--list flows share
// one loader. A path resolution failure is fatal for the management flows since
// they must write, so it is surfaced to the caller.
func loadGardenState() (store.State, string, error) {
	path, err := store.DefaultPath()
	if err != nil {
		return store.DefaultState(), "", err
	}
	st, lerr := store.Load(path)
	if lerr != nil {
		fmt.Fprintln(os.Stderr, "commit-sprout: warning:", lerr)
	}
	return st, path, nil
}

// runGardenList prints the saved garden set, one path per line, or a friendly
// hint when it is empty.
func runGardenList(cmd *cobra.Command) error {
	st, _, err := loadGardenState()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if len(st.GardenRepos) == 0 {
		fmt.Fprintln(out, "No repos saved in the garden yet.")
		fmt.Fprintln(out, "Add some with: commit-sprout garden --add <path> [<path>...]")
		return nil
	}
	fmt.Fprintf(out, "Saved garden (%d repo%s):\n", len(st.GardenRepos), plural(len(st.GardenRepos)))
	for _, p := range st.GardenRepos {
		fmt.Fprintf(out, "  %s\n", p)
	}
	return nil
}

// runGardenAdd adds paths (defaulting to the current directory when none are
// given) to the saved garden set. Non-repo paths are rejected up front so the
// saved set stays meaningful. With --no-save the change is computed and
// reported but not written.
func runGardenAdd(cmd *cobra.Command, args []string) error {
	st, path, err := loadGardenState()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()

	paths := args
	if len(paths) == 0 {
		paths = []string{"."}
	}

	// Validate every path is a git repo before saving; a typo shouldn't
	// silently pollute the saved set.
	var valid []string
	for _, p := range paths {
		if !isRepo(p) {
			fmt.Fprintf(out, "skip: %s is not a git repository\n", p)
			continue
		}
		valid = append(valid, p)
	}
	if len(valid) == 0 {
		return fmt.Errorf("garden: no valid repositories to add")
	}

	updated, added := st.AddGardenRepos(valid...)
	if added == 0 {
		fmt.Fprintln(out, "Nothing new to add \u2014 those repos are already in the garden.")
		return nil
	}

	if noSave {
		fmt.Fprintf(out, "Would add %d repo%s (not saved: --no-save).\n", added, plural(added))
		return nil
	}
	if serr := store.Save(path, updated); serr != nil {
		return fmt.Errorf("garden: saving state: %w", serr)
	}
	fmt.Fprintf(out, "Added %d repo%s to the garden (now %d total).\n",
		added, plural(added), len(updated.GardenRepos))
	return nil
}

// runGardenRemove drops the given paths from the saved garden set.
func runGardenRemove(cmd *cobra.Command, args []string) error {
	st, path, err := loadGardenState()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if len(args) == 0 {
		return fmt.Errorf("garden: --remove needs at least one path")
	}

	updated, removed := st.RemoveGardenRepos(args...)
	if removed == 0 {
		fmt.Fprintln(out, "Nothing removed \u2014 none of those paths were in the garden.")
		return nil
	}
	if noSave {
		fmt.Fprintf(out, "Would remove %d repo%s (not saved: --no-save).\n", removed, plural(removed))
		return nil
	}
	if serr := store.Save(path, updated); serr != nil {
		return fmt.Errorf("garden: saving state: %w", serr)
	}
	fmt.Fprintf(out, "Removed %d repo%s from the garden (%d remaining).\n",
		removed, plural(removed), len(updated.GardenRepos))
	return nil
}

// renderGarden resolves the repo list, reads each repo's activity, computes its
// plant, and prints the side-by-side windowsill. It never persists plant state
// (a garden is a read-only glance). Repos that cannot be read are reported on
// stderr and skipped so one bad path doesn't sink the whole view.
func renderGarden(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()

	repos, err := resolveGardenRepos(args)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Fprintln(out, "No repos to show. Pass paths, use --scan <dir>, or save some with --add.")
		return nil
	}

	window := gardenWindow
	if window <= 0 {
		window = gitstat.DefaultWindowDays
	}

	now := time.Now()
	plants := make([]render.GardenPlant, 0, len(repos))
	skipped := 0
	for _, dir := range repos {
		act, rerr := gitstat.ReadAt(dir, gardenAuthor, window)
		if rerr != nil {
			fmt.Fprintf(os.Stderr, "commit-sprout: skip %s: %v\n", dir, rerr)
			skipped++
			continue
		}
		// Read-only: start from a fresh (unremembered) plant so the garden
		// reflects live activity without needing per-repo saved state.
		ps := plant.Compute(act, store.DefaultState().Plant(now, act.LastCommit), now)
		plants = append(plants, render.GardenPlant{
			Label:      gitstat.RepoName(dir),
			State:      ps,
			LastCommit: act.LastCommit,
		})
	}

	if len(plants) == 0 {
		fmt.Fprintln(out, "No readable git repositories in the garden.")
		return nil
	}

	frame := render.Garden(plants, render.Options{
		Color: useColor(cmd),
		Now:   now,
		Width: terminalWidth(cmd),
	})
	if _, perr := fmt.Fprintln(out, frame); perr != nil {
		return perr
	}
	if skipped > 0 {
		fmt.Fprintf(out, "\n(%d repo%s skipped \u2014 see messages above.)\n", skipped, plural(skipped))
	}
	return nil
}

// resolveGardenRepos returns the ordered list of repo directories to render,
// following the documented precedence: explicit args, then --scan, then the
// saved set. Explicit args and --scan are mutually informative but args win.
func resolveGardenRepos(args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}
	if gardenScan != "" {
		return scanForRepos(gardenScan)
	}
	st, _, err := loadGardenState()
	if err != nil {
		return nil, err
	}
	return st.GardenRepos, nil
}

// scanForRepos lists immediate subdirectories of root that are git repositories
// (contain a .git entry). It scans exactly one level deep -- enough to point it
// at a parent "code" directory -- and returns the matches sorted by name for a
// stable windowsill order.
func scanForRepos(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("garden: scanning %s: %w", root, err)
	}
	var repos []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if isRepo(dir) {
			repos = append(repos, dir)
		}
	}
	sort.Strings(repos)
	return repos, nil
}

// isRepo reports whether dir looks like a git working tree by checking for a
// .git entry (directory or file, the latter covering worktrees/submodules). It
// is a cheap filesystem check that avoids spawning git just to filter a scan.
func isRepo(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return true
	}
	return false
}

// terminalWidth determines the column budget for the garden layout. It prefers
// an explicit COLUMNS environment variable (respected by many shells and easy
// to pin in tests/CI), and otherwise returns 0 so render.Garden falls back to
// its own DefaultGardenWidth. Keeping width detection dependency-free avoids
// pulling in a terminal-sizing library for a cosmetic layout hint.
func terminalWidth(_ *cobra.Command) int {
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if n, err := strconv.Atoi(cols); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

// plural returns "s" for counts other than 1, for simple message pluralization.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func init() {
	gardenCmd.Flags().StringVar(&gardenScan, "scan", "",
		"scan a parent directory one level deep for git repos")
	gardenCmd.Flags().BoolVar(&gardenAdd, "add", false,
		"add path arguments (or the current repo) to the saved garden set")
	gardenCmd.Flags().BoolVar(&gardenRemove, "remove", false,
		"remove path arguments from the saved garden set")
	gardenCmd.Flags().BoolVar(&gardenList, "list", false,
		"list the saved garden set and exit")
	gardenCmd.Flags().StringVar(&gardenAuthor, "author", "",
		"score every repo against this author email (default: each repo's own)")
	gardenCmd.Flags().IntVar(&gardenWindow, "window", 0,
		"activity look-back window in days (default: 7)")

	rootCmd.AddCommand(gardenCmd)
}
