package testutil

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"driftpin/core"
	"driftpin/pinstore"
)

func NewSpec(id string, hash string) core.Spec {
	return core.Spec{ID: id, Hash: hash, Filepath: id + ".xml", LineNumber: 10}
}

func NewSpecWithLocation(id string, hash string, filepath string, lineNumber int) core.Spec {
	return core.Spec{ID: id, Hash: hash, Filepath: filepath, LineNumber: lineNumber}
}

func NewMarker(id string, hash string) core.Marker {
	return core.Marker{ID: id, Hash: hash, Filepath: id + ".go", LineNumber: 20}
}

func NewMarkerWithLocation(id string, hash string, filepath string, lineNumber int) core.Marker {
	return core.Marker{ID: id, Hash: hash, Filepath: filepath, LineNumber: lineNumber}
}

func NewLink(specID string, markerID string) core.Link {
	return core.Link{SpecID: specID, MarkerID: markerID}
}

func NewResolutionState(specID string, markerID string, currentSpecHash string, currentMarkerHash string) core.ResolutionState {
	return core.ResolutionState{
		SpecID:            specID,
		MarkerID:          markerID,
		CurrentSpecHash:   currentSpecHash,
		CurrentMarkerHash: currentMarkerHash,
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
	if len(state.ResolutionState) != want {
		t.Fatalf("resolution state count = %d, want %d", len(state.ResolutionState), want)
	}
}

func AssertResolutionStateEntry(t *testing.T, state core.EvaluatedState, markerID string, specID string, currentSpecHash string, currentMarkerHash string) {
	t.Helper()
	for _, res := range state.ResolutionState {
		if res.MarkerID == markerID && res.SpecID == specID {
			if res.CurrentSpecHash != currentSpecHash {
				t.Fatalf("resolution spec hash = %q, want %q", res.CurrentSpecHash, currentSpecHash)
			}
			if res.CurrentMarkerHash != currentMarkerHash {
				t.Fatalf("resolution marker hash = %q, want %q", res.CurrentMarkerHash, currentMarkerHash)
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
	if todo.SpecChanged != wantSpecChanged {
		t.Fatalf("todo spec changed = %v, want %v", todo.SpecChanged, wantSpecChanged)
	}
	if todo.MarkerChanged != wantMarkerChanged {
		t.Fatalf("todo marker changed = %v, want %v", todo.MarkerChanged, wantMarkerChanged)
	}
}

func AssertPinStateEquals(t *testing.T, got, want pinstore.PinState) {
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
	if len(got.Links) != len(want.Links) {
		t.Fatalf("links length = %d, want %d (got=%v want=%v)", len(got.Links), len(want.Links), got.Links, want.Links)
	}
	for i := range got.Links {
		if got.Links[i] != want.Links[i] {
			t.Fatalf("link[%d] = %+v, want %+v", i, got.Links[i], want.Links[i])
		}
	}
	if len(got.ResolutionState) != len(want.ResolutionState) {
		t.Fatalf("resolutions length = %d, want %d (got=%v want=%v)", len(got.ResolutionState), len(want.ResolutionState), got.ResolutionState, want.ResolutionState)
	}
	for i := range got.ResolutionState {
		if got.ResolutionState[i] != want.ResolutionState[i] {
			t.Fatalf("resolution[%d] = %+v, want %+v", i, got.ResolutionState[i], want.ResolutionState[i])
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

func EvaluatedStateToPinState(state core.EvaluatedState) pinstore.PinState {
	return pinstore.PinState{
		Specs:           state.Specs,
		Markers:         state.Markers,
		Links:           state.Links,
		ResolutionState: state.ResolutionState,
	}
}
