// Package commitrow defines the narrow, renderer-neutral presentation model
// shared by commit lists.
package commitrow

import (
	"strconv"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/josephembrey/reviewr/internal/commitgraph"
)

const (
	SHAWidth      = 7
	AgeWidth      = 3
	AuthorCap     = 10
	RefTrailCap   = 30
	minimumProse  = 12
	preferredText = 22
)

// RefKind gives a label a semantic icon and color at paint time.
type RefKind uint8

const (
	Branch RefKind = iota
	Remote
	Tag
)

// Ref is one semantic public-ref label.
type Ref struct {
	Kind RefKind
	Name string
}

// Row contains commit facts without terminal styling.
type Row struct {
	Graph        commitgraph.Row
	OID          string
	ShortOID     string
	Subject      string
	Author       string
	AuthoredUnix int64
	Refs         []Ref
	Merge        bool
}

// Columns is one responsive set of widths shared across a visible commit
// universe so graph and author columns do not jitter between rows.
type Columns struct {
	Graph  int
	Author int
	Age    int
}

// Measure derives responsive columns while preserving subject space ahead of
// ref trails and slowly growing author names.
func Measure(rows []Row, width int) Columns {
	if width <= 0 {
		return Columns{}
	}
	graphWidth := 0
	authorWidth := 0
	for _, row := range rows {
		graphWidth = max(graphWidth, row.Graph.Width())
		authorWidth = max(authorWidth, ansi.StringWidth(row.Author))
	}
	graphBudget := max(2, width/4)
	graphWidth = min(graphWidth, commitgraph.MaxWidth, graphBudget)

	// Graph, SHA, two gaps, minimum subject, and age have priority over author.
	fixedWithoutAuthor := graphWidth + SHAWidth + 2 + minimumProse + 2 + AgeWidth
	ageWidth := 0
	if width >= fixedWithoutAuthor {
		ageWidth = AgeWidth
	}
	spare := width - (graphWidth + SHAWidth + 2 + minimumProse)
	if ageWidth > 0 {
		spare -= 2 + ageWidth
	}
	// One author cell is earned for every three spare cells.
	responsiveAuthor := max(0, spare/3)
	authorWidth = min(authorWidth, AuthorCap, responsiveAuthor)
	return Columns{Graph: graphWidth, Author: authorWidth, Age: ageWidth}
}

// TrailWidth reports the complete unstyled ref trail width, including the
// leading two-cell separation and inter-label separators.
func TrailWidth(row Row) int {
	if len(row.Refs) == 0 {
		if !row.Merge {
			return 0
		}
		return len("  merge")
	}
	width := 2
	for index, reference := range row.Refs {
		if index > 0 {
			width += 3
		}
		width += 2 + ansi.StringWidth(reference.Name)
	}
	if row.Merge {
		width += 3 + len("merge")
	}
	return width
}

// TrailReserve bounds decorations and only grants them room left after the
// preferred subject allocation.
func TrailReserve(row Row, contentWidth int) int {
	if contentWidth <= preferredText {
		return 0
	}
	return min(TrailWidth(row), RefTrailCap, contentWidth-preferredText)
}

// AgeLabel formats a compact terminal age in at most AgeWidth cells.
func AgeLabel(now time.Time, authoredUnix int64) string {
	if authoredUnix <= 0 {
		return "?"
	}
	seconds := max(int64(0), now.Unix()-authoredUnix)
	switch {
	case seconds < 60:
		return "now"
	case seconds < 60*60:
		return strconv.FormatInt(seconds/60, 10) + "m"
	case seconds < 24*60*60:
		return strconv.FormatInt(seconds/(60*60), 10) + "h"
	case seconds < 7*24*60*60:
		return strconv.FormatInt(seconds/(24*60*60), 10) + "d"
	case seconds < 52*7*24*60*60:
		return strconv.FormatInt(seconds/(7*24*60*60), 10) + "w"
	default:
		years := max(int64(1), min(int64(99), seconds/(365*24*60*60)))
		return strconv.FormatInt(years, 10) + "y"
	}
}
