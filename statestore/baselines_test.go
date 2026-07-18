package statestore_test

import (
	"os"
	"path/filepath"
	"testing"

	"drift/internal/testutil"
	"drift/statestore"
)

func TestBaselineStoreWriteReadRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".drift", "baselines")
	store := statestore.NewBaselineStore(dir)

	content := "line1\nline2\nline3\n"
	hash := testutil.ExpectedSha1Hex(content)

	if err := store.Write(hash, content); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, ok := store.Read(hash)
	if !ok {
		t.Fatalf("Read returned ok=false for existing baseline")
	}
	if got != content {
		t.Fatalf("Read content mismatch: got %q, want %q", got, content)
	}

	// File should live at .drift/baselines/<hash>.
	if _, err := os.Stat(filepath.Join(dir, hash)); os.IsNotExist(err) {
		t.Fatalf("baseline file not created at expected path")
	}
}

func TestBaselineStoreWriteDedup(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".drift", "baselines")
	store := statestore.NewBaselineStore(dir)

	content := "same\n"
	hash := testutil.ExpectedSha1Hex(content)

	if err := store.Write(hash, content); err != nil {
		t.Fatal(err)
	}
	// Second Write with same hash must not error and must not corrupt content.
	if err := store.Write(hash, content); err != nil {
		t.Fatalf("second Write failed: %v", err)
	}

	got, ok := store.Read(hash)
	if !ok || got != content {
		t.Fatalf("dedup Read mismatch: ok=%v got=%q", ok, got)
	}
}

func TestBaselineStoreReadMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".drift", "baselines")
	store := statestore.NewBaselineStore(dir)

	if _, err := os.Stat(dir); err == nil {
		t.Fatalf("baselines dir should not exist yet")
	}
	if _, ok := store.Read("nonexistenthash"); ok {
		t.Fatalf("Read returned ok=true for missing baseline")
	}
}

func TestBaselineStoreWriteToleratesHashMismatch(t *testing.T) {
	// Since refs were introduced, the canonical hash (refs stripped) does not
	// equal sha1(raw content). Write must tolerate the mismatch so that diff
	// display can use raw spec text while drift detection uses canonical hash.
	dir := filepath.Join(t.TempDir(), ".drift", "baselines")
	store := statestore.NewBaselineStore(dir)

	content := "actual\n"
	declaredHash := "deadbeef" // does not match sha1(content)
	if err := store.Write(declaredHash, content); err != nil {
		t.Fatalf("Write returned error for hash mismatch: %v", err)
	}
	got, ok := store.Read(declaredHash)
	if !ok {
		t.Fatalf("baseline file should exist after Write")
	}
	if got != content {
		t.Fatalf("Read returned %q, want %q", got, content)
	}
}

func TestBaselineStoreDelete(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".drift", "baselines")
	store := statestore.NewBaselineStore(dir)

	content := "toDelete\n"
	hash := testutil.ExpectedSha1Hex(content)
	if err := store.Write(hash, content); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(hash); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, ok := store.Read(hash); ok {
		t.Fatalf("Read returned ok=true after Delete")
	}
}

func TestBaselineStoreDeleteMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".drift", "baselines")
	store := statestore.NewBaselineStore(dir)
	if err := store.Delete("neverExisted"); err != nil {
		t.Fatalf("Delete missing file should not error: %v", err)
	}
}
