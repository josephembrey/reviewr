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
	current, maxPos := livePipes(previous)
	pos := commitLane(current, commit, maxPos)
	continuingSpots := continuingLanes(current, commit)
	var taken, traversed laneSet
	firstParent := emptyTree
	if len(parents) > 0 {
		firstParent = parents[0]
	}
	next := make([]pipe, 0, min(maxTrackedPipes, len(current)+len(parents)+1))
	next = append(next, pipe{
		from: commit, to: firstParent, fromPos: pos, toPos: pos, kind: starts, color: Color(pos),
	})
	next = routePipesThroughCommit(next, current, commit, pos, &taken, &traversed)
	if len(parents) > 1 {
		next = startAdditionalParents(next, parents[1:], commit, pos, &taken, &continuingSpots)
	}
	next = routePipesRightOfCommit(next, current, commit, pos, &taken, &traversed)

	sortPipes(next)
	return next
}

type laneSet [maxTrackedPipes]bool

func livePipes(previous []pipe) ([]pipe, int) {
	maxPos := 0
	current := make([]pipe, 0, len(previous))
	for _, item := range previous {
		maxPos = max(maxPos, item.toPos)
		if item.kind != terminates {
			current = append(current, item)
		}
	}
	return current, maxPos
}

func commitLane(pipes []pipe, commit string, maxPos int) int {
	for _, item := range pipes {
		if item.to == commit {
			return item.toPos
		}
	}
	return min(maxPos+1, maxTrackedLane)
}

func continuingLanes(pipes []pipe, commit string) laneSet {
	var lanes laneSet
	for _, item := range pipes {
		if item.to != commit {
			lanes[item.toPos] = true
		}
	}
	return lanes
}

func routePipesThroughCommit(next, current []pipe, commit string, pos int, taken, traversed *laneSet) []pipe {
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
	return next
}

func startAdditionalParents(next []pipe, parents []string, commit string, pos int, taken, continuing *laneSet) []pipe {
	for _, parent := range parents {
		if len(next) >= maxTrackedPipes {
			break
		}
		available, ok := nextAvailableNew(taken, continuing)
		if !ok {
			break
		}
		next = append(next, pipe{
			from: commit, to: parent, fromPos: pos, toPos: available, kind: starts, color: Color(pos),
		})
		taken[available] = true
	}
	return next
}

func routePipesRightOfCommit(next, current []pipe, commit string, pos int, taken, traversed *laneSet) []pipe {
	for _, item := range current {
		if len(next) >= maxTrackedPipes {
			break
		}
		if item.to == commit || item.toPos <= pos {
			continue
		}
		item.fromPos = item.toPos
		item.toPos = slideLeft(item.toPos, pos, taken, traversed)
		item.kind = continues
		next = append(next, item)
		traverse(item.fromPos, item.toPos, taken, traversed)
	}
	return next
}

func slideLeft(from, boundary int, taken, traversed *laneSet) int {
	last := from
	for candidate := from; candidate > boundary; candidate-- {
		if taken[candidate] || traversed[candidate] {
			break
		}
		last = candidate
	}
	return last
}

func nextAvailableContinuing(traversed *laneSet) int {
	for spot := 0; spot <= maxTrackedLane; spot++ {
		if !traversed[spot] {
			return spot
		}
	}
	return maxTrackedLane
}

func nextAvailableNew(taken, continuing *laneSet) (int, bool) {
	for spot := 0; spot <= maxTrackedLane; spot++ {
		if !taken[spot] && !continuing[spot] {
			return spot, true
		}
	}
	return 0, false
}

func traverse(from, to int, taken, traversed *laneSet) {
	for spot := min(from, to); spot <= max(from, to); spot++ {
		traversed[spot] = true
	}
	taken[to] = true
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
