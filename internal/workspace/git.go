package workspace

// GitFocus identifies one visible region in the Git workspace. Modes cycle
// only through the subset they render.
type GitFocus uint8

const (
	GitSource GitFocus = iota + 1
	GitTimeline
	GitStash
	GitFiles
	GitDiff
)

func (focus GitFocus) Label() string {
	switch focus {
	case GitTimeline:
		return "timeline"
	case GitStash:
		return "stashes"
	case GitFiles:
		return "files"
	case GitDiff:
		return "diff"
	default:
		return "source"
	}
}
