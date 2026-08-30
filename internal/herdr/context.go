// Package herdr owns optional integration with a Herdr-hosted runtime.
//
// Host detection is a pure startup operation. Effects that use the captured
// context belong to Runtime so feature packages never read Herdr environment
// variables or launch Herdr commands independently.
package herdr

import "path/filepath"

// LookupEnv is the environment lookup shape consumed by Detect.
type LookupEnv func(string) (string, bool)

// Context is the immutable snapshot of Herdr's startup environment.
// Optional values are empty when Herdr did not provide them.
type Context struct {
	hosted      bool
	workspaceID string
	tabID       string
	paneID      string
	socketPath  string
	binPath     string
}

// Detect captures Herdr's host context without subprocesses or side effects.
// HERDR_ENV=1 is the only authoritative hosted marker.
func Detect(lookup LookupEnv) Context {
	marker, ok := lookup("HERDR_ENV")
	if !ok || marker != "1" {
		return Context{}
	}
	return Context{
		hosted:      true,
		workspaceID: value(lookup, "HERDR_WORKSPACE_ID"),
		tabID:       value(lookup, "HERDR_TAB_ID"),
		paneID:      value(lookup, "HERDR_PANE_ID"),
		socketPath:  value(lookup, "HERDR_SOCKET_PATH"),
		binPath:     value(lookup, "HERDR_BIN_PATH"),
	}
}

func value(lookup LookupEnv, name string) string {
	value, _ := lookup(name)
	return value
}

// Hosted reports whether HERDR_ENV=1 was present at startup.
func (c Context) Hosted() bool { return c.hosted }

// WorkspaceID is the optional Herdr workspace identity.
func (c Context) WorkspaceID() string { return c.workspaceID }

// TabID is the optional Herdr tab identity.
func (c Context) TabID() string { return c.tabID }

// PaneID is the optional current-pane identity.
func (c Context) PaneID() string { return c.paneID }

// SocketPath is the optional path to the hosting Herdr socket.
func (c Context) SocketPath() string { return c.socketPath }

// BinPath is the optional absolute path to the hosting Herdr executable.
func (c Context) BinPath() string { return c.binPath }

func (c Context) canLabelPane() bool {
	return c.hosted && c.paneID != "" && filepath.IsAbs(c.binPath)
}
