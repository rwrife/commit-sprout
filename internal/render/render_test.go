package render

import (
	"strings"
	"testing"
)

func TestSeedlingNotEmpty(t *testing.T) {
	got := Seedling()
	if strings.TrimSpace(got) == "" {
		t.Fatal("Seedling() returned empty output; expected ASCII art")
	}
}

func TestSeedlingIsStable(t *testing.T) {
	// The M1 seedling is hard-coded; guard against accidental edits.
	if Seedling() != seedling {
		t.Errorf("Seedling() did not match the seedling constant")
	}
}
