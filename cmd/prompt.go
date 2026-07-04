// Prompt subcommand: emit a single compact line -- a glyph plus the stage name
// -- suitable for embedding in a shell prompt (bash/zsh $PROMPT), a tmux status
// line, or a starship custom module.
//
// Design constraints (M6): prompt mode must be fast and must never block or
// break the shell. Two things follow from that:
//
//   - It is read-only by default. A prompt renders on every command you run, so
//     writing state each time would be needless disk churn; prompt mode skips
//     the save unless you explicitly pass --save. (The root command and status
//     still persist normally.)
//   - It fails soft. Outside a git repo -- or if git errors for any reason --
//     it prints nothing and exits 0, so a misconfigured prompt never dumps an
//     error into your shell. Embed it and forget it.
package cmd

import (
	"errors"
	"fmt"

	"github.com/rwrife/commit-sprout/internal/gitstat"
	"github.com/rwrife/commit-sprout/internal/render"
	"github.com/spf13/cobra"
)

// promptSave opts prompt mode back into persisting state. Off by default so the
// hot path (a prompt on every command) does no disk writes.
var promptSave bool

// promptCmd implements `commit-sprout prompt`.
var promptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Emit a one-line glyph for a shell prompt, tmux, or starship",
	Long: `Print a single compact line -- a glyph plus the plant's stage -- meant to
be embedded in a shell prompt, a tmux status line, or a starship custom module.

Prompt mode is built for the hot path: it is read-only by default (no state is
written, since a prompt renders constantly) and fails silently outside a git
repo, printing nothing and exiting 0 so it can never break your shell. Pass
--save if you do want a prompt render to update the plant's memory.`,
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return renderPrompt(cmd)
	},
}

// renderPrompt prints the compact glyph line. It swallows the not-a-repo case
// (and, defensively, does not surface git errors into the prompt) so that a
// prompt integration is always safe to leave in place.
func renderPrompt(cmd *cobra.Command) error {
	r, err := runPipeline()
	if err != nil {
		if errors.Is(err, gitstat.ErrNotARepo) {
			// Silent: nothing growing here, so contribute nothing to the prompt.
			return nil
		}
		// Any other git failure also stays silent in prompt mode; a broken
		// prompt is worse than a missing glyph.
		return nil
	}

	if _, perr := fmt.Fprintln(cmd.OutOrStdout(), render.PromptGlyph(r.plant)); perr != nil {
		return perr
	}

	// Prompt mode only persists when explicitly asked (--save). The --no-save
	// persistent flag still wins if set.
	if promptSave {
		persist(r)
	}
	return nil
}

func init() {
	// --save re-enables persistence for prompt mode (off by default).
	promptCmd.Flags().BoolVar(&promptSave, "save", false, "persist plant state on this prompt render (off by default)")
}
