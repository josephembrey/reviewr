package ui

import (
	"testing"

	"github.com/josephembrey/reviewr/internal/workspace"
)

func TestGitGeometryKeepsHistoryDominantAndMakesInspectionResponsive(t *testing.T) {
	t.Parallel()
	wide := Calculate(120, 18)
	history := CalculateGitGeometry(wide, GitHistoryLayout, GitWidths{})
	if history.Rail.Width < MinimumPaneWidth || history.Content.Width <= history.Rail.Width ||
		history.PrimaryDivider.Width != 1 || history.Status.Height != 1 || history.Status.Width != history.Content.Width {
		t.Fatalf("history geometry = %+v", history)
	}
	if history.Rail.X != wide.Body.X || history.Content.X+history.Content.Width != wide.Body.X+wide.Body.Width {
		t.Fatalf("history geometry does not cover body: body %+v history %+v", wide.Body, history)
	}
	custom := CalculateGitGeometry(wide, GitHistoryLayout, GitWidths{Rail: 41})
	if custom.Rail.Width != 41 || custom.Content.Width != wide.Body.Width-42 {
		t.Fatalf("custom source width = %+v", custom)
	}

	commitWide := CalculateGitGeometry(wide, GitCommitLayout, GitWidths{})
	if commitWide.FilesStacked || commitWide.Content.Width <= commitWide.Files.Width || commitWide.SecondaryDivider.Width != 1 {
		t.Fatalf("wide commit inspection = %+v", commitWide)
	}
	commitNarrow := CalculateGitGeometry(Calculate(MinimumWidth, 18), GitCommitLayout, GitWidths{})
	if !commitNarrow.FilesStacked || commitNarrow.Files.Width != MinimumWidth || commitNarrow.Content.Width != MinimumWidth ||
		commitNarrow.Content.Height <= commitNarrow.Files.Height || commitNarrow.SecondaryDivider.Height != 1 {
		t.Fatalf("narrow commit inspection = %+v", commitNarrow)
	}

	stashes := CalculateGitGeometry(wide, GitStashesLayout, GitWidths{})
	if stashes.Rail.Width >= stashes.Content.Width || stashes.Files.Width >= stashes.Content.Width || stashes.FilesStacked {
		t.Fatalf("wide stash diff is not dominant: %+v", stashes)
	}
	stashesNarrow := CalculateGitGeometry(Calculate(MinimumWidth, MinimumHeight), GitStashesLayout, GitWidths{})
	if !stashesNarrow.FilesStacked || stashesNarrow.Rail.Width <= 0 || stashesNarrow.Files.Height <= 0 || stashesNarrow.Content.Height <= 0 {
		t.Fatalf("minimum stash geometry = %+v", stashesNarrow)
	}
}

func TestGitHitTestingGivesControlsDividersAndScrollbarsPaintPrecedence(t *testing.T) {
	t.Parallel()
	base := Calculate(100, 16)
	geometry := CalculateGitGeometry(base, GitHistoryLayout, GitWidths{})
	controls := workspace.Controls{Git: workspace.GitHistory, Traversal: workspace.GitGraph}
	state := GitHitState{RailCount: 100, ContentCount: 100}

	header := layoutHeaderControls(base, workspace.Git, controls)
	if len(header) != 2 {
		t.Fatalf("Git header controls = %+v", header)
	}
	for _, control := range header {
		hit := geometry.HitTest(control.rect.X, control.rect.Y, workspace.Git, controls, state)
		if hit.Kind != control.hit || hit.Region != 0 || hit.Divider != GitDividerNone {
			t.Fatalf("header hit = %+v, want %v", hit, control.hit)
		}
	}
	divider := geometry.HitTest(geometry.PrimaryDivider.X, geometry.PrimaryDivider.Y, workspace.Git, controls, state)
	if divider.Kind != HitDivider || divider.Divider != GitPrimaryDivider {
		t.Fatalf("primary divider hit = %+v", divider)
	}

	railBar, ok := CalculateScrollbar(geometry.RailRows, state.RailCount, state.RailTop)
	if !ok {
		t.Fatal("overflowing source rail produced no scrollbar")
	}
	hit := geometry.HitTest(railBar.Track.X, railBar.Track.Y, workspace.Git, controls, state)
	if hit.Kind != HitNavigatorScrollbar || hit.Region != workspace.GitSource {
		t.Fatalf("source scrollbar lost precedence to row = %+v", hit)
	}
	hit = geometry.HitTest(geometry.ContentRows.X, geometry.ContentRows.Y+1, workspace.Git, controls, state)
	if hit.Kind != HitNavigatorRow || hit.Region != workspace.GitTimeline || hit.Index != 1 {
		t.Fatalf("timeline row hit = %+v", hit)
	}
}
