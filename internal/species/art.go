// Species art sets: a full stage x health ASCII table per species.
//
// Each species owns a complete table so the render layer can select art purely
// by (species, stage, health) with no fallbacks that would hide a gap. The Fern
// table is byte-for-byte the original commit-sprout art, so choosing the
// default species (or passing no --species) renders exactly as the tool always
// has.
//
// Art is intentionally plain data here; the render package handles layout,
// captions, and color.
package species

import "github.com/rwrife/commit-sprout/internal/plant"

// artSets maps each species to its stage x health art table. Every species in
// All() must have a complete entry (guarded by a test) so ArtFor never has to
// fall back across species.
var artSets = map[Kind]map[plant.Stage]map[plant.Health]string{
	Fern:      fernArt,
	Cactus:    cactusArt,
	Bonsai:    bonsaiArt,
	Sunflower: sunflowerArt,
}

// ArtFor returns the ASCII frame for a (species, stage, health) triple. It
// resolves the species table (falling back to the default species only if an
// unknown Kind slips through), then the stage, then the health, using the
// species' Healthy frame as a last resort so a caller always gets art.
func ArtFor(k Kind, stage plant.Stage, health plant.Health) string {
	table, ok := artSets[k]
	if !ok {
		table = artSets[Default]
	}
	byHealth, ok := table[stage]
	if !ok {
		byHealth = table[plant.Seed]
	}
	if art, ok := byHealth[health]; ok {
		return art
	}
	return byHealth[plant.Healthy]
}

// fernArt is the original commit-sprout art, preserved exactly so the default
// species is a no-op visual change.
var fernArt = map[plant.Stage]map[plant.Health]string{
	plant.Seed: {
		plant.Healthy: "   \\|/\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Thirsty: "   .|.\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: "   .,.\n" +
			"    .\n" +
			"   _._\n" +
			"  (   )\n" +
			"   '-'",
	},
	plant.Sprout: {
		plant.Healthy: "    ,\n" +
			"   /|\n" +
			"  ' |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Thirsty: "    .\n" +
			"   \\|\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: "\n" +
			"   \\,\n" +
			"    `.\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
	},
	plant.Leafy: {
		plant.Healthy: "   \\ | /\n" +
			"    \\|/\n" +
			"   --+--\n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Thirsty: "   \\ | \n" +
			"    \\| \n" +
			"   --+  \n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Wilting: "   . . .\n" +
			"    \\ / \n" +
			"     +._ \n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
	},
	plant.Tall: {
		plant.Healthy: "   \\\\|//\n" +
			"    \\|/\n" +
			"   --+--\n" +
			"    /|\\\n" +
			"     |\n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Thirsty: "   \\\\| \n" +
			"    \\| \n" +
			"   --+- \n" +
			"    /| \n" +
			"     |\n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Wilting: "   . . .\n" +
			"    \\|/ \n" +
			"   ._+   \n" +
			"     \\\n" +
			"     |\n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
	},
	plant.Blooming: {
		plant.Healthy: "  @ \\|/ @\n" +
			"   \\\\|//\n" +
			"  @--+--@\n" +
			"    /|\\\n" +
			"     |\n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Thirsty: "  @ \\| \n" +
			"   \\\\| \n" +
			"  @--+- \n" +
			"    /| \n" +
			"     |\n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Wilting: "  * . . \n" +
			"   \\\\|/ \n" +
			"  .--+   \n" +
			"    /\\\n" +
			"     |\n" +
			"     |\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
	},
}

// cactusArt is a blocky, spiky desert plant. Its wilt frames droop far less
// dramatically than the fern's -- a cactus that has gone dry looks a little
// shrunken and dotted rather than collapsed -- reinforcing its drought-hardy
// personality (the real gameplay twist lives in its wilt thresholds).
var cactusArt = map[plant.Stage]map[plant.Health]string{
	plant.Seed: {
		plant.Healthy: "    .\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Thirsty: "    ,\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: "    .\n" +
			"   _._\n" +
			"  (   )\n" +
			"   '-'",
	},
	plant.Sprout: {
		plant.Healthy: "   _|_\n" +
			"  | | |\n" +
			"  |_|_|\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Thirsty: "   .|.\n" +
			"  | | |\n" +
			"  |_|_|\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: "   . .\n" +
			"  |_|_|\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
	},
	plant.Leafy: {
		plant.Healthy: " _  |  _\n" +
			"| |_|_| |\n" +
			"|_| | |_|\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Thirsty: " .  |  .\n" +
			"| |_|_| |\n" +
			"|_| | |_|\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: " .  |  .\n" +
			"|_|_|_|_|\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
	},
	plant.Tall: {
		plant.Healthy: " _  |  _\n" +
			"| |_|_| |\n" +
			"| | | | |\n" +
			"|_| | |_|\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Thirsty: " .  |  .\n" +
			"| |_|_| |\n" +
			"| | | | |\n" +
			"|_| | |_|\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: " .  |  .\n" +
			"| | | | |\n" +
			"|_|_|_|_|\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
	},
	plant.Blooming: {
		plant.Healthy: " @  |  @\n" +
			"| |_|_| |\n" +
			"| | | | |\n" +
			"|_| | |_|\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Thirsty: " o  |  o\n" +
			"| |_|_| |\n" +
			"| | | | |\n" +
			"|_| | |_|\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: " .  |  .\n" +
			"| | | | |\n" +
			"|_|_|_|_|\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
	},
}

// bonsaiArt is a gnarled, sculptural little tree: a leaning trunk with a
// compact canopy that fills out as it grows.
var bonsaiArt = map[plant.Stage]map[plant.Health]string{
	plant.Seed: {
		plant.Healthy: "    ~\n" +
			"    \\\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Thirsty: "    .\n" +
			"    \\\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: "    .\n" +
			"    ,\n" +
			"   _._\n" +
			"  (   )\n" +
			"   '-'",
	},
	plant.Sprout: {
		plant.Healthy: "   (~)\n" +
			"    \\\n" +
			"    /\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Thirsty: "   (.)\n" +
			"    \\\n" +
			"    /\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: "   ...\n" +
			"    \\\n" +
			"    ,\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
	},
	plant.Leafy: {
		plant.Healthy: "  (~~~)\n" +
			"    \\_\n" +
			"    _/\n" +
			"     \\\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Thirsty: "  (~..)\n" +
			"    \\_\n" +
			"    _/\n" +
			"     \\\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Wilting: "  (. .)\n" +
			"    \\_\n" +
			"     ,\n" +
			"     \\\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
	},
	plant.Tall: {
		plant.Healthy: " (~~~~~)\n" +
			"   \\_/\n" +
			"   _/\n" +
			"    \\_\n" +
			"     \\\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Thirsty: " (~~..~)\n" +
			"   \\_/\n" +
			"   _/\n" +
			"    \\_\n" +
			"     \\\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Wilting: " (. . .)\n" +
			"   \\_/\n" +
			"   ,/\n" +
			"    \\_\n" +
			"     \\\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
	},
	plant.Blooming: {
		plant.Healthy: " (*~~~*)\n" +
			"  \\_*_/\n" +
			"   _/\n" +
			"    \\_\n" +
			"     \\\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Thirsty: " (o~~~o)\n" +
			"  \\_o_/\n" +
			"   _/\n" +
			"    \\_\n" +
			"     \\\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
		plant.Wilting: " (. * .)\n" +
			"  \\_._/\n" +
			"   ,/\n" +
			"    \\_\n" +
			"     \\\n" +
			"    _|_\n" +
			"   (   )\n" +
			"    '-'",
	},
}

// sunflowerArt is a tall single stalk that ends in a large round bloom as it
// matures -- showy and unmistakable at the blooming stage.
var sunflowerArt = map[plant.Stage]map[plant.Health]string{
	plant.Seed: {
		plant.Healthy: "    o\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Thirsty: "    .\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: "    ,\n" +
			"    .\n" +
			"   _._\n" +
			"  (   )\n" +
			"   '-'",
	},
	plant.Sprout: {
		plant.Healthy: "   (o)\n" +
			"    |\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Thirsty: "   (.)\n" +
			"    |\n" +
			"    \\\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: "   ,.,\n" +
			"    \\\n" +
			"    .\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
	},
	plant.Leafy: {
		plant.Healthy: "  \\(o)/\n" +
			"    |\n" +
			"   -|\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Thirsty: "  .(o).\n" +
			"    |\n" +
			"   -|\n" +
			"    \\\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: "  .(.).\n" +
			"    \\\n" +
			"   .|\n" +
			"    .\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
	},
	plant.Tall: {
		plant.Healthy: " \\(oOo)/\n" +
			"    |\n" +
			"   -|-\n" +
			"    |\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Thirsty: " .(oOo).\n" +
			"    |\n" +
			"   -|-\n" +
			"    \\\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: " .(. .).\n" +
			"    \\\n" +
			"   .|.\n" +
			"    .\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
	},
	plant.Blooming: {
		plant.Healthy: " \\(@O@)/\n" +
			"   \\|/\n" +
			"   -|-\n" +
			"    |\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Thirsty: " .(@O@).\n" +
			"   \\|/\n" +
			"   -|-\n" +
			"    \\\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
		plant.Wilting: " .(o.o).\n" +
			"   .|.\n" +
			"   .|.\n" +
			"    .\n" +
			"    |\n" +
			"   _|_\n" +
			"  (   )\n" +
			"   '-'",
	},
}
