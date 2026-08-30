// Package commitgraph lays out a compact, one-row-per-commit graph.
//
// The pipe transition and two-character cell grammar are substantially
// adapted from LazyGit's pkg/gui/presentation/graph/graph.go and cell.go at
// commit c300c319f954d740b67ed8db4d44ea18551ff231. LazyGit is Copyright (c)
// 2018 Jesse Duffield and licensed under the MIT License. See
// THIRD_PARTY_NOTICES.md.
//
// The package is deliberately ignorant of Git, ANSI escapes, and terminal
// themes. Callers supply ordered commits and parents and receive glyphs with
// stable lane color identities.
package commitgraph

const (
	// MaxWidth is the largest graph presentation exposed to a row renderer.
	MaxWidth = 16
	maxLanes = MaxWidth / 2
	// Tracking more lanes than can be painted preserves nearby convergence
	// while bounding pathological octopus merges and disconnected roots.
	maxTrackedPipes = 64
	maxTrackedLane  = maxTrackedPipes - 1
)

const (
	emptyTree = "EMPTY_TREE"
	start     = "START"
)

// Color is one stable pipe color identity. Renderers rotate it through their
// terminal-aware palette.
type Color uint

// Cell is one two-character graph lane.
type Cell struct {
	Glyph             rune
	GlyphColor        Color
	GlyphColored      bool
	Horizontal        rune
	HorizontalColor   Color
	HorizontalColored bool
}

// Row is one commit's compact graph presentation.
type Row struct {
	Cells []Cell
}

// Width returns the display-cell width of the row.
func (row Row) Width() int { return len(row.Cells) * 2 }

// Text returns plain graph glyphs for tests and non-terminal consumers.
func (row Row) Text() string {
	result := make([]rune, 0, row.Width())
	for _, cell := range row.Cells {
		result = append(result, cell.Glyph, cell.Horizontal)
	}
	return string(result)
}

// Commit is one ordered commit projected into the traversal being laid out.
// Merge describes the commit even when Parents contains only its first parent.
type Commit struct {
	OID     string
	Parents []string
	Merge   bool
}

type pipeKind uint8

const (
	terminates pipeKind = iota
	starts
	continues
)

type pipe struct {
	from    string
	to      string
	fromPos int
	toPos   int
	kind    pipeKind
	color   Color
}

func (p pipe) left() int  { return min(p.fromPos, p.toPos) }
func (p pipe) right() int { return max(p.fromPos, p.toPos) }

// Layout returns exactly one bounded graph row for every input commit.
// Parents absent from the input remain live targets, preserving shallow and
// off-window boundaries instead of falsely turning them into roots.
func Layout(commits []Commit) []Row {
	if len(commits) == 0 {
		return nil
	}
	pipes := []pipe{{
		from: start, to: commits[0].OID, kind: starts, color: 0,
	}}
	rows := make([]Row, 0, len(commits))
	for _, commit := range commits {
		pipes = nextPipes(pipes, commit.OID, commit.Parents)
		rows = append(rows, renderPipeSet(pipes, commit.Merge))
	}
	return rows
}

func nextPipes(previous []pipe, commit string, parents []string) []pipe {
	maxPos := 0
	current := make([]pipe, 0, len(previous))
	for _, item := range previous {
		maxPos = max(maxPos, item.toPos)
		if item.kind != terminates {
			current = append(current, item)
		}
	}
	pos := min(maxPos+1, maxTrackedLane)
	for _, item := range current {
		if item.to == commit {
			pos = item.toPos
			break
		}
	}

	taken := make(map[int]struct{})
	traversed := make(map[int]struct{})
	continuingSpots := make(map[int]struct{})
	for _, item := range current {
		if item.to != commit {
			continuingSpots[item.toPos] = struct{}{}
		}
	}
	firstParent := emptyTree
	if len(parents) > 0 {
		firstParent = parents[0]
	}
	next := make([]pipe, 0, min(maxTrackedPipes, len(current)+len(parents)+1))
	next = append(next, pipe{
		from: commit, to: firstParent, fromPos: pos, toPos: pos, kind: starts, color: Color(pos),
	})

	for _, item := range current {
		if len(next) >= maxTrackedPipes {
			break
		}
		if item.to == commit {
			item.fromPos = item.toPos
			item.toPos = pos
			item.kind = terminates
			next = append(next, item)
			traverse(item.fromPos, pos, taken, traversed)
		} else if item.toPos < pos {
			available := nextAvailableContinuing(traversed)
			item.fromPos = item.toPos
			item.toPos = available
			item.kind = continues
			next = append(next, item)
			traverse(item.fromPos, available, taken, traversed)
		}
	}

	additionalParents := parents
	if len(additionalParents) > 0 {
		additionalParents = additionalParents[1:]
	}
	for _, parent := range additionalParents {
		if len(next) >= maxTrackedPipes {
			break
		}
		available, ok := nextAvailableNew(taken, continuingSpots)
		if !ok {
			break
		}
		next = append(next, pipe{
			from: commit, to: parent, fromPos: pos, toPos: available, kind: starts, color: Color(pos),
		})
		taken[available] = struct{}{}
	}

	for _, item := range current {
		if len(next) >= maxTrackedPipes {
			break
		}
		if item.to == commit || item.toPos <= pos {
			continue
		}
		last := item.toPos
		for candidate := item.toPos; candidate > pos; candidate-- {
			if _, exists := taken[candidate]; exists {
				break
			}
			if _, exists := traversed[candidate]; exists {
				break
			}
			last = candidate
		}
		item.fromPos = item.toPos
		item.toPos = last
		item.kind = continues
		next = append(next, item)
		traverse(item.fromPos, last, taken, traversed)
	}

	sortPipes(next)
	return next
}

func nextAvailableContinuing(traversed map[int]struct{}) int {
	for spot := 0; spot <= maxTrackedLane; spot++ {
		if _, exists := traversed[spot]; !exists {
			return spot
		}
	}
	return maxTrackedLane
}

func nextAvailableNew(taken, continuing map[int]struct{}) (int, bool) {
	for spot := 0; spot <= maxTrackedLane; spot++ {
		_, isTaken := taken[spot]
		_, isContinuing := continuing[spot]
		if !isTaken && !isContinuing {
			return spot, true
		}
	}
	return 0, false
}

func traverse(from, to int, taken, traversed map[int]struct{}) {
	for spot := min(from, to); spot <= max(from, to); spot++ {
		traversed[spot] = struct{}{}
	}
	taken[to] = struct{}{}
}

func sortPipes(pipes []pipe) {
	for index := 1; index < len(pipes); index++ {
		for current := index; current > 0; current-- {
			left, right := pipes[current-1], pipes[current]
			if left.toPos < right.toPos || left.toPos == right.toPos && left.kind <= right.kind {
				break
			}
			pipes[current-1], pipes[current] = pipes[current], pipes[current-1]
		}
	}
}

type cellKind uint8

const (
	connection cellKind = iota
	commitNode
	mergeNode
)

type cell struct {
	connections uint8
	kind        cellKind
	color       Color
	rightColor  Color
	rightSet    bool
}

const (
	up    uint8 = 0b1000
	down  uint8 = 0b0100
	left  uint8 = 0b0010
	right uint8 = 0b0001
)

func (c cell) has(direction uint8) bool { return c.connections&direction != 0 }

func (c *cell) setUp(color Color) {
	c.connections |= up
	c.color = color
}

func (c *cell) setDown(color Color) {
	c.connections |= down
	c.color = color
}

func (c *cell) setLeft(color Color) {
	c.connections |= left
	if !c.has(up) && !c.has(down) {
		c.color = color
	}
}

func (c *cell) setRight(color Color, override bool) {
	c.connections |= right
	if !c.rightSet || override {
		c.rightColor = color
		c.rightSet = true
	}
}

func (c cell) rendered() Cell {
	glyph, horizontal := boxDrawingChars(c.connections)
	switch c.kind {
	case commitNode:
		glyph = '○'
	case mergeNode:
		glyph = '◎'
	}
	horizontalColor := c.color
	if c.rightSet {
		horizontalColor = c.rightColor
	}
	return Cell{
		Glyph:             glyph,
		GlyphColor:        c.color,
		GlyphColored:      glyph != ' ',
		Horizontal:        horizontal,
		HorizontalColor:   horizontalColor,
		HorizontalColored: horizontal != ' ',
	}
}

func renderPipeSet(pipes []pipe, merge bool) Row {
	maxPos := 0
	commitPos := 0
	for _, item := range pipes {
		if item.kind == starts {
			commitPos = item.fromPos
		} else if item.kind == terminates {
			commitPos = item.toPos
		}
		maxPos = max(maxPos, item.right())
	}
	cells := make([]cell, min(maxPos+1, maxTrackedPipes))
	for _, item := range pipes {
		if item.kind == starts {
			renderPipe(cells, item, true)
		}
	}
	for _, item := range pipes {
		if item.kind != starts && !(item.kind == terminates && item.fromPos == commitPos && item.toPos == commitPos) {
			renderPipe(cells, item, false)
		}
	}
	if commitPos >= len(cells) {
		commitPos = len(cells) - 1
	}
	if merge {
		cells[commitPos].kind = mergeNode
	} else {
		cells[commitPos].kind = commitNode
	}
	visible := min(len(cells), maxLanes)
	row := Row{Cells: make([]Cell, visible)}
	for index := range row.Cells {
		row.Cells[index] = cells[index].rendered()
	}
	return row
}

func renderPipe(cells []cell, item pipe, overrideRightColor bool) {
	if item.left() < 0 || item.right() >= len(cells) {
		return
	}
	if item.left() != item.right() {
		for index := item.left() + 1; index < item.right(); index++ {
			cells[index].setLeft(item.color)
			cells[index].setRight(item.color, overrideRightColor)
		}
		cells[item.left()].setRight(item.color, overrideRightColor)
		cells[item.right()].setLeft(item.color)
	}
	if item.kind == starts || item.kind == continues {
		cells[item.toPos].setDown(item.color)
	}
	if item.kind == terminates || item.kind == continues {
		cells[item.fromPos].setUp(item.color)
	}
}

func boxDrawingChars(connections uint8) (rune, rune) {
	glyphs := [16][2]rune{
		{' ', ' '},
		{'╶', '─'},
		{'─', ' '},
		{'─', '─'},
		{'╷', ' '},
		{'╭', '─'},
		{'╮', ' '},
		{'┬', '─'},
		{'╵', ' '},
		{'╰', '─'},
		{'╯', ' '},
		{'┴', '─'},
		{'│', ' '},
		{'│', '─'},
		{'│', ' '},
		{'│', '─'},
	}
	return glyphs[connections][0], glyphs[connections][1]
}
