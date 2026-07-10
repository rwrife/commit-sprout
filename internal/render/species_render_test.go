package render

import (
	"strings"
	"testing"

	"github.com/rwrife/commit-sprout/internal/plant"
	"github.com/rwrife/commit-sprout/internal/species"
)

// TestFrameSpeciesSelectsArtSet verifies that setting Options.Species changes
// the rendered art: a cactus frame must differ from the default fern frame for
// the same stage/health.
func TestFrameSpeciesSelectsArtSet(t *testing.T) {
	ps := plant.PlantState{
		Stage:  plant.Blooming,
		Health: plant.Healthy,
		Streak: 20,
	}

	fern := Frame(ps, Options{})                          // default species
	cactus := Frame(ps, Options{Species: species.Cactus}) // explicit cactus

	if fern == cactus {
		t.Fatal("cactus frame matched the default fern frame; species not applied")
	}
	// Sanity: the fern still contains its signature pot line.
	if !strings.Contains(fern, "(   )") {
		t.Error("fern frame missing expected pot art")
	}
}

// TestFrameDefaultSpeciesIsFern confirms the zero-value species renders the
// original art, so existing callers that never set Species are unaffected.
func TestFrameDefaultSpeciesIsFern(t *testing.T) {
	ps := plant.PlantState{Stage: plant.Leafy, Health: plant.Healthy}

	zero := Frame(ps, Options{})
	explicit := Frame(ps, Options{Species: species.Fern})
	if zero != explicit {
		t.Error("zero-value species did not match explicit Fern; default is not fern")
	}
}

// TestGardenPlantSpecies verifies a garden cell honors its per-plant species.
func TestGardenPlantSpecies(t *testing.T) {
	ps := plant.PlantState{Stage: plant.Tall, Health: plant.Healthy}
	out := Garden([]GardenPlant{
		{Label: "fern-repo", State: ps},
		{Label: "cactus-repo", State: ps, Species: species.Cactus},
	}, Options{Width: 200})

	if strings.TrimSpace(out) == "" {
		t.Fatal("garden rendered empty")
	}
	// The cactus tall art has a distinctive block trunk row absent from the fern.
	if !strings.Contains(out, "|_| | |_|") {
		t.Error("garden output missing cactus art; per-plant species not applied")
	}
}
