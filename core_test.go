package main

import (
	"errors"
	"testing"
)

func newSpec(id string, hash string) Spec {
	return Spec{ID: id, Hash: hash, Filepath: id + ".xml", LineNumber: 10}
}

func newSpecWithLocation(id string, hash string, filepath string, lineNumber int) Spec {
	return Spec{ID: id, Hash: hash, Filepath: filepath, LineNumber: lineNumber}
}

func newMarker(id string, hash string) Marker {
	return Marker{ID: id, Hash: hash, Filepath: id + ".go", LineNumber: 20}
}

func newMarkerWithLocation(id string, hash string, filepath string, lineNumber int) Marker {
	return Marker{ID: id, Hash: hash, Filepath: filepath, LineNumber: lineNumber}
}

func newLink(specID string, markerID string) Link {
	return Link{SpecID: specID, MarkerID: markerID}
}

func newResolutionState(specID string, markerID string, currentSpecHash string, currentMarkerHash string) ResolutionState {
	return ResolutionState{
		SpecID:            specID,
		MarkerID:          markerID,
		CurrentSpecHash:   currentSpecHash,
		CurrentMarkerHash: currentMarkerHash,
	}
}

func newScanFromBaselines(specs []Spec, markers []Marker, specHashOverrides map[string]string, markerHashOverrides map[string]string) Scan {
	specHashes := make(map[string]string, len(specs))
	for _, spec := range specs {
		specHashes[spec.ID] = spec.Hash
	}
	for id, hash := range specHashOverrides {
		specHashes[id] = hash
	}
	markerHashes := make(map[string]string, len(markers))
	for _, marker := range markers {
		markerHashes[marker.ID] = marker.Hash
	}
	for id, hash := range markerHashOverrides {
		markerHashes[id] = hash
	}
	return Scan{SpecHashes: specHashes, MarkerHashes: markerHashes}
}

func newBaselineScan(specs []Spec, markers []Marker) Scan {
	return newScanFromBaselines(specs, markers, nil, nil)
}

func findSpecByID(t *testing.T, evaluatedState EvaluatedState, specID string) Spec {
	t.Helper()
	for _, spec := range evaluatedState.Specs {
		if spec.ID == specID {
			return spec
		}
	}
	t.Fatalf("spec %q not found in evaluated state", specID)
	return Spec{}
}

func findMarkerByID(t *testing.T, evaluatedState EvaluatedState, markerID string) Marker {
	t.Helper()
	for _, marker := range evaluatedState.Markers {
		if marker.ID == markerID {
			return marker
		}
	}
	t.Fatalf("marker %q not found in evaluated state", markerID)
	return Marker{}
}

func findResolutionStateByEdge(evaluatedState EvaluatedState, markerID string, specID string) (ResolutionState, bool) {
	for _, res := range evaluatedState.ResolutionState {
		if res.MarkerID == markerID && res.SpecID == specID {
			return res, true
		}
	}
	return ResolutionState{}, false
}

func findTodoByEdge(evaluatedState EvaluatedState, markerID string, specID string) (Todo, bool) {
	for _, todo := range evaluatedState.Todos {
		if todo.MarkerID == markerID && todo.SpecID == specID {
			return todo, true
		}
	}
	return Todo{}, false
}

func evaluateTodoActionExpectingSuccess(t *testing.T, ctx CoreAlgorithmContext) EvaluatedState {
	t.Helper()
	evaluatedState, err := NewCoreAlgorithm().EvaluateState(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return evaluatedState
}

func evaluateResetActionExpectingSuccess(t *testing.T, ctx CoreAlgorithmContext) EvaluatedState {
	t.Helper()
	evaluatedState, err := NewCoreAlgorithm().EvaluateState(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return evaluatedState
}

func assertErrorWraps(t *testing.T, err error, target error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error wrapping %v, got nil", target)
	}
	if !errors.Is(err, target) {
		t.Fatalf("expected error wrapping %v, got %v", target, err)
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertTodoCount(t *testing.T, evaluatedState EvaluatedState, want int) {
	t.Helper()
	if len(evaluatedState.Todos) != want {
		t.Fatalf("todo count = %d, want %d (todos=%v)", len(evaluatedState.Todos), want, evaluatedState.Todos)
	}
}

func assertResolutionStateCount(t *testing.T, evaluatedState EvaluatedState, want int) {
	t.Helper()
	if len(evaluatedState.ResolutionState) != want {
		t.Fatalf("resolution count = %d, want %d (res=%v)", len(evaluatedState.ResolutionState), want, evaluatedState.ResolutionState)
	}
}

func assertResolutionStateEntry(t *testing.T, evaluatedState EvaluatedState, markerID string, specID string, currentSpecHash string, currentMarkerHash string) {
	t.Helper()
	res, ok := findResolutionStateByEdge(evaluatedState, markerID, specID)
	if !ok {
		t.Fatalf("expected resolution entry for %s:%s, none found (res=%v)", markerID, specID, evaluatedState.ResolutionState)
	}
	if res.CurrentSpecHash != currentSpecHash || res.CurrentMarkerHash != currentMarkerHash {
		t.Fatalf("resolution %s:%s = (spec=%q marker=%q), want (spec=%q marker=%q)",
			markerID, specID, res.CurrentSpecHash, res.CurrentMarkerHash, currentSpecHash, currentMarkerHash)
	}
}

func assertBaselineHashes(t *testing.T, evaluatedState EvaluatedState, specID string, wantSpecHash string, markerID string, wantMarkerHash string) {
	t.Helper()
	if specID != "" {
		if spec := findSpecByID(t, evaluatedState, specID); spec.Hash != wantSpecHash {
			t.Fatalf("spec %q hash = %q, want %q", specID, spec.Hash, wantSpecHash)
		}
	}
	if markerID != "" {
		if marker := findMarkerByID(t, evaluatedState, markerID); marker.Hash != wantMarkerHash {
			t.Fatalf("marker %q hash = %q, want %q", markerID, marker.Hash, wantMarkerHash)
		}
	}
}

func assertTodoDriftFlags(t *testing.T, todo Todo, wantSpecChanged bool, wantMarkerChanged bool) {
	t.Helper()
	if todo.SpecChanged != wantSpecChanged || todo.MarkerChanged != wantMarkerChanged {
		t.Fatalf("todo %s:%s flags = (spec=%v marker=%v), want (spec=%v marker=%v)",
			todo.MarkerID, todo.SpecID, todo.SpecChanged, todo.MarkerChanged, wantSpecChanged, wantMarkerChanged)
	}
}

func TestArityShapes(t *testing.T) {
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
		t.Run(shape.name, func(t *testing.T) {
			baselineScan := newBaselineScan(shape.specs, shape.markers)
			todoCtx := CoreAlgorithmContext{
				Specs:   shape.specs,
				Markers: shape.markers,
				Links:   shape.links,
				Action:  TodoAction{Scan: baselineScan},
			}
			evaluatedState := evaluateTodoActionExpectingSuccess(t, todoCtx)
			assertTodoCount(t, evaluatedState, 0)
			assertResolutionStateCount(t, evaluatedState, 0)

			resetCtx := CoreAlgorithmContext{
				Specs:   shape.specs,
				Markers: shape.markers,
				Links:   shape.links,
				Action:  ResetAction{SpecID: "nonexistent_spec", MarkerID: "nonexistent_marker", Scan: baselineScan},
			}
			_, err := NewCoreAlgorithm().EvaluateState(resetCtx)
			if err == nil {
				t.Fatalf("expected reset on nonexistent edge to error, got nil")
			}
		})
	}
}

func TestIsolatedNodeWithDriftStillReportsZeroTodos(t *testing.T) {
	specs := []Spec{newSpec("s1", "baseline_hash")}
	markers := []Marker{newMarker("m1", "baseline_hash")}
	driftedScan := newScanFromBaselines(specs, markers,
		map[string]string{"s1": "changed_hash"},
		map[string]string{"m1": "changed_hash"})

	t.Run("isolated_drifted_spec", func(t *testing.T) {
		evaluatedState := evaluateTodoActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs:   specs,
			Markers: nil,
			Links:   nil,
			Action:  TodoAction{Scan: Scan{SpecHashes: driftedScan.SpecHashes, MarkerHashes: map[string]string{}}},
		})
		assertTodoCount(t, evaluatedState, 0)
	})
	t.Run("isolated_drifted_marker", func(t *testing.T) {
		evaluatedState := evaluateTodoActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs:   nil,
			Markers: markers,
			Links:   nil,
			Action:  TodoAction{Scan: Scan{SpecHashes: map[string]string{}, MarkerHashes: driftedScan.MarkerHashes}},
		})
		assertTodoCount(t, evaluatedState, 0)
	})
	t.Run("isolated_drifted_spec_and_marker_no_link", func(t *testing.T) {
		evaluatedState := evaluateTodoActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs:   specs,
			Markers: markers,
			Links:   nil,
			Action:  TodoAction{Scan: driftedScan},
		})
		assertTodoCount(t, evaluatedState, 0)
	})
}

func TestTodoFieldsRoundTrip(t *testing.T) {
	specs := []Spec{newSpecWithLocation("spec_auth", "old_hash", "/project/specs/auth.xml", 42)}
	markers := []Marker{newMarkerWithLocation("marker_auth", "old_hash", "/project/src/auth.go", 88)}
	links := []Link{newLink("spec_auth", "marker_auth")}
	scan := newScanFromBaselines(specs, markers,
		map[string]string{"spec_auth": "new_hash"},
		map[string]string{"marker_auth": "new_hash"})

	evaluatedState := evaluateTodoActionExpectingSuccess(t, CoreAlgorithmContext{
		Specs:   specs,
		Markers: markers,
		Links:   links,
		Action:  TodoAction{Scan: scan},
	})
	assertTodoCount(t, evaluatedState, 1)
	todo, ok := findTodoByEdge(evaluatedState, "marker_auth", "spec_auth")
	if !ok {
		t.Fatalf("expected todo for marker_auth:spec_auth")
	}
	if todo.SpecFilepath != "/project/specs/auth.xml" {
		t.Fatalf("todo.SpecFilepath = %q, want %q", todo.SpecFilepath, "/project/specs/auth.xml")
	}
	if todo.SpecLineNumber != 42 {
		t.Fatalf("todo.SpecLineNumber = %d, want %d", todo.SpecLineNumber, 42)
	}
	if todo.MarkerFilepath != "/project/src/auth.go" {
		t.Fatalf("todo.MarkerFilepath = %q, want %q", todo.MarkerFilepath, "/project/src/auth.go")
	}
	if todo.MarkerLineNumber != 88 {
		t.Fatalf("todo.MarkerLineNumber = %d, want %d", todo.MarkerLineNumber, 88)
	}
	assertTodoDriftFlags(t, todo, true, true)
}

func TestEdgeDriftCombos(t *testing.T) {
	const (
		baselineSpecHash   = "bs"
		baselineMarkerHash = "bm"
		currentSpecHash    = "cs"
		currentMarkerHash   = "cm"
	)
	specs := []Spec{newSpec("s1", baselineSpecHash)}
	markers := []Marker{newMarker("m1", baselineMarkerHash)}
	links := []Link{newLink("s1", "m1")}

	type driftScenario struct {
		name           string
		specCurrent    string
		markerCurrent  string
		specChanged    bool
		markerChanged  bool
	}
	driftScenarios := []driftScenario{
		{"none_unchanged", baselineSpecHash, baselineMarkerHash, false, false},
		{"spec_changed", currentSpecHash, baselineMarkerHash, true, false},
		{"marker_changed", baselineSpecHash, currentMarkerHash, false, true},
		{"both_changed", currentSpecHash, currentMarkerHash, true, true},
	}

	for _, drift := range driftScenarios {
		anySideChanged := drift.specChanged || drift.markerChanged
		resolutionVariants := []struct {
			name       string
			res        []ResolutionState
			suppresses bool
		}{
			{"no_resolution", nil, false},
			{"resolution_matches_current", []ResolutionState{newResolutionState("s1", "m1", drift.specCurrent, drift.markerCurrent)}, true},
			{"resolution_stale", []ResolutionState{newResolutionState("s1", "m1", "OLD_SPEC", "OLD_MARKER")}, false},
		}
		for _, variant := range resolutionVariants {
			t.Run("todo/"+drift.name+"/"+variant.name, func(t *testing.T) {
				scan := newScanFromBaselines(specs, markers,
					map[string]string{"s1": drift.specCurrent},
					map[string]string{"m1": drift.markerCurrent})
				ctx := CoreAlgorithmContext{
					Specs:           specs,
					Markers:         markers,
					Links:           links,
					ResolutionState: variant.res,
					Action:          TodoAction{Scan: scan},
				}
				evaluatedState := evaluateTodoActionExpectingSuccess(t, ctx)

				expectTodo := anySideChanged && !variant.suppresses
				todo, ok := findTodoByEdge(evaluatedState, "m1", "s1")
				if expectTodo {
					if !ok {
						t.Fatalf("expected a todo, got none (todos=%v)", evaluatedState.Todos)
					}
					assertTodoDriftFlags(t, todo, drift.specChanged, drift.markerChanged)
				} else {
					if ok {
						t.Fatalf("expected no todo, got %v", todo)
					}
					assertTodoCount(t, evaluatedState, 0)
				}
			})
		}
	}

	for _, drift := range driftScenarios {
		t.Run("reset/"+drift.name, func(t *testing.T) {
			scan := newScanFromBaselines(specs, markers,
				map[string]string{"s1": drift.specCurrent},
				map[string]string{"m1": drift.markerCurrent})
			ctx := CoreAlgorithmContext{
				Specs:   specs,
				Markers: markers,
				Links:   links,
				Action:  ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scan},
			}
			evaluatedState := evaluateResetActionExpectingSuccess(t, ctx)
			assertBaselineHashes(t, evaluatedState, "s1", drift.specCurrent, "m1", drift.markerCurrent)
			assertResolutionStateCount(t, evaluatedState, 0)
		})
	}

	t.Run("reset_overwrites_stale_resolution", func(t *testing.T) {
		scan := newScanFromBaselines(specs, markers,
			map[string]string{"s1": currentSpecHash},
			map[string]string{"m1": currentMarkerHash})
		ctx := CoreAlgorithmContext{
			Specs:           specs,
			Markers:         markers,
			Links:           links,
			ResolutionState: []ResolutionState{newResolutionState("s1", "m1", "STALE_SPEC", "STALE_MARKER")},
			Action:          ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scan},
		}
		evaluatedState := evaluateResetActionExpectingSuccess(t, ctx)
		assertResolutionStateCount(t, evaluatedState, 0)
		assertBaselineHashes(t, evaluatedState, "s1", currentSpecHash, "m1", currentMarkerHash)
	})

	t.Run("redrift_after_resolution", func(t *testing.T) {
		scanAfterSpecChange := newScanFromBaselines(specs, markers,
			map[string]string{"s1": currentSpecHash}, nil)
		ctxAfterSpecChange := CoreAlgorithmContext{
			Specs:   specs,
			Markers: markers,
			Links:   links,
			Action:  TodoAction{Scan: scanAfterSpecChange},
		}
		evaluatedStateAfterSpecChange := evaluateTodoActionExpectingSuccess(t, ctxAfterSpecChange)
		assertTodoCount(t, evaluatedStateAfterSpecChange, 1)

		resolvedResolutionState := []ResolutionState{newResolutionState("s1", "m1", currentSpecHash, baselineMarkerHash)}
		ctxWithResolution := CoreAlgorithmContext{
			Specs:           specs,
			Markers:         markers,
			Links:           links,
			ResolutionState: resolvedResolutionState,
			Action:          TodoAction{Scan: scanAfterSpecChange},
		}
		evaluatedStateWithResolution := evaluateTodoActionExpectingSuccess(t, ctxWithResolution)
		assertTodoCount(t, evaluatedStateWithResolution, 0)

		scanAfterFurtherChange := newScanFromBaselines(specs, markers,
			map[string]string{"s1": "cs2"}, nil)
		ctxAfterFurtherChange := CoreAlgorithmContext{
			Specs:           specs,
			Markers:         markers,
			Links:           links,
			ResolutionState: resolvedResolutionState,
			Action:          TodoAction{Scan: scanAfterFurtherChange},
		}
		evaluatedStateAfterFurtherChange := evaluateTodoActionExpectingSuccess(t, ctxAfterFurtherChange)
		assertTodoCount(t, evaluatedStateAfterFurtherChange, 1)
		todo, _ := findTodoByEdge(evaluatedStateAfterFurtherChange, "m1", "s1")
		assertTodoDriftFlags(t, todo, true, false)
	})
}

func TestTopologyCollapse(t *testing.T) {
	t.Run("many_specs_1_marker_all_changed_progressive_collapse", func(t *testing.T) {
		specs := []Spec{newSpec("s1", "bs")}
		markers := []Marker{newMarker("m1", "bm1"), newMarker("m2", "bm2")}
		links := []Link{newLink("s1", "m1"), newLink("s1", "m2")}
		scan := newScanFromBaselines(specs, markers,
			map[string]string{"s1": "cs"},
			map[string]string{"m1": "cm1", "m2": "cm2"})

		evaluatedState := evaluateTodoActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs: specs, Markers: markers, Links: links, Action: TodoAction{Scan: scan}})
		assertTodoCount(t, evaluatedState, 2)

		evaluatedState = evaluateResetActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs: specs, Markers: markers, Links: links,
			Action: ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scan}})
		assertBaselineHashes(t, evaluatedState, "s1", "bs", "m1", "cm1")
		assertBaselineHashes(t, evaluatedState, "", "", "m2", "bm2")
		assertResolutionStateEntry(t, evaluatedState, "m1", "s1", "cs", "cm1")
		assertResolutionStateCount(t, evaluatedState, 1)

		evaluatedStateTodo := evaluateTodoActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs: evaluatedState.Specs, Markers: evaluatedState.Markers, Links: links,
			ResolutionState: evaluatedState.ResolutionState, Action: TodoAction{Scan: scan}})
		assertTodoCount(t, evaluatedStateTodo, 1)
		if _, ok := findTodoByEdge(evaluatedStateTodo, "m2", "s1"); !ok {
			t.Fatalf("expected remaining todo for m2:s1")
		}

		evaluatedState = evaluateResetActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs: evaluatedState.Specs, Markers: evaluatedState.Markers, Links: links,
			ResolutionState: evaluatedState.ResolutionState,
			Action:          ResetAction{SpecID: "s1", MarkerID: "m2", Scan: scan}})
		assertBaselineHashes(t, evaluatedState, "s1", "cs", "m1", "cm1")
		assertBaselineHashes(t, evaluatedState, "", "", "m2", "cm2")
		assertResolutionStateCount(t, evaluatedState, 0)
	})

	t.Run("1_spec_many_markers_all_changed_progressive_collapse", func(t *testing.T) {
		specs := []Spec{newSpec("s1", "bs1"), newSpec("s2", "bs2")}
		markers := []Marker{newMarker("m1", "bm")}
		links := []Link{newLink("s1", "m1"), newLink("s2", "m1")}
		scan := newScanFromBaselines(specs, markers,
			map[string]string{"s1": "cs1", "s2": "cs2"},
			map[string]string{"m1": "cm"})

		evaluatedState := evaluateTodoActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs: specs, Markers: markers, Links: links, Action: TodoAction{Scan: scan}})
		assertTodoCount(t, evaluatedState, 2)

		evaluatedState = evaluateResetActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs: specs, Markers: markers, Links: links,
			Action: ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scan}})
		assertBaselineHashes(t, evaluatedState, "s1", "cs1", "m1", "bm")
		assertBaselineHashes(t, evaluatedState, "s2", "bs2", "", "")
		assertResolutionStateEntry(t, evaluatedState, "m1", "s1", "cs1", "cm")
		assertResolutionStateCount(t, evaluatedState, 1)

		evaluatedState = evaluateResetActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs: evaluatedState.Specs, Markers: evaluatedState.Markers, Links: links,
			ResolutionState: evaluatedState.ResolutionState,
			Action:           ResetAction{SpecID: "s2", MarkerID: "m1", Scan: scan}})
		assertBaselineHashes(t, evaluatedState, "s1", "cs1", "m1", "cm")
		assertBaselineHashes(t, evaluatedState, "s2", "cs2", "", "")
		assertResolutionStateCount(t, evaluatedState, 0)
	})

	t.Run("many_specs_many_markers_3x3_docs_regression", func(t *testing.T) {
		specIDs := []string{"validate_amount", "check_fraud_rules", "log_transaction"}
		markerIDs := []string{"a1", "e5", "f9"}
		specs := []Spec{}
		for _, id := range specIDs {
			specs = append(specs, newSpec(id, "old_"+id))
		}
		markers := []Marker{}
		for _, id := range markerIDs {
			markers = append(markers, newMarker(id, "old_"+id))
		}
		links := []Link{}
		for _, markerID := range markerIDs {
			for _, specID := range specIDs {
				links = append(links, newLink(specID, markerID))
			}
		}
		specHashOverrides := map[string]string{}
		for _, id := range specIDs {
			specHashOverrides[id] = "new_" + id
		}
		markerHashOverrides := map[string]string{}
		for _, id := range markerIDs {
			markerHashOverrides[id] = "new_" + id
		}
		scan := newScanFromBaselines(specs, markers, specHashOverrides, markerHashOverrides)

		evaluatedState := evaluateTodoActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs: specs, Markers: markers, Links: links, Action: TodoAction{Scan: scan}})
		assertTodoCount(t, evaluatedState, 9)

		currentContext := CoreAlgorithmContext{Specs: specs, Markers: markers, Links: links}
		currentContext.Action = ResetAction{SpecID: "validate_amount", MarkerID: "a1", Scan: scan}
		evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
		currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

		currentContext.Action = ResetAction{SpecID: "check_fraud_rules", MarkerID: "e5", Scan: scan}
		evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
		currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

		assertResolutionStateCount(t, evaluatedState, 2)
		assertResolutionStateEntry(t, evaluatedState, "a1", "validate_amount", "new_validate_amount", "new_a1")
		assertResolutionStateEntry(t, evaluatedState, "e5", "check_fraud_rules", "new_check_fraud_rules", "new_e5")
		for _, specID := range specIDs {
			assertBaselineHashes(t, evaluatedState, specID, "old_"+specID, "", "")
		}
		for _, markerID := range markerIDs {
			assertBaselineHashes(t, evaluatedState, "", "", markerID, "old_"+markerID)
		}
		evaluatedStateTodo := evaluateTodoActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs: evaluatedState.Specs, Markers: evaluatedState.Markers, Links: links,
			ResolutionState: evaluatedState.ResolutionState, Action: TodoAction{Scan: scan}})
		assertTodoCount(t, evaluatedStateTodo, 7)

		for _, link := range links {
			currentContext.Action = ResetAction{SpecID: link.SpecID, MarkerID: link.MarkerID, Scan: scan}
			evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
			currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState
		}
		assertResolutionStateCount(t, evaluatedState, 0)
		for _, specID := range specIDs {
			assertBaselineHashes(t, evaluatedState, specID, "new_"+specID, "", "")
		}
		for _, markerID := range markerIDs {
			assertBaselineHashes(t, evaluatedState, "", "", markerID, "new_"+markerID)
		}
	})

	t.Run("partial_collapse_marker_only", func(t *testing.T) {
		specs := []Spec{newSpec("s1", "bs1"), newSpec("s2", "bs2")}
		markers := []Marker{newMarker("m1", "bm1"), newMarker("m2", "bm2")}
		links := []Link{newLink("s1", "m1"), newLink("s2", "m1"), newLink("s1", "m2"), newLink("s2", "m2")}
		scan := newScanFromBaselines(specs, markers,
			map[string]string{"s1": "cs1", "s2": "cs2"},
			map[string]string{"m1": "cm1", "m2": "cm2"})

		currentContext := CoreAlgorithmContext{Specs: specs, Markers: markers, Links: links}
		currentContext.Action = ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scan}
		evaluatedState := evaluateResetActionExpectingSuccess(t, currentContext)
		currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

		currentContext.Action = ResetAction{SpecID: "s2", MarkerID: "m1", Scan: scan}
		evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
		currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

		assertBaselineHashes(t, evaluatedState, "", "", "m1", "cm1")
		assertBaselineHashes(t, evaluatedState, "", "", "m2", "bm2")
		assertBaselineHashes(t, evaluatedState, "s1", "bs1", "", "")
		assertBaselineHashes(t, evaluatedState, "s2", "bs2", "", "")
		assertResolutionStateCount(t, evaluatedState, 2)
		assertResolutionStateEntry(t, evaluatedState, "m1", "s1", "cs1", "cm1")
		assertResolutionStateEntry(t, evaluatedState, "m1", "s2", "cs2", "cm1")

		evaluatedStateTodo := evaluateTodoActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs: evaluatedState.Specs, Markers: evaluatedState.Markers, Links: links,
			ResolutionState: evaluatedState.ResolutionState, Action: TodoAction{Scan: scan}})
		assertTodoCount(t, evaluatedStateTodo, 2)
		if _, ok := findTodoByEdge(evaluatedStateTodo, "m2", "s1"); !ok {
			t.Fatalf("expected todo for m2:s1")
		}
		if _, ok := findTodoByEdge(evaluatedStateTodo, "m2", "s2"); !ok {
			t.Fatalf("expected todo for m2:s2")
		}

		currentContext.Action = ResetAction{SpecID: "s1", MarkerID: "m2", Scan: scan}
		evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
		currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState
		currentContext.Action = ResetAction{SpecID: "s2", MarkerID: "m2", Scan: scan}
		evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)

		assertResolutionStateCount(t, evaluatedState, 0)
		assertBaselineHashes(t, evaluatedState, "s1", "cs1", "m1", "cm1")
		assertBaselineHashes(t, evaluatedState, "s2", "cs2", "m2", "cm2")
	})

	t.Run("reset_idempotent_when_unchanged", func(t *testing.T) {
		specs := []Spec{newSpec("s1", "bs")}
		markers := []Marker{newMarker("m1", "bm")}
		links := []Link{newLink("s1", "m1")}
		scan := newBaselineScan(specs, markers)

		evaluatedState := evaluateResetActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs: specs, Markers: markers, Links: links,
			Action: ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scan}})
		assertResolutionStateCount(t, evaluatedState, 0)
		assertBaselineHashes(t, evaluatedState, "s1", "bs", "m1", "bm")
	})
}

func TestRedriftAfterCollapse(t *testing.T) {
	specs := []Spec{newSpec("s1", "bs1"), newSpec("s2", "bs2")}
	markers := []Marker{newMarker("m1", "bm1"), newMarker("m2", "bm2")}
	links := []Link{newLink("s1", "m1"), newLink("s1", "m2"), newLink("s2", "m1"), newLink("s2", "m2")}
	scanAllChanged := newScanFromBaselines(specs, markers,
		map[string]string{"s1": "cs1", "s2": "cs2"},
		map[string]string{"m1": "cm1", "m2": "cm2"})

	currentContext := CoreAlgorithmContext{Specs: specs, Markers: markers, Links: links}
	currentContext.Action = ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scanAllChanged}
	evaluatedState := evaluateResetActionExpectingSuccess(t, currentContext)
	currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

	currentContext.Action = ResetAction{SpecID: "s2", MarkerID: "m1", Scan: scanAllChanged}
	evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
	currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

	assertBaselineHashes(t, evaluatedState, "", "", "m1", "cm1")
	assertResolutionStateCount(t, evaluatedState, 2)

	currentContext.Action = ResetAction{SpecID: "s1", MarkerID: "m2", Scan: scanAllChanged}
	evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
	currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

	currentContext.Action = ResetAction{SpecID: "s2", MarkerID: "m2", Scan: scanAllChanged}
	evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
	currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

	assertResolutionStateCount(t, evaluatedState, 0)
	assertBaselineHashes(t, evaluatedState, "s1", "cs1", "m1", "cm1")
	assertBaselineHashes(t, evaluatedState, "s2", "cs2", "m2", "cm2")

	scanAfterMarkerM1Rechanges := newScanFromBaselines(evaluatedState.Specs, evaluatedState.Markers,
		nil,
		map[string]string{"m1": "cm1_v2"})

	evaluatedStateTodo := evaluateTodoActionExpectingSuccess(t, CoreAlgorithmContext{
		Specs: evaluatedState.Specs, Markers: evaluatedState.Markers, Links: links,
		ResolutionState: evaluatedState.ResolutionState,
		Action:          TodoAction{Scan: scanAfterMarkerM1Rechanges}})
	assertTodoCount(t, evaluatedStateTodo, 2)
	if _, ok := findTodoByEdge(evaluatedStateTodo, "m1", "s1"); !ok {
		t.Fatalf("expected re-drifted todo for m1:s1 after post-collapse marker change")
	}
	if _, ok := findTodoByEdge(evaluatedStateTodo, "m1", "s2"); !ok {
		t.Fatalf("expected re-drifted todo for m1:s2 after post-collapse marker change")
	}
	if _, ok := findTodoByEdge(evaluatedStateTodo, "m2", "s1"); ok {
		t.Fatalf("did not expect todo for unchanged m2:s1")
	}
}

func TestMixedChangedAndUnchangedEdgesCollapseImmediately(t *testing.T) {
	specs := []Spec{newSpec("s1", "baseline_s1"), newSpec("s2", "baseline_s2")}
	markers := []Marker{newMarker("m1", "baseline_m1"), newMarker("m2", "baseline_m2")}
	links := []Link{newLink("s1", "m1"), newLink("s1", "m2"), newLink("s2", "m1"), newLink("s2", "m2")}
	scan := newScanFromBaselines(specs, markers,
		map[string]string{"s1": "current_s1"},
		map[string]string{"m1": "current_m1"})

	t.Run("initial_todos_count_only_drifted_edges", func(t *testing.T) {
		evaluatedState := evaluateTodoActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs: specs, Markers: markers, Links: links, Action: TodoAction{Scan: scan}})
		assertTodoCount(t, evaluatedState, 3)
		for _, edge := range []struct{ markerID, specID string }{{"m1", "s1"}, {"m1", "s2"}, {"m2", "s1"}} {
			if _, ok := findTodoByEdge(evaluatedState, edge.markerID, edge.specID); !ok {
				t.Fatalf("expected todo for drifted edge %s:%s", edge.markerID, edge.specID)
			}
		}
		if _, ok := findTodoByEdge(evaluatedState, "m2", "s2"); ok {
			t.Fatalf("did not expect todo for consistent edge m2:s2")
		}
	})

	t.Run("unchanged_node_collapses_via_consistent_edge_branch", func(t *testing.T) {
		currentContext := CoreAlgorithmContext{Specs: specs, Markers: markers, Links: links}
		currentContext.Action = ResetAction{SpecID: "s2", MarkerID: "m1", Scan: scan}
		evaluatedState := evaluateResetActionExpectingSuccess(t, currentContext)

		assertBaselineHashes(t, evaluatedState, "s2", "baseline_s2", "m2", "baseline_m2")
		assertBaselineHashes(t, evaluatedState, "s1", "baseline_s1", "m1", "baseline_m1")
		assertResolutionStateEntry(t, evaluatedState, "m1", "s2", "baseline_s2", "current_m1")
		assertResolutionStateCount(t, evaluatedState, 1)

		currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState
		currentContext.Action = ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scan}
		evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)

		assertBaselineHashes(t, evaluatedState, "s1", "baseline_s1", "m1", "current_m1")
		assertBaselineHashes(t, evaluatedState, "s2", "baseline_s2", "m2", "baseline_m2")
		assertResolutionStateEntry(t, evaluatedState, "m1", "s1", "current_s1", "current_m1")
		assertResolutionStateCount(t, evaluatedState, 1)

		currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState
		currentContext.Action = ResetAction{SpecID: "s1", MarkerID: "m2", Scan: scan}
		evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)

		assertResolutionStateCount(t, evaluatedState, 0)
		assertBaselineHashes(t, evaluatedState, "s1", "current_s1", "m1", "current_m1")
		assertBaselineHashes(t, evaluatedState, "s2", "baseline_s2", "m2", "baseline_m2")

		evaluatedStateTodo := evaluateTodoActionExpectingSuccess(t, CoreAlgorithmContext{
			Specs: evaluatedState.Specs, Markers: evaluatedState.Markers, Links: links,
			ResolutionState: evaluatedState.ResolutionState, Action: TodoAction{Scan: scan}})
		assertTodoCount(t, evaluatedStateTodo, 0)
	})
}

func TestValidate(t *testing.T) {
	baseSpecs := []Spec{newSpec("s1", "b1"), newSpec("s2", "b2")}
	baseMarkers := []Marker{newMarker("m1", "b1"), newMarker("m2", "b2")}
	baseLinks := []Link{newLink("s1", "m1"), newLink("s2", "m2")}

	cases := []struct {
		name    string
		ctx     CoreAlgorithmContext
		viaEval bool
		target  error
	}{
		{
			name:   "duplicate_spec_id",
			ctx:    CoreAlgorithmContext{Specs: []Spec{newSpec("s1", "a"), newSpec("s1", "b")}},
			target: ErrDuplicateSpecID,
		},
		{
			name:   "duplicate_marker_id",
			ctx:    CoreAlgorithmContext{Markers: []Marker{newMarker("m1", "a"), newMarker("m1", "b")}},
			target: ErrDuplicateMarkerID,
		},
		{
			name:   "duplicate_link",
			ctx:    CoreAlgorithmContext{Specs: baseSpecs, Markers: baseMarkers, Links: []Link{newLink("s1", "m1"), newLink("s1", "m1")}},
			target: ErrDuplicateLink,
		},
		{
			name:   "link_unknown_spec",
			ctx:    CoreAlgorithmContext{Markers: baseMarkers, Links: []Link{newLink("sX", "m1")}},
			target: ErrLinkUnknownSpec,
		},
		{
			name:   "link_unknown_marker",
			ctx:    CoreAlgorithmContext{Specs: baseSpecs, Links: []Link{newLink("s1", "mX")}},
			target: ErrLinkUnknownMarker,
		},
		{
			name:   "resolution_unknown_spec",
			ctx:    CoreAlgorithmContext{Specs: baseSpecs, Markers: baseMarkers, Links: baseLinks, ResolutionState: []ResolutionState{newResolutionState("sX", "m1", "x", "y")}},
			target: ErrResolutionUnknownSpec,
		},
		{
			name:   "resolution_unknown_marker",
			ctx:    CoreAlgorithmContext{Specs: baseSpecs, Markers: baseMarkers, Links: baseLinks, ResolutionState: []ResolutionState{newResolutionState("s1", "mX", "x", "y")}},
			target: ErrResolutionUnknownMarker,
		},
		{
			name: "resolution_edge_not_linked",
			ctx: CoreAlgorithmContext{
				Specs:           []Spec{newSpec("s1", "b"), newSpec("s2", "b")},
				Markers:         []Marker{newMarker("m1", "b"), newMarker("m2", "b")},
				Links:           []Link{newLink("s1", "m1")},
				ResolutionState: []ResolutionState{newResolutionState("s2", "m2", "x", "y")},
			},
			target: ErrResolutionEdgeNotLinked,
		},
		{
			name: "duplicate_resolution",
			ctx: CoreAlgorithmContext{
				Specs:           baseSpecs,
				Markers:         baseMarkers,
				Links:           baseLinks,
				ResolutionState: []ResolutionState{newResolutionState("s1", "m1", "x", "y"), newResolutionState("s1", "m1", "z", "w")},
			},
			target: ErrDuplicateResolution,
		},
		{
			name: "scan_missing_spec_hash",
			ctx: CoreAlgorithmContext{
				Specs:   baseSpecs,
				Markers: baseMarkers,
				Links:   baseLinks,
				Action:  TodoAction{Scan: Scan{SpecHashes: map[string]string{"s1": "b"}, MarkerHashes: map[string]string{"m1": "b", "m2": "b"}}},
			},
			viaEval: true,
			target:  ErrScanMissingSpecHash,
		},
		{
			name: "scan_missing_marker_hash",
			ctx: CoreAlgorithmContext{
				Specs:   baseSpecs,
				Markers: baseMarkers,
				Links:   baseLinks,
				Action:  TodoAction{Scan: Scan{SpecHashes: map[string]string{"s1": "b", "s2": "b"}, MarkerHashes: map[string]string{"m1": "b"}}},
			},
			viaEval: true,
			target:  ErrScanMissingMarkerHash,
		},
		{
			name: "scan_unknown_spec_hash",
			ctx: CoreAlgorithmContext{
				Specs:   baseSpecs,
				Markers: baseMarkers,
				Links:   baseLinks,
				Action:  TodoAction{Scan: Scan{SpecHashes: map[string]string{"s1": "b", "s2": "b", "sX": "b"}, MarkerHashes: map[string]string{"m1": "b", "m2": "b"}}},
			},
			viaEval: true,
			target:  ErrScanUnknownSpecHash,
		},
		{
			name: "scan_unknown_marker_hash",
			ctx: CoreAlgorithmContext{
				Specs:   baseSpecs,
				Markers: baseMarkers,
				Links:   baseLinks,
				Action:  TodoAction{Scan: Scan{SpecHashes: map[string]string{"s1": "b", "s2": "b"}, MarkerHashes: map[string]string{"m1": "b", "m2": "b", "mX": "b"}}},
			},
			viaEval: true,
			target:  ErrScanUnknownMarkerHash,
		},
		{
			name: "unknown_action",
			ctx: CoreAlgorithmContext{
				Specs:   baseSpecs,
				Markers: baseMarkers,
				Links:   baseLinks,
				Action:  nil,
			},
			viaEval: true,
			target:  ErrUnknownAction,
		},
		{
			name: "reset_edge_not_linked",
			ctx: CoreAlgorithmContext{
				Specs:   baseSpecs,
				Markers: baseMarkers,
				Links:   baseLinks,
				Action:  ResetAction{SpecID: "s1", MarkerID: "m2", Scan: newBaselineScan(baseSpecs, baseMarkers)},
			},
			viaEval: true,
			target:  ErrResetEdgeNotLinked,
		},
		{
			name: "reset_scan_missing_spec_hash",
			ctx: CoreAlgorithmContext{
				Specs:   baseSpecs,
				Markers: baseMarkers,
				Links:   baseLinks,
				Action: ResetAction{
					SpecID: "s1", MarkerID: "m1",
					Scan: Scan{SpecHashes: map[string]string{"s2": "b"}, MarkerHashes: map[string]string{"m1": "b", "m2": "b"}},
				},
			},
			viaEval: true,
			target:  ErrScanMissingSpecHash,
		},
	}

	for _, validateCase := range cases {
		t.Run(validateCase.name, func(t *testing.T) {
			var err error
			if validateCase.viaEval {
				_, err = NewCoreAlgorithm().EvaluateState(validateCase.ctx)
			} else {
				err = validateCase.ctx.Validate()
			}
			assertErrorWraps(t, err, validateCase.target)
		})
	}

	t.Run("valid_context_passes_validate", func(t *testing.T) {
		ctx := CoreAlgorithmContext{
			Specs:           baseSpecs,
			Markers:         baseMarkers,
			Links:           baseLinks,
			ResolutionState: []ResolutionState{newResolutionState("s1", "m1", "x", "y")},
		}
		assertNoError(t, ctx.Validate())
	})

	t.Run("empty_context_passes_validate", func(t *testing.T) {
		assertNoError(t, CoreAlgorithmContext{}.Validate())
	})
}
