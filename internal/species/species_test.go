package species

import (
	"strings"
	"testing"

	"github.com/rwrife/commit-sprout/internal/plant"
)

// TestParseRoundTrip verifies every species name parses back to its Kind and
// that String/Parse are inverses for the canonical names.
func TestParseRoundTrip(t *testing.T) {
	for _, k := range All() {
		name := k.String()
		got, ok := Parse(name)
		if !ok {
			t.Errorf("Parse(%q) reported not-ok for a canonical name", name)
		}
		if got != k {
			t.Errorf("Parse(%q) = %v, want %v", name, got, k)
		}
	}
}

// TestParseCaseAndSpace checks that parsing is case-insensitive and trims
// surrounding whitespace, matching how a user might type the flag.
func TestParseCaseAndSpace(t *testing.T) {
	cases := map[string]Kind{
		"  Cactus ": Cactus,
		"FERN":      Fern,
		"BonSai":    Bonsai,
		"sunflower": Sunflower,
	}
	for in, want := range cases {
		got, ok := Parse(in)
		if !ok || got != want {
			t.Errorf("Parse(%q) = (%v, %v), want (%v, true)", in, got, ok, want)
		}
	}
}

// TestParseUnknownFallsBackToDefault ensures unknown/empty names report not-ok
// and return the default species rather than a garbage Kind.
func TestParseUnknownFallsBackToDefault(t *testing.T) {
	for _, in := range []string{"", "  ", "oak", "weed"} {
		got, ok := Parse(in)
		if ok {
			t.Errorf("Parse(%q) unexpectedly reported ok", in)
		}
		if got != Default {
			t.Errorf("Parse(%q) = %v, want default %v", in, got, Default)
		}
	}
}

// TestArtCompleteForEverySpecies asserts every species has a non-empty frame
// for every (stage x health) combination the state machine can produce, so
// ArtFor never has to fall back and hide a gap.
func TestArtCompleteForEverySpecies(t *testing.T) {
	stages := []plant.Stage{plant.Seed, plant.Sprout, plant.Leafy, plant.Tall, plant.Blooming}
	healths := []plant.Health{plant.Healthy, plant.Thirsty, plant.Wilting}

	for _, k := range All() {
		for _, s := range stages {
			for _, h := range healths {
				art := ArtFor(k, s, h)
				if strings.TrimSpace(art) == "" {
					t.Errorf("species %s stage %s health %s has empty art", k, s, h)
				}
			}
		}
	}
}

// TestArtDiffersAcrossSpecies is a light guard that the species art sets are
// genuinely distinct and not accidental copies of the default.
func TestArtDiffersAcrossSpecies(t *testing.T) {
	base := ArtFor(Fern, plant.Blooming, plant.Healthy)
	for _, k := range []Kind{Cactus, Bonsai, Sunflower} {
		if ArtFor(k, plant.Blooming, plant.Healthy) == base {
			t.Errorf("species %s blooming/healthy art matches the fern; expected distinct art", k)
		}
	}
}

// TestCactusToleratesLongerDrought is the gameplay-twist assertion: the cactus
// must have wilt thresholds strictly more forgiving than the shared defaults,
// so it survives droughts longer than the leafier species.
func TestCactusToleratesLongerDrought(t *testing.T) {
	tune, ok := TuningFor(Cactus)
	if !ok {
		t.Fatal("cactus has no tuning override; expected a drought-hardy twist")
	}
	def := plant.DefaultHealthTuning()
	if tune.WiltingAfterDays <= def.WiltingAfterDays {
		t.Errorf("cactus WiltingAfterDays=%d not more forgiving than default=%d",
			tune.WiltingAfterDays, def.WiltingAfterDays)
	}
	if tune.ThirstyAfterDays < def.ThirstyAfterDays {
		t.Errorf("cactus ThirstyAfterDays=%d should be >= default=%d",
			tune.ThirstyAfterDays, def.ThirstyAfterDays)
	}
}

// TestNonCactusUseSharedDefaults confirms only the cactus bends the wilt rules
// today; the other species report no override and thus keep shared thresholds.
func TestNonCactusUseSharedDefaults(t *testing.T) {
	for _, k := range []Kind{Fern, Bonsai, Sunflower} {
		if _, ok := TuningFor(k); ok {
			t.Errorf("species %s unexpectedly has a tuning override", k)
		}
	}
}

// TestFernArtIsOriginal pins the fern's blooming/healthy frame to the original
// commit-sprout art so the default species is a guaranteed no-op visual change.
func TestFernArtIsOriginal(t *testing.T) {
	const original = "  @ \\|/ @\n" +
		"   \\\\|//\n" +
		"  @--+--@\n" +
		"    /|\\\n" +
		"     |\n" +
		"     |\n" +
		"    _|_\n" +
		"   (   )\n" +
		"    '-'"
	if got := ArtFor(Fern, plant.Blooming, plant.Healthy); got != original {
		t.Errorf("fern blooming/healthy art drifted from the original:\n%s", got)
	}
}
