package pinstore_test

import (
	"os"
	"path/filepath"
	"testing"

	"driftpin/internal/testutil"
	"driftpin/pinstore"
)

func TestBaselineStoreWriteReadRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".driftpin", "baselines")
	store := pinstore.NewBaselineStore(dir)

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

	// File should live at .driftpin/baselines/<hash>.
	if _, err := os.Stat(filepath.Join(dir, hash)); os.IsNotExist(err) {
		t.Fatalf("baseline file not created at expected path")
	}
}

func TestBaselineStoreWriteDedup(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".driftpin", "baselines")
	store := pinstore.NewBaselineStore(dir)

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
	dir := filepath.Join(t.TempDir(), ".driftpin", "baselines")
	store := pinstore.NewBaselineStore(dir)

	if _, err := os.Stat(dir); err == nil {
		t.Fatalf("baselines dir should not exist yet")
	}
	if _, ok := store.Read("nonexistenthash"); ok {
		t.Fatalf("Read returned ok=true for missing baseline")
	}
}

func TestBaselineStoreWriteHashMismatch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".driftpin", "baselines")
	store := pinstore.NewBaselineStore(dir)

	content := "actual\n"
	declaredHash := "deadbeef" // wrong on purpose
	err := store.Write(declaredHash, content)
	if err == nil {
		t.Fatalf("expected error on hash mismatch")
	}
	// File must not be written for a mismatched hash.
	if _, statErr := os.Stat(filepath.Join(dir, declaredHash)); statErr == nil {
		t.Fatalf("baseline file should not exist after hash mismatch")
	}
}

func TestBaselineStoreDelete(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".driftpin", "baselines")
	store := pinstore.NewBaselineStore(dir)

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
	dir := filepath.Join(t.TempDir(), ".driftpin", "baselines")
	store := pinstore.NewBaselineStore(dir)
	if err := store.Delete("neverExisted"); err != nil {
		t.Fatalf("Delete missing file should not error: %v", err)
	}
}
