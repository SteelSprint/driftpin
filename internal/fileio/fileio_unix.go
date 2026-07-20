//go:build !windows

package fileio

import (
	"os"

	"golang.org/x/sys/unix"
)

// D! id=fsysu range-start

// lockFileExclusive acquires an exclusive advisory lock on the file using
// flock (Unix). Blocks until acquired. The lock is released when the file
// descriptor is closed.
func lockFileExclusive(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_EX)
}

// unlockFile releases the advisory lock and closes the file descriptor.
func unlockFile(f *os.File) error {
	if err := unix.Flock(int(f.Fd()), unix.LOCK_UN); err != nil {
		return err
	}
	return f.Close()
}

// D! id=fsysu range-end
