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
	wide := geometry.Header.Width >= wideHeaderControls
	gap := 1
	if wide {
		gap = 2
	}
	if active == workspace.Files {
		return layoutFilesHeaderControls(geometry, controls, wide, gap)
	}

	definitions := make([]headerControl, 0, 3)
	switch active {
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

	position := geometry.HeaderSwitcher.X + geometry.HeaderSwitcher.Width
	visible := make([]headerControl, 0, len(definitions))
	for _, control := range definitions {
		position += gap
		controlWidth := headerControlWidth(control, wide)
		if position+controlWidth > geometry.Header.X+geometry.Header.Width {
			break
		}
		control.rect = Rect{X: position, Y: geometry.Header.Y, Width: controlWidth, Height: geometry.Header.Height}
		visible = append(visible, control)
		position += controlWidth
	}
	return visible
}

func layoutFilesHeaderControls(geometry Geometry, controls workspace.Controls, wide bool, gap int) []headerControl {
	comparison := headerControl{hit: HitComparisonControl, key: workspace.ComparisonControlKey, value: controls.Comparison.Label()}
	comparison.rect = Rect{
		X:      geometry.Header.X + geometry.Header.Width - headerControlWidth(comparison, wide),
		Y:      geometry.Header.Y,
		Width:  headerControlWidth(comparison, wide),
		Height: geometry.Header.Height,
	}
	leftX := geometry.HeaderSwitcher.X + geometry.HeaderSwitcher.Width + gap
	if comparison.rect.X < leftX {
		return nil
	}

	visible := make([]headerControl, 0, 2)
	if controls.RichDiff {
		highlight := headerControl{hit: HitDiffHighlightControl, key: workspace.DiffHighlightKey, value: controls.DiffHighlight.Label()}
		highlightWidth := headerControlWidth(highlight, wide)
		if leftX+highlightWidth+gap <= comparison.rect.X {
			highlight.rect = Rect{X: leftX, Y: geometry.Header.Y, Width: highlightWidth, Height: geometry.Header.Height}
			visible = append(visible, highlight)
		}
	}
	return append(visible, comparison)
}

func headerControlWidth(control headerControl, wide bool) int {
	width := len(control.value) + 2
	if wide {
		width += len(control.key) + 1
	}
	return width
}

type paneHeaderControls struct {
	navigator headerControl
	reader    headerControl
}

func layoutPaneHeaderControls(geometry Geometry, active workspace.Kind, controls workspace.Controls) paneHeaderControls {
	if active != workspace.Files {
		return paneHeaderControls{}
	}
	return paneHeaderControls{
		navigator: layoutRightPaneControl(
			geometry.NavigatorTitle,
			HitSecondaryControl,
			workspace.SecondaryControlKey,
			controls.Files.Label(),
		),
		reader: layoutRightPaneControl(
			geometry.ReaderTitle,
			HitTertiaryControl,
			workspace.TertiaryControlKey,
			controls.Reader.Label(),
		),
	}
}

func layoutRightPaneControl(title Rect, hit HitKind, key, value string) headerControl {
	width := len(key) + 1 + len(value) + 2
	if title.Height == 0 || title.Width < width+2 {
		return headerControl{}
	}
	return headerControl{
		hit: hit, key: key, value: value,
		rect: Rect{X: title.X + title.Width - width, Y: title.Y, Width: width, Height: title.Height},
	}
}
