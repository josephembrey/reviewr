package ui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/commitgraph"
	"github.com/josephembrey/reviewr/internal/commitrow"
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestRefsRowsRenderDistinctRestrainedIconsTonesAndTrails(t *testing.T) {
	t.Parallel()
	rows := []NavigatorRow{
		{Identity: "all", Prefix: []Segment{{Text: "  ", Tone: ToneSpecial}}, Label: "All refs", Suffix: []Segment{{Text: "  public refs", Tone: ToneQuiet}}},
		{Identity: "current", Prefix: []Segment{{Text: "  ", Tone: ToneAdded}}, Label: "main", Suffix: []Segment{{Text: "  current worktree", Tone: ToneQuiet}}},
		{Identity: "linked", Prefix: []Segment{{Text: "  ", Tone: ToneSpecial}}, Label: "feature", Suffix: []Segment{{Text: "  /very/long/linked/worktree/path", Tone: ToneQuiet}}},
		{Identity: "branch", Prefix: []Segment{{Text: "  ", Tone: ToneAdded}}, Label: "topic", Suffix: []Segment{{Text: "  origin/topic >", Tone: ToneQuiet}}},
		{Identity: "remote", Prefix: []Segment{{Text: "  ", Tone: ToneInfo}}, Label: "origin/main", Suffix: []Segment{{Text: "  remote", Tone: ToneQuiet}}},
		{Identity: "tag", Prefix: []Segment{{Text: "  ", Tone: ToneWarning}}, Label: "v1", Suffix: []Segment{{Text: "  tag", Tone: ToneQuiet}}},
	}
	model := Model{
		Geometry:         Calculate(100, 16),
		Workspace:        workspace.Git,
		Controls:         workspace.Controls{Git: workspace.GitRefs},
		NavigatorTitle:   "refs · 5",
		NavigatorRows:    rows,
		Selected:         1,
		Focus:            navigation.FocusNavigator,
		ReaderTitle:      "history · main · aaaaaaa · 2 commits",
		ReaderCommitRows: refTestCommitRows(),
	}
	frame := Render(model)
	plain := ansi.Strip(frame)
	for _, want := range []string{"refs · 5", "All refs", "public refs", "current worktree", "origin/main", "remote", "history · main", "aaaaaaa", "tip subject", "Ada"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("Refs frame lacks %q:\n%s", want, plain)
		}
	}
	for _, want := range []string{
		specialStyle.Render("  "),
		addedStyle.Render("  "),
		headerStyle.Render("  "),
		warningStyle.Render("  "),
	} {
		if !strings.Contains(frame, want) {
			t.Fatalf("Refs frame lacks colored icon %q:\n%q", want, frame)
		}
	}
	if strings.Contains(plain, "reviewed") || strings.Contains(plain, "unreviewed") {
		t.Fatalf("Git view contains a review badge:\n%s", plain)
	}
	if width, height := lipgloss.Size(frame); width != 100 || height != 16 {
		t.Fatalf("Refs frame size = %dx%d, want 100x16", width, height)
	}
}

func TestRefsNavigatorPrioritizesLabelAndKeepsSelectionFullWidth(t *testing.T) {
	t.Parallel()
	row := NavigatorRow{
		Identity: "worktree",
		Prefix:   []Segment{{Text: "  ", Tone: ToneSpecial}},
		Label:    "important-source-label",
		Suffix:   []Segment{{Text: "  /an/extremely/long/worktree/path/that/is/secondary", Tone: ToneQuiet}},
	}
	for _, focused := range []bool{false, true} {
		rendered := renderNavigatorPresentationRow(row, 18, true, focused, commitrow.Columns{}, time.Time{})
		plain := ansi.Strip(rendered)
		if lipgloss.Width(rendered) != 18 || lipgloss.Width(plain) != 18 {
			t.Fatalf("selected metadata row width = %d/%d, want 18: %q", lipgloss.Width(rendered), lipgloss.Width(plain), rendered)
		}
		if !strings.Contains(plain, "important-") || strings.Contains(plain, "/an/") {
			t.Fatalf("narrow row did not prioritize label: %q", plain)
		}
		if !strings.Contains(rendered, "\x1b[7m") && !strings.Contains(rendered, ";7m") {
			t.Fatalf("selected row lacks terminal reverse treatment: %q", rendered)
		}
	}

	short := renderNavigatorPresentationRow(NavigatorRow{Prefix: []Segment{{Text: "  ", Tone: ToneAdded}}, Label: "main", Suffix: []Segment{{Text: "  origin/main =", Tone: ToneQuiet}}}, 28, false, false, commitrow.Columns{}, time.Time{})
	if plain := ansi.Strip(short); !strings.Contains(plain, "main  origin/main =") {
		t.Fatalf("roomy branch row omitted useful trail: %q", plain)
	}

	hostile := renderNavigatorPresentationRow(NavigatorRow{Prefix: []Segment{{Text: "  "}}, Label: "topic\x1b[31m\nnext", Suffix: []Segment{{Text: "  remote\rtrail", Tone: ToneQuiet}}}, 40, false, false, commitrow.Columns{}, time.Time{})
	plain := ansi.Strip(hostile)
	if strings.ContainsRune(plain, '\x1b') || strings.ContainsRune(plain, '\n') || strings.ContainsRune(plain, '\r') || !strings.Contains(plain, "␛") || !strings.Contains(plain, "↵") {
		t.Fatalf("hostile metadata row was not inert: %q", plain)
	}
}

func TestCommitRowSeamPrioritizesSubjectAndAcceptsGraphCells(t *testing.T) {
	t.Parallel()
	graph := commitgraph.Layout([]commitgraph.Commit{{OID: "oid", Merge: true}})
	row := commitrow.Row{
		Graph:    graph[0],
		OID:      "oid",
		ShortOID: "abcdef0",
		Subject:  "the commit subject is the primary content",
		Refs: []commitrow.Ref{
			{Kind: commitrow.Remote, Name: "origin/topic"},
			{Kind: commitrow.Tag, Name: "release"},
		},
		Author:       "A Very Long Author Name",
		AuthoredUnix: time.Now().Add(-48 * time.Hour).Unix(),
		Merge:        true,
	}
	narrow := ansi.Strip(renderCommitRow(row, commitrow.Measure([]commitrow.Row{row}, 30), 30, false, false, time.Now()))
	if !strings.Contains(narrow, "◎") || !strings.Contains(narrow, "abcdef0") || !strings.Contains(narrow, "the commit") {
		t.Fatalf("narrow commit row lost graph/SHA/subject: %q", narrow)
	}
	if strings.Contains(narrow, "origin/") || strings.Contains(narrow, "Author") {
		t.Fatalf("narrow commit row retained lower-priority metadata: %q", narrow)
	}
	if lipgloss.Width(narrow) != 30 {
		t.Fatalf("narrow commit row width = %d, want 30", lipgloss.Width(narrow))
	}

	wide := ansi.Strip(renderCommitRow(row, commitrow.Measure([]commitrow.Row{row}, 150), 150, false, false, time.Now()))
	for _, want := range []string{"the commit subject", "origin/topic", "release", "A Very Lon", "2d"} {
		if !strings.Contains(wide, want) {
			t.Fatalf("wide commit row lacks %q: %q", want, wide)
		}
	}
}

func TestRefsPaintAndHitGeometryShareFullRowsAndScrollbars(t *testing.T) {
	t.Parallel()
	g := Calculate(80, 12)
	rows := make([]NavigatorRow, 24)
	for index := range rows {
		rows[index] = NavigatorRow{Identity: "source", Prefix: []Segment{{Text: "  ", Tone: ToneAdded}}, Label: "source"}
	}
	commits := make([]commitrow.Row, 24)
	for index := range commits {
		commits[index] = commitrow.Row{Graph: commitgraph.Layout([]commitgraph.Commit{{OID: "commit"}})[0], OID: "commit", ShortOID: "aaaaaaa", Subject: "subject"}
	}
	frame := Render(Model{
		Geometry:         g,
		Workspace:        workspace.Git,
		Controls:         workspace.Controls{Git: workspace.GitRefs},
		NavigatorRows:    rows,
		Selected:         7,
		Top:              5,
		Focus:            navigation.FocusReader,
		ReaderCommitRows: commits,
		ReaderOffset:     4,
	})
	if !strings.Contains(ansi.Strip(frame), "▐") {
		t.Fatalf("Refs frame omitted scrollbar thumbs:\n%s", ansi.Strip(frame))
	}
	for _, x := range []int{g.NavigatorRows.X, g.NavigatorRows.X + g.NavigatorRows.Width - 2} {
		hit := g.HitTest(x, g.NavigatorRows.Y+2, workspace.Git, workspace.Controls{Git: workspace.GitRefs}, 5, len(rows), 4, len(commits))
		if hit.Kind != HitNavigatorRow || hit.Index != 7 {
			t.Fatalf("full source row hit at x=%d = %+v", x, hit)
		}
	}
	readerBar, ok := CalculateScrollbar(g.ReaderRows, len(commits), 4)
	if !ok {
		t.Fatal("reader scrollbar geometry missing")
	}
	if hit := g.HitTest(readerBar.Thumb.X, readerBar.Thumb.Y, workspace.Git, workspace.Controls{Git: workspace.GitRefs}, 5, len(rows), 4, len(commits)); hit.Kind != HitReaderScrollbar {
		t.Fatalf("reader thumb hit = %+v", hit)
	}
}

func refTestCommitRows() []commitrow.Row {
	now := time.Now().Unix()
	graphs := commitgraph.Layout([]commitgraph.Commit{{OID: "a", Parents: []string{"b"}}, {OID: "b"}})
	return []commitrow.Row{
		{Graph: graphs[0], OID: "a", ShortOID: "aaaaaaa", Subject: "tip subject", Refs: []commitrow.Ref{{Kind: commitrow.Branch, Name: "main"}}, Author: "Ada", AuthoredUnix: now - 2*60*60},
		{Graph: graphs[1], OID: "b", ShortOID: "bbbbbbb", Subject: "root subject"},
	}
}
