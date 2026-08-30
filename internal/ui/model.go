package ui

import (
	"github.com/josephembrey/reviewr/internal/navigation"
	"github.com/josephembrey/reviewr/internal/workspace"
)

// Tone is semantic text emphasis chosen before terminal styling.
type Tone uint8

const (
	ToneDefault Tone = iota
	ToneQuiet
	ToneError
	ToneAccent
	ToneAdded
	ToneRemoved
	ToneInfo
	ToneWarning
)

// Line is one logical reader or empty-state line.
type Line struct {
	Text string
	Tone Tone
}

// Segment is one styled, single-line fragment in a compact row.
type Segment struct {
	Text string
	Tone Tone
}

// NavigatorRow separates stable identity from its display label.
type NavigatorRow struct {
	Identity  string
	Label     string
	Prefix    []Segment
	Suffix    []Segment
	Tree      bool
	Depth     int
	Directory bool
	Expanded  bool
}

// CommitRow is the reusable compact history presentation seam shared by Git
// surfaces. Lane accepts graph cells from the Log component; Refs may supply a
// single fallback node without implementing graph layout itself.
type CommitRow struct {
	Identity    string
	Lane        []Segment
	ShortOID    string
	Subject     string
	Decorations []Segment
	Author      string
	Age         string
}

// ChangeSummary is the aggregate worktree status shown in the header.
type ChangeSummary struct {
	Files     uint64
	Additions uint64
	Deletions uint64
	Ready     bool
}

// Model contains only the workspace-neutral derived state needed to paint a frame.
type Model struct {
	Geometry         Geometry
	Workspace        workspace.Kind
	PrimaryWorkspace workspace.Kind
	DividerDragging  bool
	Controls         workspace.Controls
	Changes          ChangeSummary

	NavigatorTitle string
	NavigatorRows  []NavigatorRow
	NavigatorEmpty Line
	Selected       int
	Top            int
	Focus          navigation.Focus

	ReaderTitle      string
	ReaderLines      []Line
	ReaderCommitRows []CommitRow
	ReaderEmpty      Line
	ReaderOffset     int
}
