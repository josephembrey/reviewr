package git

import (
	"bytes"
)

// ReadDiff returns a bounded no-color patch for one literal repository entry.
// The old path participates when Git reported a rename. Untracked files are
// compared to /dev/null and Git's expected exit status 1 is accepted.
func (Client) ReadDiff(root, path, previousPath string, untracked bool, maxBytes int64) ([]byte, error) {
	if untracked {
		return runBoundedAllowExit(
			root,
			maxBytes,
			map[int]struct{}{1: {}},
			"diff",
			"--no-index",
			"--no-color",
			"--no-ext-diff",
			"--",
			"/dev/null",
			path,
		)
	}
	base, err := run(root, "rev-parse", "--verify", "-q", "HEAD")
	if err != nil {
		base, err = hashEmptyTree(root)
		if err != nil {
			return nil, err
		}
	}
	args := []string{
		"diff",
		"--no-color",
		"--no-ext-diff",
		"--find-renames",
		string(bytes.TrimSpace(base)),
		"--",
	}
	if previousPath != "" && previousPath != path {
		args = append(args, previousPath)
	}
	args = append(args, path)
	return runBounded(root, maxBytes, args...)
}
