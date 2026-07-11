package cmd

import (
	"strings"
	"testing"
)

// TestSwayIndent verifies the sway offsets form the expected triangle wave and
// that every line is padded (not just the first), joined with \r\n for raw mode.
func TestSwayIndent(t *testing.T) {
	frame := "abc\ndef"
	cases := []struct {
		phase int
		pad   string
	}{
		{0, ""},
		{1, " "},
		{2, "  "},
		{3, " "},
		{4, ""}, // wraps
	}
	for _, c := range cases {
		got := swayIndent(frame, c.phase)
		want := c.pad + "abc\r\n" + c.pad + "def"
		if got != want {
			t.Errorf("phase %d: got %q, want %q", c.phase, got, want)
		}
	}
}

// TestComposeScreenFooter ensures the quit footer is always present so the user
// is never left guessing how to exit the full-screen view.
func TestComposeScreenFooter(t *testing.T) {
	screen := composeScreen("plant", 0)
	if !strings.Contains(screen, "q / Ctrl-C to quit") {
		t.Errorf("footer missing from watch screen: %q", screen)
	}
	if !strings.HasPrefix(screen, "plant") {
		t.Errorf("plant body missing/misplaced: %q", screen)
	}
}
