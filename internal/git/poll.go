package git

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"path/filepath"
)

// StateFingerprint separates worktree/index changes from public history
// changes so callers only rebuild the affected application domains.
type StateFingerprint struct {
	Worktree string
	Refs     string
}

// PollState returns a cheap, read-only fingerprint of observable Git state.
// Private reviewr refs are intentionally excluded so our own bookkeeping can
// never create a refresh loop.
func (Client) PollState(root string) (StateFingerprint, error) {
	status, err := run(
		root,
		"status",
		"--porcelain=v2",
		"-z",
		"--untracked-files=all",
		"--ignored=no",
		"--renames",
	)
	if err != nil {
		return StateFingerprint{}, err
	}
	entries, err := ParsePorcelainV2(status)
	if err != nil {
		return StateFingerprint{}, err
	}
	refs, err := run(
		root,
		"for-each-ref",
		"--format=%(refname)%00%(objectname)",
		"refs/heads",
		"refs/remotes",
		"refs/tags",
		"refs/stash",
	)
	if err != nil {
		return StateFingerprint{}, err
	}
	head, _ := run(root, "rev-parse", "--verify", "HEAD")

	worktreeHash := sha256.New()
	writeFingerprintPart(worktreeHash, status)
	writeFingerprintPart(worktreeHash, head)
	for _, entry := range entries {
		writePathMetadata(worktreeHash, root, entry.Path)
	}
	refsHash := sha256.New()
	writeFingerprintPart(refsHash, refs)
	writeFingerprintPart(refsHash, head)
	return StateFingerprint{
		Worktree: hex.EncodeToString(worktreeHash.Sum(nil)),
		Refs:     hex.EncodeToString(refsHash.Sum(nil)),
	}, nil
}

func writeFingerprintPart(destination hash.Hash, value []byte) {
	_, _ = fmt.Fprintf(destination, "%d:", len(value))
	_, _ = destination.Write(value)
}

func writePathMetadata(destination hash.Hash, root, path string) {
	info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		_, _ = fmt.Fprintf(destination, "%q:error:%v\n", path, err)
		return
	}
	_, _ = fmt.Fprintf(
		destination,
		"%q:%d:%d:%d\n",
		path,
		info.Size(),
		info.Mode(),
		info.ModTime().UnixNano(),
	)
}
