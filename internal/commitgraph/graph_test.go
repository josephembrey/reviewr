package commitgraph

import (
	"slices"
	"strings"
	"testing"
)

func graph(commits []struct {
	oid     string
	parents []string
}) []string {
	input := make([]Commit, len(commits))
	for index, item := range commits {
		input[index] = Commit{OID: item.oid, Parents: item.parents, Merge: len(item.parents) > 1}
	}
	rows := Layout(input)
	result := make([]string, len(rows))
	for index, row := range rows {
		result[index] = strings.TrimRight(row.Text(), " ")
	}
	return result
}

func fixture(oid string, parents ...string) struct {
	oid     string
	parents []string
} {
	return struct {
		oid     string
		parents []string
	}{oid: oid, parents: parents}
}

func TestLinearRootsAndOffWindowParents(t *testing.T) {
	t.Parallel()
	if got := graph([]struct {
		oid     string
		parents []string
	}{fixture("1", "2"), fixture("2", "3"), fixture("3")}); !slices.Equal(got, []string{"○", "○", "○"}) {
		t.Fatalf("linear graph = %q", got)
	}
	if got := graph([]struct {
		oid     string
		parents []string
	}{fixture("A", "missing"), fixture("X")}); !slices.Equal(got, []string{"○", "│ ○"}) {
		t.Fatalf("off-window graph = %q", got)
	}
	if got := graph([]struct {
		oid     string
		parents []string
	}{fixture("A"), fixture("X", "Y"), fixture("Y")}); !slices.Equal(got, []string{"○", "│ ○", "│ ○"}) {
		t.Fatalf("disconnected roots = %q", got)
	}
}

func TestMergeFanOutAndLaneMotion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		commits []struct {
			oid     string
			parents []string
		}
		want []string
	}{
		{
			name: "simple fan out",
			commits: []struct {
				oid     string
				parents []string
			}{fixture("1", "2", "3"), fixture("3", "2"), fixture("2")},
			want: []string{"◎─╮", "│ ○", "○─╯"},
		},
		{
			name: "lane motion",
			commits: []struct {
				oid     string
				parents []string
			}{
				fixture("1", "2"),
				fixture("2", "3", "4"),
				fixture("4", "3", "5"),
				fixture("3", "5"),
				fixture("5", "6"),
				fixture("6", "7"),
			},
			want: []string{"○", "◎─╮", "│ ◎─╮", "○─╯ │", "○───╯", "○"},
		},
		{
			name: "octopus",
			commits: []struct {
				oid     string
				parents []string
			}{
				fixture("1", "2", "3", "4", "5"),
				fixture("4", "2"),
				fixture("2", "A"),
				fixture("A", "6", "B"),
				fixture("B", "C"),
			},
			want: []string{"◎─┬─┬─╮", "│ │ ○ │", "○─│─╯ │", "◎─│─╮ │", "│ │ ○ │"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := graph(test.commits); !slices.Equal(got, test.want) {
				t.Fatalf("graph = %q, want %q", got, test.want)
			}
		})
	}
}

func TestMergeNodeSurvivesFirstParentProjection(t *testing.T) {
	t.Parallel()
	rows := Layout([]Commit{{OID: "merge", Parents: []string{"main"}, Merge: true}})
	if got := strings.TrimRight(rows[0].Text(), " "); got != "◎" {
		t.Fatalf("merge projection = %q", got)
	}
}

func TestColorIdentityCrossesHorizontalJunction(t *testing.T) {
	t.Parallel()
	rows := Layout([]Commit{
		{OID: "1", Parents: []string{"2", "3"}, Merge: true},
		{OID: "3", Parents: []string{"2"}},
		{OID: "2"},
	})
	colors := make(map[Color]struct{})
	for _, cell := range rows[0].Cells {
		if cell.GlyphColored {
			colors[cell.GlyphColor] = struct{}{}
		}
		if cell.HorizontalColored {
			colors[cell.HorizontalColor] = struct{}{}
		}
	}
	if len(colors) != 1 {
		t.Fatalf("merge edge colors = %v, want one identity", colors)
	}
	foundUncoloredSpace := false
	for _, row := range rows {
		for _, cell := range row.Cells {
			foundUncoloredSpace = foundUncoloredSpace || cell.Horizontal == ' ' && !cell.HorizontalColored
		}
	}
	if !foundUncoloredSpace {
		t.Fatal("blank graph cells all carried false color")
	}
}

func TestLayoutIsDeterministicAndCapsWideFanOut(t *testing.T) {
	t.Parallel()
	parents := make([]string, 512)
	for index := range parents {
		parents[index] = "parent-" + strings.Repeat("x", index%17)
	}
	commits := []Commit{{OID: "octopus", Parents: parents, Merge: true}}
	first := Layout(commits)
	second := Layout(commits)
	if len(first) != 1 || first[0].Width() != MaxWidth {
		t.Fatalf("wide graph dimensions = %d rows, %d cells", len(first), first[0].Width())
	}
	if first[0].Text() != second[0].Text() {
		t.Fatalf("wide graph is nondeterministic: %q != %q", first[0].Text(), second[0].Text())
	}
}
