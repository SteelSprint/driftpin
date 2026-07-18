package testutil

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"drift/core"
	"drift/statestore"
)

func NewSpec(id string, hash string) core.Spec {
	return core.Spec{ID: id, Hash: hash, Filepath: id + ".xml", LineNumber: 10}
}

func NewSpecWithLocation(id string, hash string, filepath string, lineNumber int) core.Spec {
	return core.Spec{ID: id, Hash: hash, Filepath: filepath, LineNumber: lineNumber}
}

func NewMarker(id string, hash string) core.Marker {
	return core.Marker{ID: id, Hash: hash, Filepath: id + ".go", LineNumber: 20, EndLineNumber: 30}
}

func NewMarkerWithLocation(id string, hash string, filepath string, lineNumber int) core.Marker {
	return core.Marker{ID: id, Hash: hash, Filepath: filepath, LineNumber: lineNumber, EndLineNumber: lineNumber + 10}
}

// NewLink constructs a link-style Edge: marker stores edge to spec.
// Argument order preserved from the pre-collapse API for test readability.
func NewLink(specID string, markerID string) core.Edge {
	return core.Edge{From: markerID, To: specID}
}

// NewRef constructs a spec-spec Edge: fromSpec stores edge to toSpec.
func NewRef(fromSpec, toSpec string) core.Edge {
	return core.Edge{From: fromSpec, To: toSpec}
}

// NewResolutionState constructs an EdgeResolution covering a link-style edge.
// Argument order preserved from the pre-collapse API.
func NewResolutionState(specID string, markerID string, currentSpecHash string, currentMarkerHash string) core.EdgeResolution {
	return core.EdgeResolution{
		From:            markerID,
		To:              specID,
		CurrentFromHash: currentMarkerHash,
		CurrentToHash:   currentSpecHash,
	}
}

func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func AssertErrorWraps(t *testing.T, err error, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("error %v does not wrap %v", err, target)
	}
}

func AssertTodoCount(t *testing.T, state core.EvaluatedState, want int) {
	t.Helper()
	if len(state.Todos) != want {
		t.Fatalf("todo count = %d, want %d", len(state.Todos), want)
	}
}

func AssertResolutionStateCount(t *testing.T, state core.EvaluatedState, want int) {
	t.Helper()
	if len(state.Resolutions) != want {
		t.Fatalf("resolution state count = %d, want %d", len(state.Resolutions), want)
	}
}

func AssertResolutionStateEntry(t *testing.T, state core.EvaluatedState, markerID string, specID string, currentSpecHash string, currentMarkerHash string) {
	t.Helper()
	for _, res := range state.Resolutions {
		if res.From == markerID && res.To == specID {
			if res.CurrentToHash != currentSpecHash {
				t.Fatalf("resolution spec hash = %q, want %q", res.CurrentToHash, currentSpecHash)
			}
			if res.CurrentFromHash != currentMarkerHash {
				t.Fatalf("resolution marker hash = %q, want %q", res.CurrentFromHash, currentMarkerHash)
			}
			return
		}
	}
	t.Fatalf("resolution entry for marker=%q spec=%q not found", markerID, specID)
}

func AssertBaselineHashes(t *testing.T, state core.EvaluatedState, specID string, wantSpecHash string, markerID string, wantMarkerHash string) {
	t.Helper()
	if specID != "" {
		found := false
		for _, spec := range state.Specs {
			if spec.ID == specID {
				if spec.Hash != wantSpecHash {
					t.Fatalf("spec %q baseline hash = %q, want %q", specID, spec.Hash, wantSpecHash)
				}
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("spec %q not found in evaluated state", specID)
		}
	}
	if markerID != "" {
		found := false
		for _, marker := range state.Markers {
			if marker.ID == markerID {
				if marker.Hash != wantMarkerHash {
					t.Fatalf("marker %q baseline hash = %q, want %q", markerID, marker.Hash, wantMarkerHash)
				}
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("marker %q not found in evaluated state", markerID)
		}
	}
}

func AssertTodoDriftFlags(t *testing.T, todo core.Todo, wantSpecChanged bool, wantMarkerChanged bool) {
	t.Helper()
	// In the post-collapse model, the marker is the From endpoint and the
	// spec is the To endpoint of a link-style edge. Translate accordingly.
	if todo.ToChanged != wantSpecChanged {
		t.Fatalf("todo spec changed = %v, want %v", todo.ToChanged, wantSpecChanged)
	}
	if todo.FromChanged != wantMarkerChanged {
		t.Fatalf("todo marker changed = %v, want %v", todo.FromChanged, wantMarkerChanged)
	}
}

func AssertStateEquals(t *testing.T, got, want statestore.State) {
	t.Helper()
	if len(got.Specs) != len(want.Specs) {
		t.Fatalf("specs length = %d, want %d (got=%v want=%v)", len(got.Specs), len(want.Specs), got.Specs, want.Specs)
	}
	for i := range got.Specs {
		if got.Specs[i] != want.Specs[i] {
			t.Fatalf("spec[%d] = %+v, want %+v", i, got.Specs[i], want.Specs[i])
		}
	}
	if len(got.Markers) != len(want.Markers) {
		t.Fatalf("markers length = %d, want %d (got=%v want=%v)", len(got.Markers), len(want.Markers), got.Markers, want.Markers)
	}
	for i := range got.Markers {
		if got.Markers[i] != want.Markers[i] {
			t.Fatalf("marker[%d] = %+v, want %+v", i, got.Markers[i], want.Markers[i])
		}
	}
	if len(got.Edges) != len(want.Edges) {
		t.Fatalf("edges length = %d, want %d (got=%v want=%v)", len(got.Edges), len(want.Edges), got.Edges, want.Edges)
	}
	for i := range got.Edges {
		if got.Edges[i] != want.Edges[i] {
			t.Fatalf("edge[%d] = %+v, want %+v", i, got.Edges[i], want.Edges[i])
		}
	}
	if len(got.Resolutions) != len(want.Resolutions) {
		t.Fatalf("resolutions length = %d, want %d (got=%v want=%v)", len(got.Resolutions), len(want.Resolutions), got.Resolutions, want.Resolutions)
	}
	for i := range got.Resolutions {
		if got.Resolutions[i] != want.Resolutions[i] {
			t.Fatalf("resolution[%d] = %+v, want %+v", i, got.Resolutions[i], want.Resolutions[i])
		}
	}
}

func ExpectedSha1Hex(content string) string {
	h := sha1.Sum([]byte(content))
	return fmt.Sprintf("%x", h)
}

func FindScanResultSpec(results []core.Spec, id string) (core.Spec, bool) {
	for _, s := range results {
		if s.ID == id {
			return s, true
		}
	}
	return core.Spec{}, false
}

func FindScanResultMarker(results []core.Marker, id string) (core.Marker, bool) {
	for _, m := range results {
		if m.ID == id {
			return m, true
		}
	}
	return core.Marker{}, false
}

func WriteSpecFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write spec file %s: %v", name, err)
	}
}

func WriteCodeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write code file %s: %v", name, err)
	}
}

func WriteIgnoreFile(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, "drift.ignore")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write drift.ignore: %v", err)
	}
}

func FindSpecInEvaluatedState(t *testing.T, state core.EvaluatedState, id string) core.Spec {
	t.Helper()
	for _, s := range state.Specs {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("spec %q not found in evaluated state", id)
	return core.Spec{}
}

func FindMarkerInEvaluatedState(t *testing.T, state core.EvaluatedState, id string) core.Marker {
	t.Helper()
	for _, m := range state.Markers {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("marker %q not found in evaluated state", id)
	return core.Marker{}
}

func EvaluatedToState(state core.EvaluatedState) statestore.State {
	return statestore.State{
		Specs:       state.Specs,
		Markers:     state.Markers,
		Edges:       state.Edges,
		Resolutions: state.Resolutions,
	}
}
