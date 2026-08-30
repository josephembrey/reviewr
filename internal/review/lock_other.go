//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package review

import "os"

// Unsupported application targets retain atomic replacement but cannot offer
// cross-process merge serialization.
func lockExclusive(*os.File) error { return nil }
func unlock(*os.File)              {}
