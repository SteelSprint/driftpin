package statestore

import (
	"fmt"
	"os"
	"path/filepath"
)

// Lock acquires an exclusive advisory lock on the state file. It blocks until
// the lock is acquired. The returned cleanup function releases the lock and
// closes the lock file handle; it must be called exactly once (typically via
// defer). The lock is auto-released if the process exits or crashes.
//
// All state-mutating orchestrator methods (Link, Unlink, Reset, ResetOrphan)
// must call Lock before Load→modify→Save to prevent concurrent writers from
// silently overwriting each other's changes.
func (s *FileStateStore) Lock() (func(), error) {
	lockPath := filepath.Join(s.dir, ".drift", "state.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", lockPath, err)
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("acquire lock on %s: %w", lockPath, err)
	}
	return func() {
		unlockFile(f)
		f.Close()
	}, nil
}
