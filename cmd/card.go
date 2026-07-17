// Card subcommand: export the current plant as a self-contained SVG so it can
// be embedded as a README badge (and, with --out -, streamed to stdout). It
// rides the same read pipeline as every other command, so the SVG shows exactly
// the plant the terminal render would — same stage, health, streak, and pests —
// just as a shareable, screenshot-free artifact.
//
// The green-squares contribution badge is culture; this gives commit-sprout an
// on-brand equivalent: a living-ish plant you can drop in a README.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rwrife/commit-sprout/internal/gitstat"
	"github.com/rwrife/commit-sprout/internal/render"
	"github.com/spf13/cobra"
)

// cardOut backs the --out flag: the destination SVG path. The special value "-"
// writes to stdout; any other value is a file path (default: plant.svg in cwd).
var cardOut string

// cardCmd implements `commit-sprout card`.
var cardCmd = &cobra.Command{
	Use:   "card",
	Short: "Export the current plant as an embeddable SVG (README badge)",
	Long: `Render the current plant to a self-contained SVG file so you can embed a
living-ish plant badge in your README. The card mirrors the terminal view — the
same stage, health, streak, and art — as monospace text plus a caption, with no
external fonts or assets.

By default it writes plant.svg in the current directory (atomically). Use
"--out path.svg" to choose a destination, or "--out -" to stream the SVG to
stdout. Color honors the theme (green / amber / red foliage, magenta bloom);
--no-color yields a mono card.`,
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCard(cmd)
	},
}

// runCard reads the pipeline, renders the plant to SVG, and either streams it to
// stdout (--out -) or writes it atomically to the chosen file. It does not
// persist plant state: exporting a card is a read-only snapshot.
func runCard(cmd *cobra.Command) error {
	r, err := runPipeline()
	if err != nil {
		if errors.Is(err, gitstat.ErrNotARepo) {
			return notARepoMessage(cmd)
		}
		return err
	}

	svg := render.Card(r.plant, render.CardOptions{
		Color:      cardUseColor(),
		Species:    r.species,
		Now:        r.now,
		LastCommit: r.activity.LastCommit,
	})

	// --out - streams to stdout so the card can be piped or captured inline.
	if cardOut == "-" {
		_, perr := fmt.Fprint(cmd.OutOrStdout(), svg)
		return perr
	}

	if err := atomicWriteFile(cardOut, []byte(svg)); err != nil {
		return fmt.Errorf("card: writing %s: %w", cardOut, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "🪴 Wrote %s\n", cardOut)
	return nil
}

// cardUseColor decides whether the SVG takes the themed color path. Unlike the
// terminal render, a card is an artifact (a file or an embed), not a TTY, so
// TTY detection would wrongly strip color. Color is therefore on by default and
// only disabled by --no-color or the NO_COLOR environment variable.
func cardUseColor() bool {
	if noColor {
		return false
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	return true
}

// atomicWriteFile writes data to path via a temp file in the same directory
// followed by a rename, so a reader never sees a partially written SVG and a
// crash mid-write cannot corrupt an existing card. It mirrors the durability
// approach used by internal/store.Save.
func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".commit-sprout-card-*.svg.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	return nil
}

func init() {
	cardCmd.Flags().StringVar(&cardOut, "out", "plant.svg",
		`SVG destination path, or "-" for stdout (default: plant.svg)`)
	rootCmd.AddCommand(cardCmd)
}
