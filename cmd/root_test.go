package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rwrife/commit-sprout/internal/gitstat"
	"github.com/rwrife/commit-sprout/internal/plant"
	"github.com/rwrife/commit-sprout/internal/render"
	"github.com/spf13/cobra"
)

func TestBloomFlags(t *testing.T) {
	oldNoBloom, oldCommits := noBloom, bloomCommits
	t.Cleanup(func() { noBloom, bloomCommits = oldNoBloom, oldCommits })
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	act := gitstat.Activity{
		CommitsByDay: map[string]int{"2026-06-30": 4},
		TagsByDay:    map[string]int{"2026-06-30": 1},
	}
	base := plant.PlantState{Stage: plant.Leafy, Health: plant.Thirsty, Streak: 3}
	if err := rootCmd.PersistentFlags().Set("bloom-commits", "4"); err != nil {
		t.Fatal(err)
	}
	noBloom = false
	ps := applyBlooms(base, act, now)
	if ps.Blooms != 2 {
		t.Fatalf("custom threshold: blooms = %d; want 2", ps.Blooms)
	}
	if err := rootCmd.PersistentFlags().Set("no-bloom", "true"); err != nil {
		t.Fatal(err)
	}
	ps = applyBlooms(base, act, now)
	if ps != base || strings.Contains(render.Frame(ps, render.Options{Now: now}), "milestone flower") {
		t.Fatalf("--no-bloom failed to suppress only flowers: %+v", ps)
	}
	for _, threshold := range []int{0, -1} {
		bloomCommits = threshold
		if err := validateBloomFlags(); err == nil {
			t.Fatalf("threshold %d accepted", threshold)
		}
	}
}

func TestMilestonePlainOutput(t *testing.T) {
	old := noColor
	t.Cleanup(func() { noColor = old })
	for _, mode := range []string{"pipe", "no-color", "NO_COLOR"} {
		t.Run(mode, func(t *testing.T) {
			noColor = mode == "no-color"
			if mode == "NO_COLOR" {
				t.Setenv("NO_COLOR", "")
			}
			var out bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			ps := plant.PlantState{Stage: plant.Sprout, Blooms: 1}
			got := render.Frame(ps, render.Options{Color: useColor(cmd)})
			if strings.Contains(got, "\x1b") || !strings.Contains(got, "@") {
				t.Fatalf("%s: flower output is not plain:\n%s", mode, got)
			}
		})
	}
}
