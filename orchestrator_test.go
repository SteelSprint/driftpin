package driftpin

import (
	"errors"
	"testing"
)

type fakePinStore struct {
	state   PinState
	loadErr error
	saveErr error
	saved   []PinState
}

func (f *fakePinStore) Load() (PinState, error) {
	if f.loadErr != nil {
		return PinState{}, f.loadErr
	}
	return f.state, nil
}

func (f *fakePinStore) Save(state PinState) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.state = state
	f.saved = append(f.saved, state)
	return nil
}

type fakeScanner struct {
	result ScanResult
	err    error
}

func (f *fakeScanner) Scan() (ScanResult, error) {
	if f.err != nil {
		return ScanResult{}, f.err
	}
	return f.result, nil
}

func scanResultFromSpecsMarkers(specs []Spec, markers []Marker) ScanResult {
	return ScanResult{
		Specs:   specs,
		Markers: markers,
	}
}

func scanResultWithOverrides(specs []Spec, markers []Marker, specOverrides, markerOverrides map[string]string) ScanResult {
	resultSpecs := make([]Spec, len(specs))
	for i, s := range specs {
		resultSpecs[i] = s
		if h, ok := specOverrides[s.ID]; ok {
			resultSpecs[i].Hash = h
		}
	}
	resultMarkers := make([]Marker, len(markers))
	for i, m := range markers {
		resultMarkers[i] = m
		if h, ok := markerOverrides[m.ID]; ok {
			resultMarkers[i].Hash = h
		}
	}
	return ScanResult{Specs: resultSpecs, Markers: resultMarkers}
}

func TestOrchestratorInit(t *testing.T) {
	t.Run("init_saves_empty_state", func(t *testing.T) {
		pin := &fakePinStore{}
		scanner := &fakeScanner{}
		orch := NewOrchestrator(pin, scanner)

		err := orch.Init()
		assertNoError(t, err)

		if len(pin.saved) != 1 {
			t.Fatalf("expected 1 save, got %d", len(pin.saved))
		}
		assertPinStateEquals(t, pin.saved[0], PinState{})
	})

	t.Run("init_propagates_save_error", func(t *testing.T) {
		pin := &fakePinStore{saveErr: errors.New("write failed")}
		scanner := &fakeScanner{}
		orch := NewOrchestrator(pin, scanner)

		err := orch.Init()
		if err == nil {
			t.Fatalf("expected error from save failure")
		}
	})
}

func TestOrchestratorTodoArity(t *testing.T) {
	shapes := []struct {
		name    string
		specs   []Spec
		markers []Marker
		links   []Link
	}{
		{"0_specs_0_markers", nil, nil, nil},
		{"1_spec_0_markers", []Spec{newSpec("s1", "b1")}, nil, nil},
		{"0_specs_1_marker", nil, []Marker{newMarker("m1", "b1")}, nil},
		{"1_spec_1_marker", []Spec{newSpec("s1", "b1")}, []Marker{newMarker("m1", "b1")}, []Link{newLink("s1", "m1")}},
		{"many_specs_1_marker", []Spec{newSpec("s1", "b1")}, []Marker{newMarker("m1", "b1"), newMarker("m2", "b2")}, []Link{newLink("s1", "m1"), newLink("s1", "m2")}},
		{"1_spec_many_markers", []Spec{newSpec("s1", "b1"), newSpec("s2", "b2")}, []Marker{newMarker("m1", "b1")}, []Link{newLink("s1", "m1"), newLink("s2", "m1")}},
		{"many_specs_many_markers", []Spec{newSpec("s1", "b1"), newSpec("s2", "b2")}, []Marker{newMarker("m1", "b1"), newMarker("m2", "b2")}, []Link{newLink("s1", "m1"), newLink("s1", "m2"), newLink("s2", "m1"), newLink("s2", "m2")}},
	}

	for _, shape := range shapes {
		t.Run(shape.name+"/no_drift", func(t *testing.T) {
			pinState := PinState{
				Specs:   shape.specs,
				Markers: shape.markers,
				Links:   shape.links,
			}
			scanResult := scanResultFromSpecsMarkers(shape.specs, shape.markers)
			pin := &fakePinStore{state: pinState}
			scanner := &fakeScanner{result: scanResult}
			orch := NewOrchestrator(pin, scanner)

			state, err := orch.Todo()
			assertNoError(t, err)
			assertTodoCount(t, state, 0)
		})

		t.Run(shape.name+"/all_drifted", func(t *testing.T) {
			if len(shape.links) == 0 {
				return
			}
			pinState := PinState{
				Specs:   shape.specs,
				Markers: shape.markers,
				Links:   shape.links,
			}
			specOverrides := make(map[string]string)
			for _, s := range shape.specs {
				specOverrides[s.ID] = "changed_" + s.Hash
			}
			markerOverrides := make(map[string]string)
			for _, m := range shape.markers {
				markerOverrides[m.ID] = "changed_" + m.Hash
			}
			scanResult := scanResultWithOverrides(shape.specs, shape.markers, specOverrides, markerOverrides)
			pin := &fakePinStore{state: pinState}
			scanner := &fakeScanner{result: scanResult}
			orch := NewOrchestrator(pin, scanner)

			state, err := orch.Todo()
			assertNoError(t, err)
			assertTodoCount(t, state, len(shape.links))
		})
	}
}

func TestOrchestratorTodoErrorPropagation(t *testing.T) {
	t.Run("pin_load_error", func(t *testing.T) {
		pin := &fakePinStore{loadErr: ErrPinNotFound}
		scanner := &fakeScanner{}
		orch := NewOrchestrator(pin, scanner)
		_, err := orch.Todo()
		assertErrorWraps(t, err, ErrPinNotFound)
	})

	t.Run("scanner_error", func(t *testing.T) {
		pin := &fakePinStore{}
		scanner := &fakeScanner{err: errors.New("scan failed")}
		orch := NewOrchestrator(pin, scanner)
		_, err := orch.Todo()
		if err == nil {
			t.Fatalf("expected error from scanner")
		}
	})
}

func TestOrchestratorTodoDoesNotSave(t *testing.T) {
	t.Run("todo_does_not_call_save", func(t *testing.T) {
		pinState := PinState{
			Specs:   []Spec{newSpec("s1", "b1")},
			Markers: []Marker{newMarker("m1", "b1")},
			Links:   []Link{newLink("s1", "m1")},
		}
		scanResult := scanResultWithOverrides(pinState.Specs, pinState.Markers,
			map[string]string{"s1": "changed"},
			map[string]string{"m1": "changed"})
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)

		_, err := orch.Todo()
		assertNoError(t, err)

		if len(pin.saved) != 0 {
			t.Fatalf("todo should not save, but got %d saves", len(pin.saved))
		}
	})
}

func TestOrchestratorReset(t *testing.T) {
	t.Run("reset_nonexistent_edge_errors", func(t *testing.T) {
		pinState := PinState{
			Specs:   []Spec{newSpec("s1", "b1")},
			Markers: []Marker{newMarker("m1", "b1")},
			Links:   []Link{newLink("s1", "m1")},
		}
		scanResult := scanResultFromSpecsMarkers(pinState.Specs, pinState.Markers)
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)

		_, err := orch.Reset("nonexistent", "nonexistent")
		assertErrorWraps(t, err, ErrResetEdgeNotLinked)
	})

	t.Run("reset_existing_edge_saves_state", func(t *testing.T) {
		specs := []Spec{newSpec("s1", "b1")}
		markers := []Marker{newMarker("m1", "b1")}
		links := []Link{newLink("s1", "m1")}
		pinState := PinState{
			Specs:   specs,
			Markers: markers,
			Links:   links,
		}
		scanResult := scanResultWithOverrides(specs, markers,
			map[string]string{"s1": "changed"},
			map[string]string{"m1": "changed"})
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)

		state, err := orch.Reset("m1", "s1")
		assertNoError(t, err)
		assertResolutionStateCount(t, state, 0)
		assertBaselineHashes(t, state, "s1", "changed", "m1", "changed")

		if len(pin.saved) != 1 {
			t.Fatalf("expected 1 save after reset, got %d", len(pin.saved))
		}
	})

	t.Run("reset_pin_load_error", func(t *testing.T) {
		pin := &fakePinStore{loadErr: ErrPinNotFound}
		scanner := &fakeScanner{}
		orch := NewOrchestrator(pin, scanner)
		_, err := orch.Reset("m1", "s1")
		assertErrorWraps(t, err, ErrPinNotFound)
	})

	t.Run("reset_scanner_error", func(t *testing.T) {
		pin := &fakePinStore{}
		scanner := &fakeScanner{err: errors.New("scan failed")}
		orch := NewOrchestrator(pin, scanner)
		_, err := orch.Reset("m1", "s1")
		if err == nil {
			t.Fatalf("expected error from scanner")
		}
	})

	t.Run("reset_save_error", func(t *testing.T) {
		specs := []Spec{newSpec("s1", "b1")}
		markers := []Marker{newMarker("m1", "b1")}
		links := []Link{newLink("s1", "m1")}
		pinState := PinState{
			Specs:   specs,
			Markers: markers,
			Links:   links,
		}
		scanResult := scanResultWithOverrides(specs, markers,
			map[string]string{"s1": "changed"},
			map[string]string{"m1": "changed"})
		pin := &fakePinStore{state: pinState, saveErr: errors.New("save failed")}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)

		_, err := orch.Reset("m1", "s1")
		if err == nil {
			t.Fatalf("expected error from save failure")
		}
	})
}

func TestOrchestratorResetPartialCollapse(t *testing.T) {
	t.Run("reset_one_of_two_edges_saves_resolution", func(t *testing.T) {
		specs := []Spec{newSpec("s1", "bs")}
		markers := []Marker{newMarker("m1", "bm1"), newMarker("m2", "bm2")}
		links := []Link{newLink("s1", "m1"), newLink("s1", "m2")}
		pinState := PinState{
			Specs:   specs,
			Markers: markers,
			Links:   links,
		}
		scanResult := scanResultWithOverrides(specs, markers,
			map[string]string{"s1": "cs"},
			map[string]string{"m1": "cm1", "m2": "cm2"})
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)

		state, err := orch.Reset("m1", "s1")
		assertNoError(t, err)
		assertResolutionStateCount(t, state, 1)
		assertBaselineHashes(t, state, "s1", "bs", "m1", "cm1")
		assertBaselineHashes(t, state, "", "", "m2", "bm2")

		if len(pin.saved) != 1 {
			t.Fatalf("expected 1 save after reset, got %d", len(pin.saved))
		}
		assertPinStateEquals(t, pin.saved[0], evaluatedStateToPinState(state))
	})
}

func TestOrchestratorReconciliation(t *testing.T) {
	t.Run("empty_pin_discovered_specs_markers_baselines_set_to_current", func(t *testing.T) {
		discoveredSpecs := []Spec{newSpec("s1", "current_h1")}
		discoveredMarkers := []Marker{newMarker("m1", "current_h2")}
		scanResult := scanResultFromSpecsMarkers(discoveredSpecs, discoveredMarkers)
		pin := &fakePinStore{state: PinState{}}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)

		state, err := orch.Todo()
		assertNoError(t, err)
		assertTodoCount(t, state, 0)

		spec := findSpecInEvaluatedState(t, state, "s1")
		if spec.Hash != "current_h1" {
			t.Fatalf("new spec baseline = %q, want %q (current hash)", spec.Hash, "current_h1")
		}
		marker := findMarkerInEvaluatedState(t, state, "m1")
		if marker.Hash != "current_h2" {
			t.Fatalf("new marker baseline = %q, want %q (current hash)", marker.Hash, "current_h2")
		}
	})

	t.Run("pin_with_specs_scan_same_hashes_no_drift", func(t *testing.T) {
		specs := []Spec{newSpec("s1", "h1")}
		markers := []Marker{newMarker("m1", "h2")}
		links := []Link{newLink("s1", "m1")}
		pinState := PinState{Specs: specs, Markers: markers, Links: links}
		scanResult := scanResultFromSpecsMarkers(specs, markers)
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)

		state, err := orch.Todo()
		assertNoError(t, err)
		assertTodoCount(t, state, 0)
	})

	t.Run("pin_with_specs_scan_changed_hash_drift_detected", func(t *testing.T) {
		specs := []Spec{newSpec("s1", "h1")}
		markers := []Marker{newMarker("m1", "h2")}
		links := []Link{newLink("s1", "m1")}
		pinState := PinState{Specs: specs, Markers: markers, Links: links}
		scanResult := scanResultWithOverrides(specs, markers,
			map[string]string{"s1": "changed"},
			nil)
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)

		state, err := orch.Todo()
		assertNoError(t, err)
		assertTodoCount(t, state, 1)
	})

	t.Run("spec_in_pin_not_in_scan_errors", func(t *testing.T) {
		pinState := PinState{
			Specs:   []Spec{newSpec("s1", "h1")},
			Markers: []Marker{newMarker("m1", "h2")},
			Links:   []Link{newLink("s1", "m1")},
		}
		scanResult := ScanResult{
			Specs:   []Spec{},
			Markers: []Marker{newMarker("m1", "h2")},
		}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)

		_, err := orch.Todo()
		if err == nil {
			t.Fatalf("expected error for spec in pin but not in scan")
		}
	})

	t.Run("marker_in_pin_not_in_scan_errors", func(t *testing.T) {
		pinState := PinState{
			Specs:   []Spec{newSpec("s1", "h1")},
			Markers: []Marker{newMarker("m1", "h2")},
			Links:   []Link{newLink("s1", "m1")},
		}
		scanResult := ScanResult{
			Specs:   []Spec{newSpec("s1", "h1")},
			Markers: []Marker{},
		}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)

		_, err := orch.Todo()
		if err == nil {
			t.Fatalf("expected error for marker in pin but not in scan")
		}
	})

	t.Run("new_spec_in_scan_not_in_pin_added_no_drift", func(t *testing.T) {
		pinState := PinState{
			Specs:   []Spec{newSpec("s1", "h1")},
			Markers: []Marker{newMarker("m1", "h2")},
			Links:   []Link{newLink("s1", "m1")},
		}
		scanResult := ScanResult{
			Specs:   []Spec{newSpec("s1", "h1"), newSpec("s2", "h3")},
			Markers: []Marker{newMarker("m1", "h2")},
		}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)

		state, err := orch.Todo()
		assertNoError(t, err)
		assertTodoCount(t, state, 0)

		spec := findSpecInEvaluatedState(t, state, "s2")
		if spec.Hash != "h3" {
			t.Fatalf("new spec baseline = %q, want %q", spec.Hash, "h3")
		}
	})
}

func TestOrchestratorLink(t *testing.T) {
	t.Run("link_adds_link_to_pin", func(t *testing.T) {
		specs := []Spec{newSpec("s1", "h1")}
		markers := []Marker{newMarker("m1", "h2")}
		pinState := PinState{
			Specs:   specs,
			Markers: markers,
		}
		scanResult := scanResultFromSpecsMarkers(specs, markers)
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)

		err := orch.Link("m1", "s1")
		assertNoError(t, err)

		if len(pin.saved) != 1 {
			t.Fatalf("expected 1 save, got %d", len(pin.saved))
		}
		saved := pin.saved[0]
		if len(saved.Links) != 1 {
			t.Fatalf("expected 1 link, got %d", len(saved.Links))
		}
		if saved.Links[0].SpecID != "s1" || saved.Links[0].MarkerID != "m1" {
			t.Fatalf("link = %+v, want {SpecID:s1 MarkerID:m1}", saved.Links[0])
		}
	})

	t.Run("link_nonexistent_marker_errors", func(t *testing.T) {
		specs := []Spec{newSpec("s1", "h1")}
		pinState := PinState{
			Specs: specs,
		}
		scanResult := scanResultFromSpecsMarkers(specs, nil)
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)

		err := orch.Link("nonexistent", "s1")
		if err == nil {
			t.Fatalf("expected error for nonexistent marker")
		}
	})

	t.Run("link_nonexistent_spec_errors", func(t *testing.T) {
		markers := []Marker{newMarker("m1", "h1")}
		pinState := PinState{
			Markers: markers,
		}
		scanResult := scanResultFromSpecsMarkers(nil, markers)
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)

		err := orch.Link("m1", "nonexistent")
		if err == nil {
			t.Fatalf("expected error for nonexistent spec")
		}
	})

	t.Run("link_duplicate_errors", func(t *testing.T) {
		specs := []Spec{newSpec("s1", "h1")}
		markers := []Marker{newMarker("m1", "h2")}
		pinState := PinState{
			Specs:   specs,
			Markers: markers,
			Links:   []Link{newLink("s1", "m1")},
		}
		scanResult := scanResultFromSpecsMarkers(specs, markers)
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)

		err := orch.Link("m1", "s1")
		if err == nil {
			t.Fatalf("expected error for duplicate link")
		}
	})

	t.Run("link_pin_load_error", func(t *testing.T) {
		pin := &fakePinStore{loadErr: ErrPinNotFound}
		scanner := &fakeScanner{}
		orch := NewOrchestrator(pin, scanner)
		err := orch.Link("m1", "s1")
		assertErrorWraps(t, err, ErrPinNotFound)
	})

	t.Run("link_save_error", func(t *testing.T) {
		specs := []Spec{newSpec("s1", "h1")}
		markers := []Marker{newMarker("m1", "h2")}
		pinState := PinState{
			Specs:   specs,
			Markers: markers,
		}
		scanResult := scanResultFromSpecsMarkers(specs, markers)
		pin := &fakePinStore{state: pinState, saveErr: errors.New("save failed")}
		scanner := &fakeScanner{result: scanResult}
		orch := NewOrchestrator(pin, scanner)
		err := orch.Link("m1", "s1")
		if err == nil {
			t.Fatalf("expected error from save failure")
		}
	})
}

func findSpecInEvaluatedState(t *testing.T, state EvaluatedState, id string) Spec {
	t.Helper()
	for _, s := range state.Specs {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("spec %q not found in evaluated state", id)
	return Spec{}
}

func findMarkerInEvaluatedState(t *testing.T, state EvaluatedState, id string) Marker {
	t.Helper()
	for _, m := range state.Markers {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("marker %q not found in evaluated state", id)
	return Marker{}
}

func evaluatedStateToPinState(state EvaluatedState) PinState {
	return PinState{
		Specs:           state.Specs,
		Markers:         state.Markers,
		Links:           state.Links,
		ResolutionState: state.ResolutionState,
	}
}
