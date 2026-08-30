// Package workspace names reviewr's top-level destinations without owning their state.
package workspace

// Kind identifies the active top-level destination.
type Kind uint8

const (
	Files Kind = iota
	Git
	Notes
)
