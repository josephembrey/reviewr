package git

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
)

const (
	// CommitLimit bounds startup and refresh work for the initial Git workspace.
	CommitLimit = 200
	// DefaultMaxHistoryBytes bounds one history or summary read in memory.
	DefaultMaxHistoryBytes int64 = 1 << 20
)

// Commit is one current-HEAD history row.
type Commit struct {
	OID      string
	ShortOID string
	Subject  string
}

// CommitSummary is bounded metadata and stat for one exact commit identity.
type CommitSummary struct {
	OID         string
	AuthorName  string
	AuthorEmail string
	AuthoredAt  string
	Message     string
	Stat        string
}

// ListCommits returns at most CommitLimit commits reachable from HEAD, newest first.
func (Client) ListCommits(root string) ([]Commit, error) {
	hasHead, err := hasHead(root)
	if err != nil || !hasHead {
		return nil, err
	}
	out, err := runBounded(
		root,
		DefaultMaxHistoryBytes,
		"log",
		"-z",
		"--date-order",
		"--max-count="+strconv.Itoa(CommitLimit),
		"--format=%H%x00%h%x00%s%x00",
		"HEAD",
	)
	if err != nil {
		return nil, err
	}
	return parseCommitLog(out)
}

// ReadCommit returns bounded metadata and a first-parent changed-file stat.
func (Client) ReadCommit(root, oid string, maxBytes int64) (CommitSummary, error) {
	if !validObjectID(oid) {
		return CommitSummary{}, fmt.Errorf("invalid commit object ID %q", oid)
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxHistoryBytes
	}
	metadata, err := runBounded(
		root,
		maxBytes,
		"show",
		"-s",
		"--no-color",
		"--format=%H%x00%an%x00%ae%x00%aI%x00%B",
		"--end-of-options",
		oid,
	)
	if err != nil {
		return CommitSummary{}, err
	}
	remaining := maxBytes - int64(len(metadata))
	if remaining <= 0 {
		return CommitSummary{}, fmt.Errorf("git show: %w (%d bytes)", ErrOutputTooLarge, maxBytes)
	}
	stat, err := runBounded(
		root,
		remaining,
		"show",
		"--first-parent",
		"--format=",
		"--stat",
		"--no-renames",
		"--no-ext-diff",
		"--no-textconv",
		"--no-color",
		"--end-of-options",
		oid,
		"--",
	)
	if err != nil {
		return CommitSummary{}, err
	}
	return parseCommitSummary(metadata, stat)
}

func hasHead(root string) (bool, error) {
	_, err := runBounded(root, 128, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err == nil {
		return true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func parseCommitLog(data []byte) ([]Commit, error) {
	records := bytes.Split(data, []byte{0, 0})
	commits := make([]Commit, 0, min(len(records), CommitLimit))
	for _, record := range records {
		if len(record) == 0 {
			continue
		}
		fields := bytes.Split(record, []byte{0})
		if len(fields) != 3 {
			return nil, fmt.Errorf("parse git log: record has %d fields", len(fields))
		}
		if !validObjectID(string(fields[0])) || len(fields[1]) == 0 {
			return nil, fmt.Errorf("parse git log: invalid object identity")
		}
		commits = append(commits, Commit{OID: string(fields[0]), ShortOID: string(fields[1]), Subject: string(fields[2])})
		if len(commits) == CommitLimit {
			break
		}
	}
	return commits, nil
}

func parseCommitSummary(metadata, stat []byte) (CommitSummary, error) {
	fields := bytes.SplitN(metadata, []byte{0}, 5)
	if len(fields) != 5 {
		return CommitSummary{}, fmt.Errorf("parse git show: metadata has %d fields", len(fields))
	}
	return CommitSummary{
		OID:         string(fields[0]),
		AuthorName:  string(fields[1]),
		AuthorEmail: string(fields[2]),
		AuthoredAt:  string(fields[3]),
		Message:     string(bytes.TrimSuffix(fields[4], []byte{'\n'})),
		Stat:        string(bytes.TrimSuffix(stat, []byte{'\n'})),
	}, nil
}

func validObjectID(oid string) bool {
	if len(oid) != 40 && len(oid) != 64 {
		return false
	}
	_, err := hex.DecodeString(oid)
	return err == nil
}
