package commitgraph

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
	maxPos, commitPos := pipeSetBounds(pipes)
	var storage [maxTrackedPipes]cell
	cells := storage[:min(maxPos+1, maxTrackedPipes)]
	renderStartingPipes(cells, pipes)
	renderRemainingPipes(cells, pipes, commitPos)
	commitPos = min(commitPos, len(cells)-1)
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

func pipeSetBounds(pipes []pipe) (maxPos, commitPos int) {
	for _, item := range pipes {
		switch item.kind {
		case starts:
			commitPos = item.fromPos
		case terminates:
			commitPos = item.toPos
		}
		maxPos = max(maxPos, item.right())
	}
	return maxPos, commitPos
}

func renderStartingPipes(cells []cell, pipes []pipe) {
	for _, item := range pipes {
		if item.kind == starts {
			renderPipe(cells, item, true)
		}
	}
}

func renderRemainingPipes(cells []cell, pipes []pipe, commitPos int) {
	for _, item := range pipes {
		commitPoint := item.kind == terminates && item.fromPos == commitPos && item.toPos == commitPos
		if item.kind != starts && !commitPoint {
			renderPipe(cells, item, false)
		}
	}
}

func renderPipe(cells []cell, item pipe, overrideRightColor bool) {
	left, right := item.left(), item.right()
	if left < 0 || right >= len(cells) {
		return
	}
	if left != right {
		for index := left + 1; index < right; index++ {
			cells[index].setLeft(item.color)
			cells[index].setRight(item.color, overrideRightColor)
		}
		cells[left].setRight(item.color, overrideRightColor)
		cells[right].setLeft(item.color)
	}
	if item.kind == starts || item.kind == continues {
		cells[item.toPos].setDown(item.color)
	}
	if item.kind == terminates || item.kind == continues {
		cells[item.fromPos].setUp(item.color)
	}
}

var boxDrawingGlyphs = [16][2]rune{
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

func boxDrawingChars(connections uint8) (rune, rune) {
	return boxDrawingGlyphs[connections][0], boxDrawingGlyphs[connections][1]
}
