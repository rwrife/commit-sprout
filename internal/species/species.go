// Package species defines the pluggable "art set + personality" for a plant.
//
// A Species bundles two things: a full stage x health table of ASCII art, and a
// small set of tuning overrides that give the species a gameplay quirk (e.g. a
// cactus tolerates longer dry spells before it wilts). Everything here is pure
// data plus tiny lookups -- no I/O -- so it stays cheap to test and safe to
// depend on from both the pure plant brain and the render layer.
//
// The default species (Fern) intentionally reuses the original art the tool
// shipped with, so existing renders are byte-for-byte unchanged when no
// --species is chosen.
package species

import "strings"

// Kind identifies a species by a short, stable, lowercase name. The name is
// part of the CLI/config contract (it is what --species and the state file
// store) so keep the strings stable.
type Kind int

const (
	// Fern is the default species: the original commit-sprout art, upright
	// and leafy. Its zero value makes "unset" mean "fern", which keeps old
	// state files and flag-less runs on the historical art.
	Fern Kind = iota
	// Cactus is the drought-hardy species: blocky, spiky art and a gameplay
	// twist -- it tolerates noticeably longer commit gaps before it wilts.
	Cactus
	// Bonsai is the slow, sculptural species: a gnarled trunk with a compact
	// canopy.
	Bonsai
	// Sunflower is the showy species: a tall stalk that ends in a big bloom.
	Sunflower
)

// String returns the stable lowercase name for the species. These names are
// the --species / config contract, so keep them stable.
func (k Kind) String() string {
	switch k {
	case Fern:
		return "fern"
	case Cactus:
		return "cactus"
	case Bonsai:
		return "bonsai"
	case Sunflower:
		return "sunflower"
	default:
		return "fern"
	}
}

// Default is the species used when none is chosen (flag empty, config empty,
// or an unrecognized name). It is Fern so the historical art is the fallback.
const Default = Fern

// Parse resolves a species name (case-insensitive, surrounding space trimmed)
// into a Kind. It returns ok=false for empty or unknown names so callers can
// distinguish "use the default" from "the user typed something valid".
func Parse(name string) (Kind, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "fern":
		return Fern, true
	case "cactus":
		return Cactus, true
	case "bonsai":
		return Bonsai, true
	case "sunflower":
		return Sunflower, true
	default:
		return Default, false
	}
}

// All returns every species in a stable order, handy for help text, tests, and
// completeness checks.
func All() []Kind {
	return []Kind{Fern, Cactus, Bonsai, Sunflower}
}

// Names returns the stable name of every species, in the same order as All.
func Names() []string {
	ks := All()
	out := make([]string, len(ks))
	for i, k := range ks {
		out[i] = k.String()
	}
	return out
}

// Tuning is the per-species gameplay override for health (wilt) thresholds.
// The plant brain reads these instead of its own hard-coded constants so a
// species can, for example, survive droughts longer. Growth (stage/streak)
// tuning is deliberately left shared across species; only health tolerance
// varies today.
//
// The two fields mirror plant's own health thresholds: idle days at or below
// ThirstyAfterDays are Healthy, at or below WiltingAfterDays are Thirsty, and
// beyond that the plant Wilts.
type Tuning struct {
	// ThirstyAfterDays is the last idle day count that is still Healthy.
	ThirstyAfterDays int
	// WiltingAfterDays is the last idle day count that is still Thirsty;
	// beyond it the plant Wilts.
	WiltingAfterDays int
}

// tunings holds each species' health tuning. A species absent from this map
// uses the caller's shared defaults, so only species with a real twist need an
// entry. The cactus is the gameplay twist called out in the feature: it stays
// healthy and merely thirsty far longer than the leafier species before it
// finally wilts.
var tunings = map[Kind]Tuning{
	Cactus: {
		ThirstyAfterDays: 4, // healthy for the better part of a week
		WiltingAfterDays: 9, // only truly wilts after ~10 idle days
	},
}

// TuningFor returns the health tuning for a species and whether a species
// override exists. When ok is false the caller should keep its shared default
// thresholds, so this reports "does this species bend the rules?" rather than
// silently substituting zeros.
func TuningFor(k Kind) (Tuning, bool) {
	t, ok := tunings[k]
	return t, ok
}
