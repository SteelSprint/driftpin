//go:build !windows

package statestore

import (
	"os"

	"golang.org/x/sys/unix"
)

// lockFile acquires an exclusive advisory lock on the file using flock (Unix).
// This blocks until the lock is acquired. The lock is released when the file
// descriptor is closed (via unlockFile or process exit).
func lockFile(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_EX)
}

// unlockFile releases the advisory lock and closes the file descriptor.
func unlockFile(f *os.File) error {
	if err := unix.Flock(int(f.Fd()), unix.LOCK_UN); err != nil {
		return err
	}
	return f.Close()
}
