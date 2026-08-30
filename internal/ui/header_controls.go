package ui

import "github.com/josephembrey/reviewr/internal/workspace"

const wideHeaderControls = 96

type headerControl struct {
	hit   HitKind
	key   string
	value string
	rect  Rect
}

func layoutHeaderControls(geometry Geometry, active workspace.Kind, controls workspace.Controls) []headerControl {
	definitions := make([]headerControl, 0, 4)
	switch active {
	case workspace.Files:
		definitions = append(definitions,
			headerControl{hit: HitSecondaryControl, key: workspace.SecondaryControlKey, value: controls.Files.Label()},
			headerControl{hit: HitTertiaryControl, key: workspace.TertiaryControlKey, value: controls.Reader.Label()},
			headerControl{hit: HitComparisonControl, key: workspace.ComparisonControlKey, value: controls.Comparison.Label()},
		)
	case workspace.Git:
		definitions = append(definitions,
			headerControl{hit: HitSecondaryControl, key: workspace.SecondaryControlKey, value: controls.Git.Label()},
		)
		if controls.Git == workspace.GitLog {
			definitions = append(definitions,
				headerControl{hit: HitTertiaryControl, key: workspace.TertiaryControlKey, value: controls.Traversal.Label()},
			)
		}
	}
	if controls.RichDiff {
		definitions = append(definitions, headerControl{
			hit: HitDiffHighlightControl, key: workspace.DiffHighlightKey, value: controls.DiffHighlight.Label(),
		})
	}

	wide := geometry.Header.Width >= wideHeaderControls
	gap := 1
	if wide {
		gap = 2
	}
	position := geometry.HeaderSwitcher.X + geometry.HeaderSwitcher.Width
	visible := make([]headerControl, 0, len(definitions))
	for _, control := range definitions {
		position += gap
		controlWidth := len(control.value) + 2
		if wide {
			controlWidth += len(control.key) + 1
		}
		if position+controlWidth > geometry.Header.X+geometry.Header.Width {
			break
		}
		control.rect = Rect{X: position, Y: geometry.Header.Y, Width: controlWidth, Height: geometry.Header.Height}
		visible = append(visible, control)
		position += controlWidth
	}
	return visible
}
