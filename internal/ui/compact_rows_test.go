package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestCompactNavigatorRowsPreservePrimaryProseAndFullWidthSelection(t *testing.T) {
	t.Parallel()
	row := NavigatorRow{
		Prefix: []Segment{{Text: "stash@{0} ", Tone: ToneAccent}},
		Label:  "long subject\nwith hostile controls\x1b",
		Suffix: []Segment{
			{Text: "99f ", Tone: ToneQuiet},
			{Text: "+123 ", Tone: ToneAdded},
			{Text: "-45 2h", Tone: ToneRemoved},
		},
	}
	for _, focused := range []bool{true, false} {
		narrow := renderCompactNavigatorRow(row, 20, true, focused)
		if width := lipgloss.Width(narrow); width != 20 {
			t.Fatalf("narrow selected row width = %d, want 20: %q", width, narrow)
		}
		plain := ansi.Strip(narrow)
		if !strings.Contains(plain, "stash@{0} long") || strings.Contains(plain, "99f") || strings.Contains(plain, "\n") {
			t.Fatalf("narrow row did not prioritize safe primary prose: %q", plain)
		}
	}

	wide := renderCompactNavigatorRow(row, 64, false, false)
	plain := ansi.Strip(wide)
	for _, want := range []string{"stash@{0}", "long subject↵with hostile controls", "99f", "+123", "-45", "2h"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("wide compact row misses %q: %q", want, plain)
		}
	}
	if !strings.Contains(wide, purpleStyle.Render("stash@{0} ")) ||
		!strings.Contains(wide, addedStyle.Render("+123 ")) ||
		!strings.Contains(wide, errorStyle.Render("-45 2h")) {
		t.Fatalf("compact row lacks semantic segment styles: %q", wide)
	}
}
