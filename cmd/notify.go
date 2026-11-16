package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/rwrife/commit-sprout/internal/plant"
	"github.com/spf13/cobra"
)

func newNotifyCmd() *cobra.Command {
	var threshold int
	var quiet, jsonOutput, desktop bool
	cmd := &cobra.Command{
		Use:   "notify",
		Short: "Opt-in nudge when the plant is close to wilting",
		Long: `Check once, without saving state or watching in the background.
Print one warning only when days until wilt <= threshold (including plants
already wilting). No matching activity means no warning. Successful checks always
exit 0; errors exit nonzero. Quiet suppresses stdout, not errors or desktop
notifications. JSON emits one warning object only when at risk.`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if threshold < 0 {
				return fmt.Errorf("--threshold must be non-negative")
			}
			r, err := runPipelineWithStateErrors(true)
			if err != nil {
				return err
			}
			return emitNotification(cmd, r, threshold, quiet, jsonOutput, desktop, sendDesktopNotification)
		},
	}
	cmd.Flags().IntVar(&threshold, "threshold", 1, "warn when at most this many calendar days remain (non-negative)")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress stdout (errors are still reported)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "emit a JSON warning instead of text")
	cmd.Flags().BoolVar(&desktop, "desktop", false, "also attempt a native desktop notification (Linux/macOS)")
	cmd.MarkFlagsMutuallyExclusive("quiet", "json")
	return cmd
}

func emitNotification(cmd *cobra.Command, r pipelineResult, threshold int, quiet, jsonOutput, desktop bool, send func(context.Context, string) error) error {
	days := plant.DaysUntilWilt(r.plant, r.tuning)
	if days < 0 || days > threshold {
		return nil
	}
	message := "Your plant is wilting -- a commit can revive it."
	if days == 1 {
		message = "Your plant wilts in 1 day -- a commit today keeps it green."
	} else if days > 1 {
		message = fmt.Sprintf("Your plant wilts in %d days -- a commit soon keeps it green.", days)
	}
	if !quiet {
		if jsonOutput {
			if err := json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
				DaysUntilWilt int    `json:"days_until_wilt"`
				Threshold     int    `json:"threshold"`
				Message       string `json:"message"`
			}{days, threshold, message}); err != nil {
				return err
			}
		} else if _, err := fmt.Fprintln(cmd.OutOrStdout(), message); err != nil {
			return err
		}
	}
	if desktop {
		if err := send(cmd.Context(), message); err != nil {
			_, writeErr := fmt.Fprintln(cmd.ErrOrStderr(), "commit-sprout: warning: desktop notification skipped:", err)
			return writeErr
		}
	}
	return nil
}

func sendDesktopNotification(ctx context.Context, message string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	switch runtime.GOOS {
	case "linux":
		return exec.CommandContext(ctx, "notify-send", "--", "commit-sprout", message).Run()
	case "darwin":
		return exec.CommandContext(ctx, "osascript", "-e",
			"on run argv\n display notification (item 1 of argv) with title \"commit-sprout\"\nend run",
			"--", message).Run()
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}
