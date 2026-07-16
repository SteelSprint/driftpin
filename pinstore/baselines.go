package pinstore

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// D! id=pbase range-start
var ErrBaselineHashMismatch = errors.New("baseline content hash does not match declared hash")

type BaselineStore struct {
	dir string
}

func NewBaselineStore(dir string) *BaselineStore {
	return &BaselineStore{dir: dir}
}

// Write stores content at .driftpin/baselines/<hash> if absent.
// Verifies sha1(content) == hash defensively; mismatches produce ErrBaselineHashMismatch.
// If a file already exists for hash, Write is a no-op (dedup).
func (b *BaselineStore) Write(hash, content string) error {
	if actual := sha1Hex(content); actual != hash {
		return fmt.Errorf("%w: declared %q, actual %q", ErrBaselineHashMismatch, hash, actual)
	}
	if err := os.MkdirAll(b.dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(b.dir, hash)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// Read returns the baseline content for hash. Returns false when the file
// is missing (e.g. pre-migration snapshots, or content-addressed lookup miss).
func (b *BaselineStore) Read(hash string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(b.dir, hash))
	if err != nil {
		return "", false
	}
	return string(data), true
}

// Delete removes a baseline file. Missing files are not an error.
func (b *BaselineStore) Delete(hash string) error {
	err := os.Remove(filepath.Join(b.dir, hash))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func sha1Hex(content string) string {
	h := sha1.Sum([]byte(content))
	return fmt.Sprintf("%x", h)
}

// D! id=pbase range-end
