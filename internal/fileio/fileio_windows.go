//go:build windows

package fileio

import (
	"os"

	"golang.org/x/sys/windows"
)

// Windows implementation of fileio.platform_syscalls. Not visible to drift's
// scanner on non-Windows hosts (build-tagged); the Unix counterpart in
// fileio_unix.go carries the marker for this spec on Linux/macOS.

// lockFileExclusive acquires an exclusive advisory lock on the file using
// LockFileEx (Windows). Blocks until acquired. The lock is released when the
// file handle is closed.
func lockFileExclusive(f *os.File) error {
	const lockFileExclusiveLock = 0x00000002
	var ol windows.Overlapped
	return windows.LockFileEx(
		windows.Handle(f.Fd()),
		lockFileExclusiveLock,
		0,
		1, // lock 1 byte
		0,
		&ol,
	)
}

// unlockFile releases the advisory lock and closes the file handle.
func unlockFile(f *os.File) error {
	var ol windows.Overlapped
	if err := windows.UnlockFileEx(
		windows.Handle(f.Fd()),
		0,
		1,
		0,
		&ol,
	); err != nil {
		return err
	}
	return f.Close()
}
