package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestMinimumSizeBoundary(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		width  int
		height int
		want   bool
	}{
		{width: MinimumWidth - 1, height: MinimumHeight},
		{width: MinimumWidth, height: MinimumHeight - 1},
		{width: MinimumWidth, height: MinimumHeight, want: true},
		{width: MinimumWidth + 1, height: MinimumHeight + 1, want: true},
	} {
		if got := MeetsMinimumSize(test.width, test.height); got != test.want {
			t.Fatalf("MeetsMinimumSize(%d, %d) = %v, want %v", test.width, test.height, got, test.want)
		}
	}
}

func TestMinimumSizeScreenReportsCurrentAndRequiredDimensions(t *testing.T) {
	t.Parallel()
	const width, height = 59, 11
	frame := RenderMinimumSize(width, height)
	if gotWidth, gotHeight := lipgloss.Size(frame); gotWidth != width || gotHeight != height {
		t.Fatalf("minimum-size frame = %dx%d, want %dx%d", gotWidth, gotHeight, width, height)
	}
	plain := ansi.Strip(frame)
	for _, want := range []string{"terminal too small", "current  59 × 11", "minimum  60 × 12", "resize to continue"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("minimum-size frame lacks %q:\n%s", want, plain)
		}
	}
}

func TestMinimumSizeScreenRemainsBoundedAtTinyDimensions(t *testing.T) {
	t.Parallel()
	for width := 0; width < MinimumWidth; width++ {
		for height := 0; height < MinimumHeight; height++ {
			frame := RenderMinimumSize(width, height)
			gotWidth, gotHeight := lipgloss.Size(frame)
			if width == 0 || height == 0 {
				if frame != "" {
					t.Fatalf("RenderMinimumSize(%d, %d) = %q, want empty", width, height, frame)
				}
				continue
			}
			if gotWidth != width || gotHeight != height {
				t.Fatalf("RenderMinimumSize(%d, %d) = %dx%d", width, height, gotWidth, gotHeight)
			}
		}
	}
}
