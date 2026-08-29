// Package navigation owns the Go TUI's user-controlled place state.
package navigation

// Focus identifies the pane receiving keyboard navigation intent.
type Focus uint8

const (
	FocusNavigator Focus = iota
	FocusReader
)

// State holds stable item selection and both pane offsets.
type State struct {
	Items        []string
	Selected     int
	Top          int
	Focus        Focus
	ReaderOffset int
}

// SelectedIdentity returns the selected item's stable identity.
func (s State) SelectedIdentity() (string, bool) {
	if s.Selected < 0 || s.Selected >= len(s.Items) {
		return "", false
	}
	return s.Items[s.Selected], true
}

// Reconcile replaces the loaded identities while preserving selection and top-row
// identities, then falling back to the nearest old survivor and finally clamp.
func (s *State) Reconcile(items []string) {
	oldItems := s.Items
	oldSelected := s.Selected
	oldTop := s.Top
	s.Items = append([]string(nil), items...)
	if len(items) == 0 {
		s.Selected = 0
		s.Top = 0
		return
	}

	s.Selected = reconcileIndex(oldItems, oldSelected, items)
	s.Top = reconcileIndex(oldItems, oldTop, items)
	if s.Top > s.Selected {
		s.Top = s.Selected
	}
}

// SelectDelta applies user selection intent and resets reader scroll only when
// the selected identity actually changes.
func (s *State) SelectDelta(delta int, visibleRows int) bool {
	return s.SelectIndex(s.Selected+delta, visibleRows)
}

// SelectIndex applies a user-selected row, clamped to the current item list.
func (s *State) SelectIndex(index int, visibleRows int) bool {
	if len(s.Items) == 0 {
		return false
	}
	index = clamp(index, 0, len(s.Items)-1)
	if index == s.Selected {
		return false
	}
	s.Selected = index
	s.ReaderOffset = 0
	s.EnsureSelectionVisible(visibleRows)
	return true
}

// ToggleFocus moves keyboard navigation intent to the other pane.
func (s *State) ToggleFocus() {
	if s.Focus == FocusNavigator {
		s.Focus = FocusReader
	} else {
		s.Focus = FocusNavigator
	}
}

// EnsureSelectionVisible reconciles the Navigator offset to its viewport.
func (s *State) EnsureSelectionVisible(visibleRows int) {
	if len(s.Items) == 0 {
		s.Top = 0
		return
	}
	if visibleRows <= 0 {
		s.Top = clamp(s.Top, 0, len(s.Items)-1)
		return
	}
	maxTop := max(0, len(s.Items)-visibleRows)
	s.Top = clamp(s.Top, 0, maxTop)
	if s.Selected < s.Top {
		s.Top = s.Selected
	}
	if s.Selected >= s.Top+visibleRows {
		s.Top = s.Selected - visibleRows + 1
	}
}

// ScrollReader applies user scroll intent within the currently loaded content.
func (s *State) ScrollReader(delta int, lineCount int, visibleRows int) {
	maxOffset := max(0, lineCount-max(0, visibleRows))
	s.ReaderOffset = clamp(s.ReaderOffset+delta, 0, maxOffset)
}

// ClampReader reconciles scroll after content size or viewport changes.
func (s *State) ClampReader(lineCount int, visibleRows int) {
	maxOffset := max(0, lineCount-max(0, visibleRows))
	s.ReaderOffset = clamp(s.ReaderOffset, 0, maxOffset)
}

func reconcileIndex(old []string, oldIndex int, current []string) int {
	if len(current) == 0 {
		return 0
	}
	if len(old) == 0 {
		return clamp(oldIndex, 0, len(current)-1)
	}
	oldIndex = clamp(oldIndex, 0, len(old)-1)
	currentIndices := make(map[string]int, len(current))
	for index, path := range current {
		currentIndices[path] = index
	}
	if index, ok := currentIndices[old[oldIndex]]; ok {
		return index
	}
	for distance := 1; distance < len(old); distance++ {
		if next := oldIndex + distance; next < len(old) {
			if index, ok := currentIndices[old[next]]; ok {
				return index
			}
		}
		if previous := oldIndex - distance; previous >= 0 {
			if index, ok := currentIndices[old[previous]]; ok {
				return index
			}
		}
	}
	return clamp(oldIndex, 0, len(current)-1)
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
