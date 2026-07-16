package orchestrator_test

import (
	"errors"
	"testing"

	"driftpin/core"
	"driftpin/internal/testutil"
	"driftpin/orchestrator"
	"driftpin/pinstore"
	"driftpin/scanner"
)

type fakePinStore struct {
	state   pinstore.PinState
	loadErr error
	saveErr error
	saved   []pinstore.PinState
}

func (f *fakePinStore) Load() (pinstore.PinState, error) {
	if f.loadErr != nil {
		return pinstore.PinState{}, f.loadErr
	}
	return f.state, nil
}

func (f *fakePinStore) Save(state pinstore.PinState) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.state = state
	f.saved = append(f.saved, state)
	return nil
}

type fakeScanner struct {
	result scanner.ScanResult
	err    error
}

func (f *fakeScanner) Scan() (scanner.ScanResult, error) {
	if f.err != nil {
		return scanner.ScanResult{}, f.err
	}
	return f.result, nil
}

func scanResultFromSpecsMarkers(specs []core.Spec, markers []core.Marker) scanner.ScanResult {
	return scanner.ScanResult{
		Specs:   specs,
		Markers: markers,
	}
}

func scanResultWithOverrides(specs []core.Spec, markers []core.Marker, specOverrides, markerOverrides map[string]string) scanner.ScanResult {
	resultSpecs := make([]core.Spec, len(specs))
	for i, s := range specs {
		resultSpecs[i] = s
		if h, ok := specOverrides[s.ID]; ok {
			resultSpecs[i].Hash = h
		}
	}
	resultMarkers := make([]core.Marker, len(markers))
	for i, m := range markers {
		resultMarkers[i] = m
		if h, ok := markerOverrides[m.ID]; ok {
			resultMarkers[i].Hash = h
		}
	}
	return scanner.ScanResult{Specs: resultSpecs, Markers: resultMarkers}
}

func TestOrchestratorInit(t *testing.T) {
	t.Run("init_saves_empty_state", func(t *testing.T) {
		pin := &fakePinStore{}
		scanner := &fakeScanner{}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		err := orch.Init()
		testutil.AssertNoError(t, err)

		if len(pin.saved) != 1 {
			t.Fatalf("expected 1 save, got %d", len(pin.saved))
		}
		testutil.AssertPinStateEquals(t, pin.saved[0], pinstore.PinState{})
	})

	t.Run("init_propagates_save_error", func(t *testing.T) {
		pin := &fakePinStore{saveErr: errors.New("write failed")}
		scanner := &fakeScanner{}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		err := orch.Init()
		if err == nil {
			t.Fatalf("expected error from save failure")
		}
	})
}

func TestOrchestratorTodoArity(t *testing.T) {
	shapes := []struct {
		name    string
		specs   []core.Spec
		markers []core.Marker
		links   []core.Link
	}{
		{"0_specs_0_markers", nil, nil, nil},
		{"1_spec_0_markers", []core.Spec{testutil.NewSpec("s1", "b1")}, nil, nil},
		{"0_specs_1_marker", nil, []core.Marker{testutil.NewMarker("m1", "b1")}, nil},
		{"1_spec_1_marker", []core.Spec{testutil.NewSpec("s1", "b1")}, []core.Marker{testutil.NewMarker("m1", "b1")}, []core.Link{testutil.NewLink("s1", "m1")}},
		{"many_specs_1_marker", []core.Spec{testutil.NewSpec("s1", "b1")}, []core.Marker{testutil.NewMarker("m1", "b1"), testutil.NewMarker("m2", "b2")}, []core.Link{testutil.NewLink("s1", "m1"), testutil.NewLink("s1", "m2")}},
		{"1_spec_many_markers", []core.Spec{testutil.NewSpec("s1", "b1"), testutil.NewSpec("s2", "b2")}, []core.Marker{testutil.NewMarker("m1", "b1")}, []core.Link{testutil.NewLink("s1", "m1"), testutil.NewLink("s2", "m1")}},
		{"many_specs_many_markers", []core.Spec{testutil.NewSpec("s1", "b1"), testutil.NewSpec("s2", "b2")}, []core.Marker{testutil.NewMarker("m1", "b1"), testutil.NewMarker("m2", "b2")}, []core.Link{testutil.NewLink("s1", "m1"), testutil.NewLink("s1", "m2"), testutil.NewLink("s2", "m1"), testutil.NewLink("s2", "m2")}},
	}

	for _, shape := range shapes {
		t.Run(shape.name+"/no_drift", func(t *testing.T) {
			pinState := pinstore.PinState{
				Specs:   shape.specs,
				Markers: shape.markers,
				Links:   shape.links,
			}
			scanResult := scanResultFromSpecsMarkers(shape.specs, shape.markers)
			pin := &fakePinStore{state: pinState}
			scanner := &fakeScanner{result: scanResult}
			orch := orchestrator.NewOrchestrator(pin, scanner)

			state, err := orch.Todo()
			testutil.AssertNoError(t, err)
			testutil.AssertTodoCount(t, state, 0)
		})

		t.Run(shape.name+"/all_drifted", func(t *testing.T) {
			if len(shape.links) == 0 {
				return
			}
			pinState := pinstore.PinState{
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
			orch := orchestrator.NewOrchestrator(pin, scanner)

			state, err := orch.Todo()
			testutil.AssertNoError(t, err)
			testutil.AssertTodoCount(t, state, len(shape.links))
		})
	}
}

func TestOrchestratorTodoErrorPropagation(t *testing.T) {
	t.Run("pin_load_error", func(t *testing.T) {
		pin := &fakePinStore{loadErr: pinstore.ErrPinNotFound}
		scanner := &fakeScanner{}
		orch := orchestrator.NewOrchestrator(pin, scanner)
		_, err := orch.Todo()
		testutil.AssertErrorWraps(t, err, pinstore.ErrPinNotFound)
	})

	t.Run("scanner_error", func(t *testing.T) {
		pin := &fakePinStore{}
		scanner := &fakeScanner{err: errors.New("scan failed")}
		orch := orchestrator.NewOrchestrator(pin, scanner)
		_, err := orch.Todo()
		if err == nil {
			t.Fatalf("expected error from scanner")
		}
	})
}

func TestOrchestratorTodoDoesNotSave(t *testing.T) {
	t.Run("todo_does_not_call_save", func(t *testing.T) {
		pinState := pinstore.PinState{
			Specs:   []core.Spec{testutil.NewSpec("s1", "b1")},
			Markers: []core.Marker{testutil.NewMarker("m1", "b1")},
			Links:   []core.Link{testutil.NewLink("s1", "m1")},
		}
		scanResult := scanResultWithOverrides(pinState.Specs, pinState.Markers,
			map[string]string{"s1": "changed"},
			map[string]string{"m1": "changed"})
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		_, err := orch.Todo()
		testutil.AssertNoError(t, err)

		if len(pin.saved) != 0 {
			t.Fatalf("todo should not save, but got %d saves", len(pin.saved))
		}
	})
}

func TestOrchestratorReset(t *testing.T) {
	t.Run("reset_nonexistent_edge_errors", func(t *testing.T) {
		pinState := pinstore.PinState{
			Specs:   []core.Spec{testutil.NewSpec("s1", "b1")},
			Markers: []core.Marker{testutil.NewMarker("m1", "b1")},
			Links:   []core.Link{testutil.NewLink("s1", "m1")},
		}
		scanResult := scanResultFromSpecsMarkers(pinState.Specs, pinState.Markers)
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		_, err := orch.Reset("nonexistent", "nonexistent")
		testutil.AssertErrorWraps(t, err, core.ErrResetEdgeNotLinked)
	})

	t.Run("reset_existing_edge_saves_state", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "b1")}
		markers := []core.Marker{testutil.NewMarker("m1", "b1")}
		links := []core.Link{testutil.NewLink("s1", "m1")}
		pinState := pinstore.PinState{
			Specs:   specs,
			Markers: markers,
			Links:   links,
		}
		scanResult := scanResultWithOverrides(specs, markers,
			map[string]string{"s1": "changed"},
			map[string]string{"m1": "changed"})
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		state, err := orch.Reset("m1", "s1")
		testutil.AssertNoError(t, err)
		testutil.AssertResolutionStateCount(t, state, 0)
		testutil.AssertBaselineHashes(t, state, "s1", "changed", "m1", "changed")

		if len(pin.saved) != 1 {
			t.Fatalf("expected 1 save after reset, got %d", len(pin.saved))
		}
	})

	t.Run("reset_pin_load_error", func(t *testing.T) {
		pin := &fakePinStore{loadErr: pinstore.ErrPinNotFound}
		scanner := &fakeScanner{}
		orch := orchestrator.NewOrchestrator(pin, scanner)
		_, err := orch.Reset("m1", "s1")
		testutil.AssertErrorWraps(t, err, pinstore.ErrPinNotFound)
	})

	t.Run("reset_scanner_error", func(t *testing.T) {
		pin := &fakePinStore{}
		scanner := &fakeScanner{err: errors.New("scan failed")}
		orch := orchestrator.NewOrchestrator(pin, scanner)
		_, err := orch.Reset("m1", "s1")
		if err == nil {
			t.Fatalf("expected error from scanner")
		}
	})

	t.Run("reset_save_error", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "b1")}
		markers := []core.Marker{testutil.NewMarker("m1", "b1")}
		links := []core.Link{testutil.NewLink("s1", "m1")}
		pinState := pinstore.PinState{
			Specs:   specs,
			Markers: markers,
			Links:   links,
		}
		scanResult := scanResultWithOverrides(specs, markers,
			map[string]string{"s1": "changed"},
			map[string]string{"m1": "changed"})
		pin := &fakePinStore{state: pinState, saveErr: errors.New("save failed")}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		_, err := orch.Reset("m1", "s1")
		if err == nil {
			t.Fatalf("expected error from save failure")
		}
	})
}

func TestOrchestratorResetPartialCollapse(t *testing.T) {
	t.Run("reset_one_of_two_edges_saves_resolution", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "bs")}
		markers := []core.Marker{testutil.NewMarker("m1", "bm1"), testutil.NewMarker("m2", "bm2")}
		links := []core.Link{testutil.NewLink("s1", "m1"), testutil.NewLink("s1", "m2")}
		pinState := pinstore.PinState{
			Specs:   specs,
			Markers: markers,
			Links:   links,
		}
		scanResult := scanResultWithOverrides(specs, markers,
			map[string]string{"s1": "cs"},
			map[string]string{"m1": "cm1", "m2": "cm2"})
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		state, err := orch.Reset("m1", "s1")
		testutil.AssertNoError(t, err)
		testutil.AssertResolutionStateCount(t, state, 1)
		testutil.AssertBaselineHashes(t, state, "s1", "bs", "m1", "cm1")
		testutil.AssertBaselineHashes(t, state, "", "", "m2", "bm2")

		if len(pin.saved) != 1 {
			t.Fatalf("expected 1 save after reset, got %d", len(pin.saved))
		}
		testutil.AssertPinStateEquals(t, pin.saved[0], testutil.EvaluatedStateToPinState(state))
	})
}

func TestOrchestratorReconciliation(t *testing.T) {
	t.Run("empty_pin_discovered_specs_markers_baselines_set_to_current", func(t *testing.T) {
		discoveredSpecs := []core.Spec{testutil.NewSpec("s1", "current_h1")}
		discoveredMarkers := []core.Marker{testutil.NewMarker("m1", "current_h2")}
		scanResult := scanResultFromSpecsMarkers(discoveredSpecs, discoveredMarkers)
		pin := &fakePinStore{state: pinstore.PinState{}}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		state, err := orch.Todo()
		testutil.AssertNoError(t, err)
		testutil.AssertTodoCount(t, state, 0)

		spec := testutil.FindSpecInEvaluatedState(t, state, "s1")
		if spec.Hash != "current_h1" {
			t.Fatalf("new spec baseline = %q, want %q (current hash)", spec.Hash, "current_h1")
		}
		marker := testutil.FindMarkerInEvaluatedState(t, state, "m1")
		if marker.Hash != "current_h2" {
			t.Fatalf("new marker baseline = %q, want %q (current hash)", marker.Hash, "current_h2")
		}
	})

	t.Run("pin_with_specs_scan_same_hashes_no_drift", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "h1")}
		markers := []core.Marker{testutil.NewMarker("m1", "h2")}
		links := []core.Link{testutil.NewLink("s1", "m1")}
		pinState := pinstore.PinState{Specs: specs, Markers: markers, Links: links}
		scanResult := scanResultFromSpecsMarkers(specs, markers)
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		state, err := orch.Todo()
		testutil.AssertNoError(t, err)
		testutil.AssertTodoCount(t, state, 0)
	})

	t.Run("pin_with_specs_scan_changed_hash_drift_detected", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "h1")}
		markers := []core.Marker{testutil.NewMarker("m1", "h2")}
		links := []core.Link{testutil.NewLink("s1", "m1")}
		pinState := pinstore.PinState{Specs: specs, Markers: markers, Links: links}
		scanResult := scanResultWithOverrides(specs, markers,
			map[string]string{"s1": "changed"},
			nil)
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		state, err := orch.Todo()
		testutil.AssertNoError(t, err)
		testutil.AssertTodoCount(t, state, 1)
	})

	t.Run("spec_in_pin_not_in_scan_kept_as_stale", func(t *testing.T) {
		pinState := pinstore.PinState{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h2")},
			Links:   []core.Link{testutil.NewLink("s1", "m1")},
		}
		scanResult := scanner.ScanResult{
			Specs:   []core.Spec{},
			Markers: []core.Marker{testutil.NewMarker("m1", "h2")},
		}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		state, err := orch.Todo()
		testutil.AssertNoError(t, err)
		testutil.AssertTodoCount(t, state, 1)
		if len(state.Specs) != 1 {
			t.Fatalf("expected 1 spec (stale), got %d", len(state.Specs))
		}
		if state.Specs[0].ID != "s1" {
			t.Fatalf("expected stale spec s1, got %q", state.Specs[0].ID)
		}
		if len(state.Todos) > 0 && !state.Todos[0].SpecDeleted {
			t.Fatalf("expected SpecDeleted=true on todo")
		}
	})

	t.Run("marker_in_pin_not_in_scan_kept_as_stale", func(t *testing.T) {
		pinState := pinstore.PinState{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h2")},
			Links:   []core.Link{testutil.NewLink("s1", "m1")},
		}
		scanResult := scanner.ScanResult{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1")},
			Markers: []core.Marker{},
		}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		state, err := orch.Todo()
		testutil.AssertNoError(t, err)
		testutil.AssertTodoCount(t, state, 1)
		if len(state.Markers) != 1 {
			t.Fatalf("expected 1 marker (stale), got %d", len(state.Markers))
		}
		if state.Markers[0].ID != "m1" {
			t.Fatalf("expected stale marker m1, got %q", state.Markers[0].ID)
		}
		if len(state.Todos) > 0 && !state.Todos[0].MarkerDeleted {
			t.Fatalf("expected MarkerDeleted=true on todo")
		}
	})

	t.Run("new_spec_in_scan_not_in_pin_added_no_drift", func(t *testing.T) {
		pinState := pinstore.PinState{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h2")},
			Links:   []core.Link{testutil.NewLink("s1", "m1")},
		}
		scanResult := scanner.ScanResult{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1"), testutil.NewSpec("s2", "h3")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h2")},
		}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		state, err := orch.Todo()
		testutil.AssertNoError(t, err)
		testutil.AssertTodoCount(t, state, 0)

		spec := testutil.FindSpecInEvaluatedState(t, state, "s2")
		if spec.Hash != "h3" {
			t.Fatalf("new spec baseline = %q, want %q", spec.Hash, "h3")
		}
	})
}

func TestOrchestratorLink(t *testing.T) {
	t.Run("link_adds_link_to_pin", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "h1")}
		markers := []core.Marker{testutil.NewMarker("m1", "h2")}
		pinState := pinstore.PinState{
			Specs:   specs,
			Markers: markers,
		}
		scanResult := scanResultFromSpecsMarkers(specs, markers)
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		err := orch.Link("m1", "s1")
		testutil.AssertNoError(t, err)

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
		specs := []core.Spec{testutil.NewSpec("s1", "h1")}
		pinState := pinstore.PinState{
			Specs: specs,
		}
		scanResult := scanResultFromSpecsMarkers(specs, nil)
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		err := orch.Link("nonexistent", "s1")
		if err == nil {
			t.Fatalf("expected error for nonexistent marker")
		}
	})

	t.Run("link_nonexistent_spec_errors", func(t *testing.T) {
		markers := []core.Marker{testutil.NewMarker("m1", "h1")}
		pinState := pinstore.PinState{
			Markers: markers,
		}
		scanResult := scanResultFromSpecsMarkers(nil, markers)
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		err := orch.Link("m1", "nonexistent")
		if err == nil {
			t.Fatalf("expected error for nonexistent spec")
		}
	})

	t.Run("link_duplicate_errors", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "h1")}
		markers := []core.Marker{testutil.NewMarker("m1", "h2")}
		pinState := pinstore.PinState{
			Specs:   specs,
			Markers: markers,
			Links:   []core.Link{testutil.NewLink("s1", "m1")},
		}
		scanResult := scanResultFromSpecsMarkers(specs, markers)
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		err := orch.Link("m1", "s1")
		if err == nil {
			t.Fatalf("expected error for duplicate link")
		}
	})

	t.Run("link_pin_load_error", func(t *testing.T) {
		pin := &fakePinStore{loadErr: pinstore.ErrPinNotFound}
		scanner := &fakeScanner{}
		orch := orchestrator.NewOrchestrator(pin, scanner)
		err := orch.Link("m1", "s1")
		testutil.AssertErrorWraps(t, err, pinstore.ErrPinNotFound)
	})

	t.Run("link_save_error", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "h1")}
		markers := []core.Marker{testutil.NewMarker("m1", "h2")}
		pinState := pinstore.PinState{
			Specs:   specs,
			Markers: markers,
		}
		scanResult := scanResultFromSpecsMarkers(specs, markers)
		pin := &fakePinStore{state: pinState, saveErr: errors.New("save failed")}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)
		err := orch.Link("m1", "s1")
		if err == nil {
			t.Fatalf("expected error from save failure")
		}
	})
}

func TestOrchestratorUnlink(t *testing.T) {
	t.Run("unlink_removes_link", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "h1")}
		markers := []core.Marker{testutil.NewMarker("m1", "h2")}
		links := []core.Link{testutil.NewLink("s1", "m1")}
		pinState := pinstore.PinState{
			Specs:   specs,
			Markers: markers,
			Links:   links,
		}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		err := orch.Unlink("m1", "s1")
		testutil.AssertNoError(t, err)

		if len(pin.saved) != 1 {
			t.Fatalf("expected 1 save, got %d", len(pin.saved))
		}
		saved := pin.saved[0]
		if len(saved.Links) != 0 {
			t.Fatalf("expected 0 links after unlink, got %d", len(saved.Links))
		}
		if len(saved.Specs) != 1 {
			t.Fatalf("specs should be preserved, got %d", len(saved.Specs))
		}
		if len(saved.Markers) != 1 {
			t.Fatalf("markers should be preserved, got %d", len(saved.Markers))
		}
	})

	t.Run("unlink_nonexistent_link_errors", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "h1")}
		markers := []core.Marker{testutil.NewMarker("m1", "h2")}
		pinState := pinstore.PinState{
			Specs:   specs,
			Markers: markers,
		}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		err := orch.Unlink("m1", "s1")
		if err == nil {
			t.Fatalf("expected error for nonexistent link")
		}
		testutil.AssertErrorWraps(t, err, orchestrator.ErrUnlinkNotFound)
	})

	t.Run("unlink_removes_resolution_state", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "h1")}
		markers := []core.Marker{testutil.NewMarker("m1", "h2")}
		links := []core.Link{testutil.NewLink("s1", "m1")}
		resolutions := []core.ResolutionState{
			{SpecID: "s1", MarkerID: "m1", CurrentSpecHash: "h1", CurrentMarkerHash: "h2"},
		}
		pinState := pinstore.PinState{
			Specs:           specs,
			Markers:         markers,
			Links:           links,
			ResolutionState: resolutions,
		}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		err := orch.Unlink("m1", "s1")
		testutil.AssertNoError(t, err)

		saved := pin.saved[0]
		if len(saved.ResolutionState) != 0 {
			t.Fatalf("expected 0 resolutions after unlink, got %d", len(saved.ResolutionState))
		}
	})

	t.Run("unlink_preserves_other_links_and_resolutions", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "h1"), testutil.NewSpec("s2", "h3")}
		markers := []core.Marker{testutil.NewMarker("m1", "h2"), testutil.NewMarker("m2", "h4")}
		links := []core.Link{
			testutil.NewLink("s1", "m1"),
			testutil.NewLink("s2", "m2"),
		}
		resolutions := []core.ResolutionState{
			{SpecID: "s1", MarkerID: "m1", CurrentSpecHash: "h1", CurrentMarkerHash: "h2"},
			{SpecID: "s2", MarkerID: "m2", CurrentSpecHash: "h3", CurrentMarkerHash: "h4"},
		}
		pinState := pinstore.PinState{
			Specs:           specs,
			Markers:         markers,
			Links:           links,
			ResolutionState: resolutions,
		}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		err := orch.Unlink("m1", "s1")
		testutil.AssertNoError(t, err)

		saved := pin.saved[0]
		if len(saved.Links) != 1 {
			t.Fatalf("expected 1 link after unlink, got %d", len(saved.Links))
		}
		if saved.Links[0].SpecID != "s2" || saved.Links[0].MarkerID != "m2" {
			t.Fatalf("remaining link should be s2/m2, got %+v", saved.Links[0])
		}
		if len(saved.ResolutionState) != 1 {
			t.Fatalf("expected 1 resolution after unlink, got %d", len(saved.ResolutionState))
		}
		if saved.ResolutionState[0].SpecID != "s2" || saved.ResolutionState[0].MarkerID != "m2" {
			t.Fatalf("remaining resolution should be for s2/m2, got %+v", saved.ResolutionState[0])
		}
	})

	t.Run("unlink_pin_load_error", func(t *testing.T) {
		pin := &fakePinStore{loadErr: pinstore.ErrPinNotFound}
		scanner := &fakeScanner{}
		orch := orchestrator.NewOrchestrator(pin, scanner)
		err := orch.Unlink("m1", "s1")
		testutil.AssertErrorWraps(t, err, pinstore.ErrPinNotFound)
	})

	t.Run("unlink_save_error", func(t *testing.T) {
		links := []core.Link{testutil.NewLink("s1", "m1")}
		pinState := pinstore.PinState{
			Links: links,
		}
		pin := &fakePinStore{state: pinState, saveErr: errors.New("save failed")}
		scanner := &fakeScanner{}
		orch := orchestrator.NewOrchestrator(pin, scanner)
		err := orch.Unlink("m1", "s1")
		if err == nil {
			t.Fatalf("expected error from save failure")
		}
	})
}

func TestOrchestratorStaleEntryDrift(t *testing.T) {
	t.Run("spec_deleted_with_links_drift_detected", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "h1")}
		markers := []core.Marker{testutil.NewMarker("m1", "h2")}
		links := []core.Link{testutil.NewLink("s1", "m1")}
		pinState := pinstore.PinState{Specs: specs, Markers: markers, Links: links}
		scanResult := scanner.ScanResult{
			Specs:   []core.Spec{},
			Markers: markers,
		}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		state, err := orch.Todo()
		testutil.AssertNoError(t, err)
		testutil.AssertTodoCount(t, state, 1)
		if !state.Todos[0].SpecDeleted {
			t.Fatalf("expected SpecDeleted=true")
		}
	})

	t.Run("marker_deleted_with_links_drift_detected", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "h1")}
		markers := []core.Marker{testutil.NewMarker("m1", "h2")}
		links := []core.Link{testutil.NewLink("s1", "m1")}
		pinState := pinstore.PinState{Specs: specs, Markers: markers, Links: links}
		scanResult := scanner.ScanResult{
			Specs:   specs,
			Markers: []core.Marker{},
		}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		state, err := orch.Todo()
		testutil.AssertNoError(t, err)
		testutil.AssertTodoCount(t, state, 1)
		if !state.Todos[0].MarkerDeleted {
			t.Fatalf("expected MarkerDeleted=true")
		}
	})

	t.Run("spec_deleted_with_links_reset_prunes", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "h1")}
		markers := []core.Marker{testutil.NewMarker("m1", "h2")}
		links := []core.Link{testutil.NewLink("s1", "m1")}
		pinState := pinstore.PinState{Specs: specs, Markers: markers, Links: links}
		scanResult := scanner.ScanResult{
			Specs:   []core.Spec{},
			Markers: markers,
		}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		state, err := orch.Reset("m1", "s1")
		testutil.AssertNoError(t, err)
		if len(state.Specs) != 0 {
			t.Fatalf("expected 0 specs after prune, got %d", len(state.Specs))
		}
		if len(state.Links) != 0 {
			t.Fatalf("expected 0 links after prune, got %d", len(state.Links))
		}
	})

	t.Run("marker_deleted_with_links_reset_prunes", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "h1")}
		markers := []core.Marker{testutil.NewMarker("m1", "h2")}
		links := []core.Link{testutil.NewLink("s1", "m1")}
		pinState := pinstore.PinState{Specs: specs, Markers: markers, Links: links}
		scanResult := scanner.ScanResult{
			Specs:   specs,
			Markers: []core.Marker{},
		}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		state, err := orch.Reset("m1", "s1")
		testutil.AssertNoError(t, err)
		if len(state.Markers) != 0 {
			t.Fatalf("expected 0 markers after prune, got %d", len(state.Markers))
		}
		if len(state.Links) != 0 {
			t.Fatalf("expected 0 links after prune, got %d", len(state.Links))
		}
	})
}

func TestOrchestratorResetOrphan(t *testing.T) {
	t.Run("remove_orphan_spec", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("main.deleted", "h1")}
		pinState := pinstore.PinState{Specs: specs}
		scanResult := scanner.ScanResult{}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		err := orch.ResetOrphan("main.deleted")
		testutil.AssertNoError(t, err)
		if len(pin.saved) != 1 {
			t.Fatalf("expected 1 save, got %d", len(pin.saved))
		}
		if len(pin.saved[0].Specs) != 0 {
			t.Fatalf("expected 0 specs after orphan removal, got %d", len(pin.saved[0].Specs))
		}
	})

	t.Run("remove_orphan_marker", func(t *testing.T) {
		markers := []core.Marker{testutil.NewMarker("stale_m", "h1")}
		pinState := pinstore.PinState{Markers: markers}
		scanResult := scanner.ScanResult{}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		err := orch.ResetOrphan("stale_m")
		testutil.AssertNoError(t, err)
		if len(pin.saved[0].Markers) != 0 {
			t.Fatalf("expected 0 markers after orphan removal, got %d", len(pin.saved[0].Markers))
		}
	})

	t.Run("remove_orphan_spec_still_on_disk_errors", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("main.live", "h1")}
		pinState := pinstore.PinState{Specs: specs}
		scanResult := scanner.ScanResult{Specs: specs}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		err := orch.ResetOrphan("main.live")
		testutil.AssertErrorWraps(t, err, orchestrator.ErrOrphanStillOnDisk)
	})

	t.Run("remove_orphan_spec_with_links_errors", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("main.linked", "h1")}
		markers := []core.Marker{testutil.NewMarker("m1", "h2")}
		links := []core.Link{testutil.NewLink("main.linked", "m1")}
		pinState := pinstore.PinState{Specs: specs, Markers: markers, Links: links}
		scanResult := scanner.ScanResult{Markers: markers}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		err := orch.ResetOrphan("main.linked")
		testutil.AssertErrorWraps(t, err, orchestrator.ErrOrphanHasLinks)
	})

	t.Run("remove_orphan_nonexistent_errors", func(t *testing.T) {
		pinState := pinstore.PinState{}
		scanResult := scanner.ScanResult{}
		pin := &fakePinStore{state: pinState}
		scanner := &fakeScanner{result: scanResult}
		orch := orchestrator.NewOrchestrator(pin, scanner)

		err := orch.ResetOrphan("nonexistent")
		testutil.AssertErrorWraps(t, err, orchestrator.ErrOrphanNotFound)
	})
}
