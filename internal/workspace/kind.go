// Package workspace names reviewr's primary workspaces without owning their state.
package workspace

// Kind identifies the body document selected by the primary header control.
type Kind uint8

const (
	Files Kind = iota
	Git
	Scratch
)

// Toggle returns the other primary workspace.
func (kind Kind) Toggle() Kind {
	if kind == Git {
		return Files
	}
	return Git
}
