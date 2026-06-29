// Package cmd defines the cobra command tree for commit-sprout.
//
// For M1 the root command simply renders a single hard-coded ASCII seedling.
// Later milestones wire in git activity reading, the plant state machine,
// rendering, and persistent state.
package cmd

import (
	"fmt"
	"os"

	"github.com/rwrife/commit-sprout/internal/render"
	"github.com/spf13/cobra"
)

// version is the build version. It is overridable at build time via:
//
//	go build -ldflags "-X github.com/rwrife/commit-sprout/cmd.version=v0.1.0"
var version = "dev"

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
		_, err := fmt.Fprintln(cmd.OutOrStdout(), render.Seedling())
		return err
	},
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
}
