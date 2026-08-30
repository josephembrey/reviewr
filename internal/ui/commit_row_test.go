package ui

import (
	"image/color"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/commitgraph"
	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func commitFixtureRows() []commitrow.Row {
	graphs := commitgraph.Layout([]commitgraph.Commit{
		{OID: "merge", Parents: []string{"main", "side"}, Merge: true},
		{OID: "side", Parents: []string{"root"}},
		{OID: "main", Parents: []string{"root"}},
		{OID: "root"},
	})
	now := time.Now().Unix()
	return []commitrow.Row{
		{
			Graph:        graphs[0],
			ShortOID:     "abcdef123456",
			Subject:      "Preserve the important commit message before a very long author name",
			Author:       "An Extremely Long Author Name",
			AuthoredUnix: now - 2*60*60,
			Refs: []commitrow.Ref{
				{Kind: commitrow.Branch, Name: "main"},
				{Kind: commitrow.Remote, Name: "origin/main"},
				{Kind: commitrow.Tag, Name: "v1"},
			},
			Merge: true,
		},
		{Graph: graphs[1], ShortOID: "bbbbbbb", Subject: "side", Author: "Author", AuthoredUnix: now - 5*60},
		{Graph: graphs[2], ShortOID: "ccccccc", Subject: "main", Author: "Author", AuthoredUnix: now - 24*60*60},
		{Graph: graphs[3], ShortOID: "ddddddd", Subject: "root", Author: "Author", AuthoredUnix: now - 14*24*60*60},
	}
}

func TestCommitRowUsesTruecolorGraphAndSemanticANSIPalette(t *testing.T) {
	t.Parallel()
	rows := commitFixtureRows()
	width := 100
	rendered := renderCommitRow(rows[0], commitrow.Measure(rows, width), width, false, false, time.Now())
	plain := ansi.Strip(rendered)
	if !strings.Contains(plain, "◎─╮") || !strings.Contains(plain, "abcdef1") {
		t.Fatalf("commit graph/SHA missing: %q", plain)
	}
	for _, value := range []string{" main", " origin/main", " v"} {
		if !strings.Contains(plain, value) {
			t.Fatalf("semantic trail missing %q: %q", value, plain)
		}
	}
	for _, sgr := range []string{"\x1b[36m", "\x1b[32m", "\x1b[33m"} {
		if !strings.Contains(rendered, sgr) {
			t.Fatalf("ANSI palette sequence %q missing from %q", sgr, rendered)
		}
	}
	if !strings.Contains(rendered, "38;2;") {
		t.Fatalf("commit graph lacks truecolor lane: %q", rendered)
	}
}

func TestCommitGraphPaletteReusesSixDistinctVividColors(t *testing.T) {
	t.Parallel()
	want := [...]color.Color{
		vividCyanColor,
		vividPurpleColor,
		vividGreenColor,
		vividYellowColor,
		vividBlueColor,
		vividRedColor,
	}
	seen := make(map[color.RGBA]struct{}, len(want))
	for index, graphColor := range graphPalette {
		if _, isANSI := graphColor.(ansi.BasicColor); isANSI {
			t.Fatalf("graph color %d unexpectedly uses ANSI slot %v", index, graphColor)
		}
		got := color.RGBAModel.Convert(graphColor).(color.RGBA)
		expected := color.RGBAModel.Convert(want[index]).(color.RGBA)
		if got != expected {
			t.Errorf("graph color %d = %v, want shared vivid color %v", index, got, expected)
		}
		if _, duplicate := seen[got]; duplicate {
			t.Errorf("graph color %d duplicates %v", index, got)
		}
		seen[got] = struct{}{}
	}
}

func TestMeasureNavigatorCommitsOnlyScansCommitViewports(t *testing.T) {
	t.Parallel()
	commits := commitFixtureRows()[:2]
	rows := []NavigatorRow{
		{Label: "ordinary file"},
		{Commit: &commits[0]},
		{Commit: &commits[1]},
	}
	columns, now := measureNavigatorCommits(rows, 0, 1, 80)
	if columns != (commitrow.Columns{}) || !now.IsZero() {
		t.Fatalf("non-commit viewport measured columns %+v at %v", columns, now)
	}

	columns, now = measureNavigatorCommits(rows, 1, 1, 80)
	if want := commitrow.Measure(commits, 80); columns != want {
		t.Fatalf("commit viewport columns = %+v, want %+v", columns, want)
	}
	if now.IsZero() {
		t.Fatal("commit viewport did not capture one shared age timestamp")
	}
}

func TestCommitRowSelectionCoversEveryCellAndKeepsLaneColor(t *testing.T) {
	t.Parallel()
	rows := commitFixtureRows()
	width := 72
	rendered := renderCommitRow(rows[1], commitrow.Measure(rows, width), width, true, true, time.Now())
	if lipgloss.Width(rendered) != width {
		t.Fatalf("selected width = %d, want %d", lipgloss.Width(rendered), width)
	}
	assertEveryDisplayRuneReversed(t, rendered)
	if !strings.Contains(rendered, "38;2;") {
		t.Fatalf("selected graph lost lane foreground: %q", rendered)
	}
}

func TestCommitRowClipsCalmlyAtNarrowWidths(t *testing.T) {
	t.Parallel()
	rows := commitFixtureRows()
	row := rows[0]
	row.Subject = "line\nbreak \x1b[31m"
	row.Author = "author\rname"
	row.Refs[0].Name = "ref\x1b[32m"
	for width := 1; width <= 40; width++ {
		rendered := renderCommitRow(row, commitrow.Measure(rows, width), width, width%2 == 0, true, time.Now())
		if got := lipgloss.Width(rendered); got != width {
			t.Fatalf("width %d rendered %d cells: %q", width, got, rendered)
		}
		plain := ansi.Strip(rendered)
		if strings.ContainsAny(plain, "\r\n\x1b") {
			t.Fatalf("width %d leaked control text: %q", width, plain)
		}
	}
}

func TestCommitRowPreservesMoreSubjectThanAuthor(t *testing.T) {
	t.Parallel()
	rows := commitFixtureRows()
	width := 58
	plain := ansi.Strip(renderCommitRow(rows[0], commitrow.Measure(rows, width), width, false, false, time.Now()))
	if !strings.Contains(plain, "Preserve the important") {
		t.Fatalf("subject lost primary allocation: %q", plain)
	}
	if strings.Contains(plain, rows[0].Author) {
		t.Fatalf("long author displaced prose instead of clipping: %q", plain)
	}
}

func TestCommitRowsCoexistWithNavigatorScrollbar(t *testing.T) {
	t.Parallel()
	g := Calculate(64, 12)
	fixtures := commitFixtureRows()
	rows := make([]NavigatorRow, 30)
	for index := range rows {
		row := fixtures[index%len(fixtures)]
		rows[index] = NavigatorRow{Identity: strconv.Itoa(index), Commit: &row}
	}
	frame := ansi.Strip(Render(Model{
		Geometry:       g,
		Workspace:      workspace.Git,
		NavigatorTitle: "commits · 30",
		NavigatorRows:  rows,
		Selected:       8,
		Top:            6,
		Focus:          navigation.FocusNavigator,
	}))
	lines := strings.Split(frame, "\n")
	for row := g.NavigatorRows.Y; row < g.NavigatorRows.Y+g.NavigatorRows.Height; row++ {
		cells := []rune(lines[row])
		cell := cells[g.Navigator.X+g.Navigator.Width-1]
		if cell != '▕' && cell != '▐' {
			t.Fatalf("row %d scrollbar cell = %q", row, cell)
		}
	}
}

func assertEveryDisplayRuneReversed(t *testing.T, value string) {
	t.Helper()
	reversed := false
	for index := 0; index < len(value); {
		if value[index] == '\x1b' && index+1 < len(value) && value[index+1] == '[' {
			end := strings.IndexByte(value[index+2:], 'm')
			if end < 0 {
				t.Fatalf("unterminated SGR in %q", value[index:])
			}
			parameters := value[index+2 : index+2+end]
			for _, parameter := range strings.Split(parameters, ";") {
				switch parameter {
				case "", "0":
					reversed = false
				case "7":
					reversed = true
				case "27":
					reversed = false
				}
			}
			index += end + 3
			continue
		}
		char, size := utf8.DecodeRuneInString(value[index:])
		if char != '\n' && !reversed {
			t.Fatalf("display rune %q at byte %d is outside reverse selection: %q", char, index, value)
		}
		index += size
	}
}
