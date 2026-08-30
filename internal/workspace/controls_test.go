package workspace

import "testing"

func TestTwoStateControlsToggleAndLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		first  string
		second string
		toggle func() (string, string)
	}{
		{
			name: "file set", first: "changed", second: "all",
			toggle: func() (string, string) {
				return AllFiles.Toggle().Label(), ChangedFiles.Toggle().Label()
			},
		},
		{
			name: "reader mode", first: "diff", second: "file",
			toggle: func() (string, string) {
				return FileReader.Toggle().Label(), DiffReader.Toggle().Label()
			},
		},
		{
			name: "diff highlight", first: "background", second: "sidebar",
			toggle: func() (string, string) {
				return DiffHighlightSidebar.Toggle().Label(), DiffHighlightBackground.Toggle().Label()
			},
		},
		{
			name: "Git traversal", first: "first-parent", second: "graph",
			toggle: func() (string, string) {
				return GitGraph.Toggle().Label(), GitFirstParent.Toggle().Label()
			},
		},
	}
	for _, test := range tests {
		first, second := test.toggle()
		if first != test.first || second != test.second {
			t.Errorf("%s toggle labels = %q, %q; want %q, %q", test.name, first, second, test.first, test.second)
		}
	}
}

func TestMultiStateControlsCycleInDisplayOrder(t *testing.T) {
	t.Parallel()
	comparison := Uncommitted
	for _, want := range []string{"uncommitted", "branch", "last-turn", "uncommitted"} {
		if got := comparison.Label(); got != want {
			t.Fatalf("comparison label = %q, want %q", got, want)
		}
		comparison = comparison.Next()
	}

	view := GitLog
	for _, want := range []string{"log", "refs", "stashes", "log"} {
		if got := view.Label(); got != want {
			t.Fatalf("Git view label = %q, want %q", got, want)
		}
		view = view.Next()
	}
}
