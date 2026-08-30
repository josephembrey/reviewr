package git

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ObjectKind classifies a bounded immutable Git object read.
type ObjectKind uint8

const (
	ObjectReady ObjectKind = iota + 1
	ObjectMissing
	ObjectBinary
	ObjectTooLarge
	ObjectUnreadable
)

// ObjectFile is a bounded blob or patch produced from immutable objects.
type ObjectFile struct {
	Kind ObjectKind
	Data []byte
	Size int64
	Err  error
}

// EmptyTree returns the empty tree identity for this repository's object format.
func (client Client) EmptyTree(root string) (string, error) {
	oid, err := client.EmptyTreeOID(root)
	if err != nil {
		return "", err
	}
	if !validObjectID(oid) {
		return "", fmt.Errorf("git returned an invalid empty tree identity")
	}
	return oid, nil
}

// ReadObjectFile reads one exact path from an exact tree-ish without consulting the worktree.
func (Client) ReadObjectFile(root, oid, path string, maxBytes int64) ObjectFile {
	if !validObjectID(oid) || !validGitPath(path) {
		return ObjectFile{Kind: ObjectUnreadable, Err: fmt.Errorf("invalid immutable file identity")}
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxHistoryBytes
	}
	expression := oid + ":" + path
	sizeOutput, err := runBounded(root, 128, "cat-file", "-s", expression)
	if err != nil {
		return ObjectFile{Kind: ObjectMissing, Err: err}
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(sizeOutput)), 10, 64)
	if err != nil || size < 0 {
		return ObjectFile{Kind: ObjectUnreadable, Err: fmt.Errorf("parse Git object size for %q", path)}
	}
	if size > maxBytes {
		return ObjectFile{Kind: ObjectTooLarge, Size: size}
	}
	data, err := runBounded(root, maxBytes, "cat-file", "blob", expression)
	if err != nil {
		kind := ObjectUnreadable
		if errors.Is(err, ErrOutputTooLarge) {
			kind = ObjectTooLarge
		}
		return ObjectFile{Kind: kind, Size: size, Err: err}
	}
	kind := ObjectReady
	if bytes.IndexByte(data, 0) >= 0 {
		kind = ObjectBinary
	}
	return ObjectFile{Kind: kind, Data: data, Size: size}
}

// DiffObjects returns a bounded patch between exact object IDs for the selected paths.
func (Client) DiffObjects(root, oldOID, newOID string, paths []string, maxBytes int64) ObjectFile {
	if !validObjectID(oldOID) || !validObjectID(newOID) {
		return ObjectFile{Kind: ObjectUnreadable, Err: fmt.Errorf("invalid comparison object identity")}
	}
	if len(paths) == 0 {
		return ObjectFile{Kind: ObjectUnreadable, Err: fmt.Errorf("comparison has no path")}
	}
	for _, path := range paths {
		if !validGitPath(path) {
			return ObjectFile{Kind: ObjectUnreadable, Err: fmt.Errorf("invalid comparison path")}
		}
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxHistoryBytes
	}
	args := []string{
		"diff",
		"-M",
		"-C",
		"--no-ext-diff",
		"--no-textconv",
		"--no-color",
		expandableDiffContext,
		oldOID,
		newOID,
		"--",
	}
	args = append(args, paths...)
	data, err := runBounded(root, maxBytes, args...)
	if err != nil {
		kind := ObjectUnreadable
		if errors.Is(err, ErrOutputTooLarge) {
			kind = ObjectTooLarge
		}
		return ObjectFile{Kind: kind, Size: maxBytes + 1, Err: err}
	}
	return ObjectFile{Kind: ObjectReady, Data: data, Size: int64(len(data))}
}
