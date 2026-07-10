// Watch subcommand: a live, full-screen TUI that renders the plant and lets it
// gently sway. Where the root command prints one frame and exits, `watch` keeps
// a minimal alternate-screen loop running, re-reading git activity on an
// interval so the plant reflects new commits as they land, and animating an
// idle sway between reads so the creature feels alive on a windowsill.
//
// Design goals (see issue #10):
//
//   - Minimal full-screen TUI on the alternate screen buffer.
//   - Idle sway animation; git activity is re-read on a bounded interval.
//   - Clean exit on q / Q / Ctrl-C, always restoring the terminal.
//   - Low CPU when idle: frames are throttled and we only redraw on change.
//
// The animation is deliberately tiny and dependency-free: raw terminal mode via
// golang.org/x/sys plus a handful of ANSI escapes, so it degrades to nothing
// extra in the binary and stays fast to start.
package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rwrife/commit-sprout/internal/gitstat"
	"github.com/rwrife/commit-sprout/internal/render"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

// watchInterval is how often watch re-reads git activity. Commits do not land
// every second, so a slow poll keeps the plant fresh without churning git.
var watchInterval time.Duration

// swayPeriod bounds the idle sway animation. A slow step is plenty for a gentle
// sway and keeps CPU near-idle; combined with redraw-on-change it means a
// perfectly still plant costs almost nothing.
const (
	swaySteps  = 4                      // frames in one sway cycle
	swayPeriod = 700 * time.Millisecond // wall-clock time per sway step
)

// watchCmd implements `commit-sprout watch`.
var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Live full-screen view of the plant, gently swaying",
	Long: `Open a minimal full-screen view of the plant and let it live: it sways
gently and re-reads your git activity on an interval, so a fresh commit sprouts
without you restarting anything.

Press q or Ctrl-C to exit; the terminal is always restored on the way out.

When stdout is not a terminal (piped or redirected) watch refuses to start,
since there is nothing to animate.`,
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWatch(cmd)
	},
}

// runWatch drives the watch loop. It sets up raw mode and the alternate screen,
// installs a signal handler, and runs an event loop that redraws on animation
// ticks and on git re-reads, exiting cleanly on q / Ctrl-C or SIGINT/SIGTERM.
func runWatch(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	f, ok := out.(*os.File)
	if !ok || !isTerminalFile(f) {
		return errors.New("watch requires an interactive terminal (stdout is not a TTY)")
	}
	if watchInterval <= 0 {
		return errors.New("watch: --interval must be positive")
	}

	// Enter raw mode so single keypresses (q, Ctrl-C) are delivered without
	// line buffering, and restore it no matter how we leave.
	restore, err := enterRaw(f)
	if err != nil {
		return fmt.Errorf("watch: could not set up terminal: %w", err)
	}
	defer restore()

	// Alternate screen + hidden cursor for a clean canvas; always undo both.
	fmt.Fprint(out, altScreenEnter+cursorHide)
	defer fmt.Fprint(out, cursorShow+altScreenLeave)

	// Translate SIGINT/SIGTERM into a graceful quit so the deferred restores
	// still run (a bare signal would skip them and wreck the terminal).
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// Keypresses arrive on their own goroutine so the render loop never blocks
	// on input; a closed channel signals EOF/read error.
	keyCh := make(chan byte, 8)
	go readKeys(f, keyCh)

	// Prime the first git read immediately, then refresh on the interval.
	frame := readFrame()
	ticker := time.NewTicker(swayPeriod)
	defer ticker.Stop()
	refresh := time.NewTicker(watchInterval)
	defer refresh.Stop()

	phase := 0
	var last string // last drawn frame, to skip redundant redraws (low CPU idle)

	draw := func() {
		screen := composeScreen(frame, phase)
		if screen == last {
			return
		}
		last = screen
		fmt.Fprint(out, clearScreen+home+screen)
	}
	draw()

	for {
		select {
		case <-sigCh:
			return nil
		case b, open := <-keyCh:
			if !open {
				return nil // input closed; leave gracefully
			}
			if b == 'q' || b == 'Q' || b == 3 /* Ctrl-C */ {
				return nil
			}
		case <-ticker.C:
			phase = (phase + 1) % swaySteps
			draw()
		case <-refresh.C:
			frame = readFrame()
			draw()
		}
	}
}

// readFrame reads git activity for the current repo and renders one plain-ASCII\n// plant frame, degrading gracefully: outside a repo (or on any read error) it
// returns a friendly message instead of an error so the loop keeps running.
// It never persists state — watch is a read-only view.
func readFrame() string {
	r, err := runPipeline()
	if err != nil {
		if errors.Is(err, gitstat.ErrNotARepo) {
			return "Not a git repository — nothing growing here yet."
		}
		return "commit-sprout: " + err.Error()
	}
	return render.Frame(r.plant, render.Options{
		Color:      false, // the sway is the animation; plain ASCII keeps redraw cheap
		Now:        r.now,
		LastCommit: r.activity.LastCommit,
		Species:    r.species,
	})
}

// composeScreen applies the sway to a rendered frame and appends the quit
// footer, producing the exact bytes drawn for a given animation phase.
func composeScreen(frame string, phase int) string {
	return swayIndent(frame, phase) + "\r\n\r\n  q / Ctrl-C to quit"
}

// swayIndent shifts every line right by a small, phase-driven amount to fake a
// gentle breeze. The offsets form a smooth triangle wave (0,1,2,1) so the plant
// leans one way and back rather than jumping. Lines are joined with \r\n so raw
// mode (no ONLCR translation) still returns the cursor to column 0.
func swayIndent(frame string, phase int) string {
	offsets := []int{0, 1, 2, 1}
	pad := strings.Repeat(" ", offsets[phase%len(offsets)])
	lines := strings.Split(frame, "\n")
	for i, ln := range lines {
		lines[i] = pad + ln
	}
	return strings.Join(lines, "\r\n")
}

// readKeys forwards single bytes from the terminal to keyCh until read fails
// (EOF / error), then closes the channel to signal the loop to exit.
func readKeys(f *os.File, keyCh chan<- byte) {
	defer close(keyCh)
	r := bufio.NewReader(f)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return // EOF or read error: signal exit by closing the channel
		}
		keyCh <- b
	}
}

// --- terminal plumbing -----------------------------------------------------

const (
	altScreenEnter = "\x1b[?1049h"
	altScreenLeave = "\x1b[?1049l"
	cursorHide     = "\x1b[?25l"
	cursorShow     = "\x1b[?25h"
	clearScreen    = "\x1b[2J"
	home           = "\x1b[H"
)

// isTerminalFile reports whether f is a real terminal via a TCGETS ioctl.
func isTerminalFile(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	return err == nil
}

// enterRaw puts the terminal into byte-at-a-time raw mode (no echo, no
// canonical line buffering, no signal generation on Ctrl-C so we can handle it
// ourselves) and returns a function that restores the original settings exactly.
func enterRaw(f *os.File) (func(), error) {
	fd := int(f.Fd())
	orig, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}
	raw := *orig
	raw.Lflag &^= unix.ECHO | unix.ICANON | unix.ISIG
	raw.Iflag &^= unix.IXON | unix.ICRNL
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &raw); err != nil {
		return nil, err
	}
	return func() { _ = unix.IoctlSetTermios(fd, unix.TCSETS, orig) }, nil
}

func init() {
	watchCmd.Flags().DurationVar(&watchInterval, "interval", 5*time.Second,
		"how often to re-read git activity (e.g. 2s, 1m)")
	rootCmd.AddCommand(watchCmd)
}
