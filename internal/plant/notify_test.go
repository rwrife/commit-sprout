package plant

import "testing"

func TestDaysUntilWilt(t *testing.T) {
	for _, tt := range []struct {
		name        string
		days, grace int
		tune        HealthTuning
		want        int
	}{
		{"no history", -1, 0, DefaultHealthTuning(), -1},
		{"today", 0, 0, DefaultHealthTuning(), 4},
		{"last safe day", 3, 0, DefaultHealthTuning(), 1},
		{"first wilting day", 4, 0, DefaultHealthTuning(), 0},
		{"already wilting", 20, 0, DefaultHealthTuning(), 0},
		{"watering grace", 3, 2, DefaultHealthTuning(), 3},
		{"grace ahead of idle", 0, 2, DefaultHealthTuning(), 6},
		{"grace capped", 3, 100, DefaultHealthTuning(), 3},
		{"negative grace", 3, -1, DefaultHealthTuning(), 1},
		{"drought hardy", 3, 0, HealthTuning{4, 9}, 7},
		{"drought hardy boundary", 9, 0, HealthTuning{4, 9}, 1},
		{"drought hardy watered", 10, 2, HealthTuning{4, 9}, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ps := PlantState{DaysSinceCommit: tt.days, GraceDays: tt.grace}
			if got := DaysUntilWilt(ps, tt.tune); got != tt.want {
				t.Fatalf("DaysUntilWilt = %d, want %d", got, tt.want)
			}
		})
	}
}
