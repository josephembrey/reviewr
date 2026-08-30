//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package review

import (
	"os"
	"syscall"
)

func lockExclusive(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
}

func unlock(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
