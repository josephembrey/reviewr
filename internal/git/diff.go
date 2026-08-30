package git

import "errors"

// expandableDiffContext asks Git for the complete bounded file context. The
// reader folds unchanged runs after parsing, then can reveal those exact rows
// without another subprocess or a lossy synthetic reconstruction.
const expandableDiffContext = "--unified=1000000"

// ReadDiff returns a bounded no-color patch for one literal repository entry.
// The old path participates when Git reported a rename. Untracked files are
// compared to /dev/null and Git's expected exit status 1 is accepted.
func (client Client) ReadDiff(root, path, previousPath string, untracked bool, maxBytes int64) ([]byte, error) {
	if untracked {
		return runBoundedAllowExit(
			root,
			maxBytes,
			map[int]struct{}{1: {}},
			"diff",
			"--no-index",
			"--no-color",
			"--no-ext-diff",
			expandableDiffContext,
			"--",
			"/dev/null",
			path,
		)
	}
	base, err := client.HeadOID(root)
	if errors.Is(err, ErrUnbornHead) {
		base, err = client.EmptyTreeOID(root)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	args := []string{
		"diff",
		"--no-color",
		"--no-ext-diff",
		"--find-renames",
		expandableDiffContext,
		base,
		"--",
	}
	if previousPath != "" && previousPath != path {
		args = append(args, previousPath)
	}
	args = append(args, path)
	return runBounded(root, maxBytes, args...)
}
