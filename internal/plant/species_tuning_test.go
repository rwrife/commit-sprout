package plant

import (
	"testing"
	"time"

	"github.com/rwrife/commit-sprout/internal/gitstat"
)

// TestComputeWithDefaultMatchesCompute ensures ComputeWith with the default
// tuning is identical to Compute, so the species hook is a pure superset that
// changes nothing for the default plant.
func TestComputeWithDefaultMatchesCompute(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	act := gitstat.Activity{
		HasCommits:    true,
		Streak:        5,
		TotalInWindow: 5,
		LastCommit:    now.AddDate(0, 0, -4), // 4 idle days
	}
	st := State{HighestStage: Tall}

	want := Compute(act, st, now)
	got := ComputeWith(act, st, now, DefaultHealthTuning())
	if got != want {
		t.Errorf("ComputeWith(default) = %+v, want %+v", got, want)
	}
}

// TestComputeWithDroughtTuningSurvivesLonger verifies a more forgiving tuning
// (the cactus twist) keeps a plant healthier for the same idle gap: where the
// default plant would wilt, the drought-hardy tuning is merely thirsty or fine.
func TestComputeWithDroughtTuningSurvivesLonger(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	// 6 idle days: well past the default wilting threshold (day 4+).
	act := gitstat.Activity{
		HasCommits: true,
		Streak:     0,
		LastCommit: now.AddDate(0, 0, -6),
	}
	st := State{HighestStage: Leafy}

	def := Compute(act, st, now)
	if def.Health != Wilting {
		t.Fatalf("precondition: default plant should be Wilting at 6 idle days, got %v", def.Health)
	}

	hardy := ComputeWith(act, st, now, HealthTuning{ThirstyAfterDays: 4, WiltingAfterDays: 9})
	if hardy.Health == Wilting {
		t.Errorf("drought-hardy plant should not be Wilting at 6 idle days, got %v", hardy.Health)
	}
}
