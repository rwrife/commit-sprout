package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rwrife/commit-sprout/internal/gitstat"
	"github.com/rwrife/commit-sprout/internal/plant"
	"github.com/spf13/cobra"
)

func TestNotifyOutput(t *testing.T) {
	for _, tt := range []struct {
		name                 string
		days, grace, limit   int
		quiet, json, desktop bool
		want                 string
		sendErr              error
	}{
		{name: "healthy silent", days: 0, limit: 1},
		{name: "no history silent", days: -1, limit: 10, json: true, desktop: true},
		{name: "outside boundary", days: 2, limit: 1, desktop: true},
		{name: "at boundary", days: 3, limit: 1, want: "Your plant wilts in 1 day -- a commit today keeps it green.\n"},
		{name: "custom threshold", days: 2, limit: 2, want: "Your plant wilts in 2 days -- a commit soon keeps it green.\n"},
		{name: "already wilted", days: 8, limit: 0, want: "Your plant is wilting -- a commit can revive it.\n"},
		{name: "watered silent", days: 3, grace: 1, limit: 1},
		{name: "quiet desktop", days: 3, limit: 1, quiet: true, desktop: true},
		{name: "json", days: 3, limit: 1, json: true},
		{name: "desktop failure", days: 3, limit: 1, quiet: true, desktop: true, sendErr: errors.New("unavailable")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := pipelineResult{
				plant:  plant.PlantState{DaysSinceCommit: tt.days, GraceDays: tt.grace},
				tuning: plant.DefaultHealthTuning(),
			}
			var out, stderr bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			cmd.SetContext(context.Background())
			calls := 0
			send := func(ctx context.Context, message string) error {
				calls++
				if message == "" {
					t.Fatal("empty desktop message")
				}
				return tt.sendErr
			}
			if err := emitNotification(cmd, r, tt.limit, tt.quiet, tt.json, tt.desktop, send); err != nil {
				t.Fatal(err)
			}
			if tt.json && tt.days >= 0 {
				var got struct {
					Days      int    `json:"days_until_wilt"`
					Threshold int    `json:"threshold"`
					Message   string `json:"message"`
				}
				if err := json.Unmarshal(out.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.Days != 1 || got.Threshold != tt.limit || got.Message == "" || bytes.Count(out.Bytes(), []byte("\n")) != 1 {
					t.Fatalf("unexpected JSON: %s", &out)
				}
			} else if out.String() != tt.want {
				t.Fatalf("stdout = %q, want %q", out.String(), tt.want)
			}
			atRisk := tt.days >= 0 && plant.DaysUntilWilt(r.plant, r.tuning) <= tt.limit
			wantCalls := 0
			if tt.desktop && atRisk {
				wantCalls = 1
			}
			if calls != wantCalls || (stderr.Len() > 0) != (tt.sendErr != nil) {
				t.Fatalf("desktop calls = %d, stderr = %q", calls, stderr.String())
			}
		})
	}
}

type notifyFailWriter struct{}

func (notifyFailWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestNotifyOutputErrors(t *testing.T) {
	for _, jsonOutput := range []bool{false, true} {
		cmd := &cobra.Command{}
		cmd.SetOut(notifyFailWriter{})
		r := pipelineResult{plant: plant.PlantState{DaysSinceCommit: 3}, tuning: plant.DefaultHealthTuning()}
		err := emitNotification(cmd, r, 1, false, jsonOutput, false, nil)
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("output error = %v", err)
		}
	}
}

func TestNotifyCommandErrorsAndReadOnly(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Error(err)
		}
	})
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, error) {
		t.Helper()
		cmd := newNotifyCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs(args)
		err := cmd.Execute()
		return out.String(), err
	}
	for _, args := range [][]string{{"--threshold=-1"}, {"--quiet", "--json"}, {"unexpected"}} {
		if out, err := run(args...); err == nil || out != "" {
			t.Fatalf("%v: stdout = %q, error = %v", args, out, err)
		}
	}
	if out, err := run("--quiet"); !errors.Is(err, gitstat.ErrNotARepo) || out != "" {
		t.Fatalf("outside repo: stdout = %q, error = %v", out, err)
	}
	if out, err := exec.Command("git", "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}
	if out, err := run("--json"); err == nil || out != "" {
		t.Fatalf("git log failure: stdout = %q, error = %v", out, err)
	}
	if err := os.Chdir(original); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "rev-parse", "--verify", "HEAD").Run(); err != nil {
		t.Skip("read-only pipeline test requires a checkout with history")
	}
	if out, err := run("--quiet"); err != nil || out != "" {
		t.Fatalf("read-only run: stdout = %q, error = %v", out, err)
	}
	statePath := filepath.Join(dir, "config", "commit-sprout", "state.json")
	if _, err := os.Stat(statePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("notify wrote state: %v", err)
	}
	// A directory at the state file path deterministically fails reads on all OSes.
	if err := os.MkdirAll(statePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := run("--quiet"); err == nil || out != "" || !strings.Contains(err.Error(), "store: reading") {
		t.Fatalf("state error: stdout = %q, error = %v", out, err)
	}
}

func TestDesktopUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := sendDesktopNotification(context.Background(), "A gentle nudge"); err == nil {
		t.Fatal("expected unavailable desktop tool/platform")
	}
}
