//go:build dev

package lab

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

const foldMotionLineCount = 8

type foldMotionTick struct {
	generation uint64
}

type foldMotionSpeedSpec struct {
	label string
	delay time.Duration
}

var foldMotionSpeeds = []foldMotionSpeedSpec{
	{label: "fast · 20ms/row", delay: 20 * time.Millisecond},
	{label: "medium · 32ms/row", delay: 32 * time.Millisecond},
	{label: "slow · 48ms/row", delay: 48 * time.Millisecond},
}

var foldMotionBefore = []foldPreviewRow{
	{line: 34, text: "func resolveReview(path string) ReviewState {"},
	{line: 35, text: "state := ledger.Assess(path)"},
	{line: 36, text: ""},
}

var foldMotionContextRows = []foldPreviewRow{
	{line: 37, text: "if state.BasisChanged() {"},
	{line: 38, text: "return ReviewBasisChanged"},
	{line: 39, text: "}"},
	{line: 40, text: "if state.HasGap() {"},
	{line: 41, text: "return ReviewPartial"},
	{line: 42, text: "}"},
	{line: 43, text: ""},
	{line: 44, text: "frontier := state.Frontier()"},
}

var foldMotionAfter = []foldPreviewRow{
	{line: 45, text: "return previous", tone: foldRemoved},
	{line: 45, text: "return frontier", tone: foldAdded},
	{line: 46, text: "}"},
	{line: 47, text: ""},
	{line: 48, text: "func nextReviewGap() string {"},
	{line: 49, text: "return queue.Next()"},
	{line: 50, text: "}"},
}

func (model Model) updateFoldMotion(key tea.KeyPressMsg) (Model, tea.Cmd) {
	switch key.String() {
	case "j", "down":
		model.foldMotionSpeed = min(model.foldMotionSpeed+1, len(foldMotionSpeeds)-1)
	case "k", "up":
		model.foldMotionSpeed = max(model.foldMotionSpeed-1, 0)
	case "h", "left":
		return model.startFoldMotion(0)
	case "l", "right":
		return model.startFoldMotion(foldMotionLineCount)
	case "enter", " ":
		target := foldMotionLineCount
		if model.foldMotionTarget == foldMotionLineCount {
			target = 0
		}
		return model.startFoldMotion(target)
	}
	return model, nil
}

func (model Model) startFoldMotion(target int) (Model, tea.Cmd) {
	model.foldMotionGeneration++
	model.foldMotionTarget = max(0, min(foldMotionLineCount, target))
	model = model.stepFoldMotion()
	if model.foldMotionVisible == model.foldMotionTarget {
		return model, nil
	}
	return model, model.nextFoldMotionFrame()
}

func (model Model) updateFoldMotionTick(tick foldMotionTick) (Model, tea.Cmd, bool) {
	if tick.generation != model.foldMotionGeneration || model.page != labPageFoldMotion {
		return model, nil, true
	}
	model = model.stepFoldMotion()
	if model.foldMotionVisible == model.foldMotionTarget {
		return model, nil, true
	}
	return model, model.nextFoldMotionFrame(), true
}

func (model Model) stepFoldMotion() Model {
	switch {
	case model.foldMotionVisible < model.foldMotionTarget:
		model.foldMotionVisible++
	case model.foldMotionVisible > model.foldMotionTarget:
		model.foldMotionVisible--
	}
	return model
}

func (model Model) nextFoldMotionFrame() tea.Cmd {
	generation := model.foldMotionGeneration
	delay := foldMotionSpeeds[model.foldMotionSpeed].delay
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return foldMotionTick{generation: generation}
	})
}

func (model Model) viewFoldMotion(width, height int) string {
	width = max(0, width)
	height = max(0, height)
	lines := []string{
		title.Render("lab / fold motion"),
		quiet.Render("tab next page  •  h/l collapse/expand  •  enter toggle  •  j/k speed  •  ctrl+l or esc close"),
		renderFoldMotionSpeeds(model),
		quiet.Render("Watch the edited return and the lower function move; they remain visible as spatial anchors."),
		"",
	}
	for _, row := range foldMotionBefore {
		lines = append(lines, renderFoldPreviewRow(row))
	}
	lines = append(lines, renderFoldMotionControl(model))
	for _, row := range foldMotionContextRows[:model.foldMotionVisible] {
		lines = append(lines, renderFoldPreviewRow(row))
	}
	for _, row := range foldMotionAfter {
		lines = append(lines, renderFoldPreviewRow(row))
	}
	lines = append(lines, "", quiet.Render("Prototype only: production folding remains instantaneous."))
	return fitPage(lines, width, height)
}

func renderFoldMotionSpeeds(model Model) string {
	parts := make([]string, len(foldMotionSpeeds))
	for index, speed := range foldMotionSpeeds {
		label := speed.label
		style := quiet
		if index == model.foldMotionSpeed {
			label = "[" + label + "]"
			style = title
		}
		parts[index] = style.Render(label)
	}
	return strings.Join(parts, quiet.Render("  |  "))
}

func renderFoldMotionControl(model Model) string {
	direction, glyph := "compact", "▸"
	switch {
	case model.foldMotionVisible < model.foldMotionTarget:
		direction, glyph = "opening", "▾"
	case model.foldMotionVisible > model.foldMotionTarget:
		direction, glyph = "closing", "▴"
	case model.foldMotionVisible == foldMotionLineCount:
		direction, glyph = "expanded", "▾"
	}
	progress := fmt.Sprintf("%d/%d", model.foldMotionVisible, foldMotionLineCount)
	return title.Render("── "+glyph+" 8 unchanged lines") + quiet.Render("  ·  "+direction+" "+progress+" "+strings.Repeat("─", 12))
}
