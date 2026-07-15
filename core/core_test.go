package core_test

import (
	"testing"

	"driftpin/core"
	"driftpin/internal/testutil"
)

func newScanFromBaselines(specs []core.Spec, markers []core.Marker, specHashOverrides map[string]string, markerHashOverrides map[string]string) core.Scan {
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
	return core.Scan{SpecHashes: specHashes, MarkerHashes: markerHashes}
}

func newBaselineScan(specs []core.Spec, markers []core.Marker) core.Scan {
	return newScanFromBaselines(specs, markers, nil, nil)
}

func findSpecByID(t *testing.T, evaluatedState core.EvaluatedState, specID string) core.Spec {
	t.Helper()
	for _, spec := range evaluatedState.Specs {
		if spec.ID == specID {
			return spec
		}
	}
	t.Fatalf("spec %q not found in evaluated state", specID)
	return core.Spec{}
}

func findMarkerByID(t *testing.T, evaluatedState core.EvaluatedState, markerID string) core.Marker {
	t.Helper()
	for _, marker := range evaluatedState.Markers {
		if marker.ID == markerID {
			return marker
		}
	}
	t.Fatalf("marker %q not found in evaluated state", markerID)
	return core.Marker{}
}

func findResolutionStateByEdge(evaluatedState core.EvaluatedState, markerID string, specID string) (core.ResolutionState, bool) {
	for _, res := range evaluatedState.ResolutionState {
		if res.MarkerID == markerID && res.SpecID == specID {
			return res, true
		}
	}
	return core.ResolutionState{}, false
}

func findTodoByEdge(evaluatedState core.EvaluatedState, markerID string, specID string) (core.Todo, bool) {
	for _, todo := range evaluatedState.Todos {
		if todo.MarkerID == markerID && todo.SpecID == specID {
			return todo, true
		}
	}
	return core.Todo{}, false
}

func evaluateTodoActionExpectingSuccess(t *testing.T, ctx core.CoreAlgorithmContext) core.EvaluatedState {
	t.Helper()
	evaluatedState, err := core.NewCoreAlgorithm().EvaluateState(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return evaluatedState
}

func evaluateResetActionExpectingSuccess(t *testing.T, ctx core.CoreAlgorithmContext) core.EvaluatedState {
	t.Helper()
	evaluatedState, err := core.NewCoreAlgorithm().EvaluateState(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return evaluatedState
}

func TestArityShapes(t *testing.T) {
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
		t.Run(shape.name, func(t *testing.T) {
			baselineScan := newBaselineScan(shape.specs, shape.markers)
			todoCtx := core.CoreAlgorithmContext{
				Specs:   shape.specs,
				Markers: shape.markers,
				Links:   shape.links,
				Action:  core.TodoAction{Scan: baselineScan},
			}
			evaluatedState := evaluateTodoActionExpectingSuccess(t, todoCtx)
			testutil.AssertTodoCount(t, evaluatedState, 0)
			testutil.AssertResolutionStateCount(t, evaluatedState, 0)

			resetCtx := core.CoreAlgorithmContext{
				Specs:   shape.specs,
				Markers: shape.markers,
				Links:   shape.links,
				Action:  core.ResetAction{SpecID: "nonexistent_spec", MarkerID: "nonexistent_marker", Scan: baselineScan},
			}
			_, err := core.NewCoreAlgorithm().EvaluateState(resetCtx)
			if err == nil {
				t.Fatalf("expected reset on nonexistent edge to error, got nil")
			}
		})
	}
}

func TestIsolatedNodeWithDriftStillReportsZeroTodos(t *testing.T) {
	specs := []core.Spec{testutil.NewSpec("s1", "baseline_hash")}
	markers := []core.Marker{testutil.NewMarker("m1", "baseline_hash")}
	driftedScan := newScanFromBaselines(specs, markers,
		map[string]string{"s1": "changed_hash"},
		map[string]string{"m1": "changed_hash"})

	t.Run("isolated_drifted_spec", func(t *testing.T) {
		evaluatedState := evaluateTodoActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs:   specs,
			Markers: nil,
			Links:   nil,
			Action:  core.TodoAction{Scan: core.Scan{SpecHashes: driftedScan.SpecHashes, MarkerHashes: map[string]string{}}},
		})
		testutil.AssertTodoCount(t, evaluatedState, 0)
	})
	t.Run("isolated_drifted_marker", func(t *testing.T) {
		evaluatedState := evaluateTodoActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs:   nil,
			Markers: markers,
			Links:   nil,
			Action:  core.TodoAction{Scan: core.Scan{SpecHashes: map[string]string{}, MarkerHashes: driftedScan.MarkerHashes}},
		})
		testutil.AssertTodoCount(t, evaluatedState, 0)
	})
	t.Run("isolated_drifted_spec_and_marker_no_link", func(t *testing.T) {
		evaluatedState := evaluateTodoActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs:   specs,
			Markers: markers,
			Links:   nil,
			Action:  core.TodoAction{Scan: driftedScan},
		})
		testutil.AssertTodoCount(t, evaluatedState, 0)
	})
}

func TestTodoFieldsRoundTrip(t *testing.T) {
	specs := []core.Spec{testutil.NewSpecWithLocation("spec_auth", "old_hash", "/project/specs/auth.xml", 42)}
	markers := []core.Marker{testutil.NewMarkerWithLocation("marker_auth", "old_hash", "/project/src/auth.go", 88)}
	links := []core.Link{testutil.NewLink("spec_auth", "marker_auth")}
	scan := newScanFromBaselines(specs, markers,
		map[string]string{"spec_auth": "new_hash"},
		map[string]string{"marker_auth": "new_hash"})

	evaluatedState := evaluateTodoActionExpectingSuccess(t, core.CoreAlgorithmContext{
		Specs:   specs,
		Markers: markers,
		Links:   links,
		Action:  core.TodoAction{Scan: scan},
	})
	testutil.AssertTodoCount(t, evaluatedState, 1)
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
	testutil.AssertTodoDriftFlags(t, todo, true, true)
}

func TestEdgeDriftCombos(t *testing.T) {
	const (
		baselineSpecHash   = "bs"
		baselineMarkerHash = "bm"
		currentSpecHash    = "cs"
		currentMarkerHash  = "cm"
	)
	specs := []core.Spec{testutil.NewSpec("s1", baselineSpecHash)}
	markers := []core.Marker{testutil.NewMarker("m1", baselineMarkerHash)}
	links := []core.Link{testutil.NewLink("s1", "m1")}

	type driftScenario struct {
		name          string
		specCurrent   string
		markerCurrent string
		specChanged   bool
		markerChanged bool
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
			res        []core.ResolutionState
			suppresses bool
		}{
			{"no_resolution", nil, false},
			{"resolution_matches_current", []core.ResolutionState{testutil.NewResolutionState("s1", "m1", drift.specCurrent, drift.markerCurrent)}, true},
			{"resolution_stale", []core.ResolutionState{testutil.NewResolutionState("s1", "m1", "OLD_SPEC", "OLD_MARKER")}, false},
		}
		for _, variant := range resolutionVariants {
			t.Run("todo/"+drift.name+"/"+variant.name, func(t *testing.T) {
				scan := newScanFromBaselines(specs, markers,
					map[string]string{"s1": drift.specCurrent},
					map[string]string{"m1": drift.markerCurrent})
				ctx := core.CoreAlgorithmContext{
					Specs:           specs,
					Markers:         markers,
					Links:           links,
					ResolutionState: variant.res,
					Action:          core.TodoAction{Scan: scan},
				}
				evaluatedState := evaluateTodoActionExpectingSuccess(t, ctx)

				expectTodo := anySideChanged && !variant.suppresses
				todo, ok := findTodoByEdge(evaluatedState, "m1", "s1")
				if expectTodo {
					if !ok {
						t.Fatalf("expected a todo, got none (todos=%v)", evaluatedState.Todos)
					}
					testutil.AssertTodoDriftFlags(t, todo, drift.specChanged, drift.markerChanged)
				} else {
					if ok {
						t.Fatalf("expected no todo, got %v", todo)
					}
					testutil.AssertTodoCount(t, evaluatedState, 0)
				}
			})
		}
	}

	for _, drift := range driftScenarios {
		t.Run("reset/"+drift.name, func(t *testing.T) {
			scan := newScanFromBaselines(specs, markers,
				map[string]string{"s1": drift.specCurrent},
				map[string]string{"m1": drift.markerCurrent})
			ctx := core.CoreAlgorithmContext{
				Specs:   specs,
				Markers: markers,
				Links:   links,
				Action:  core.ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scan},
			}
			evaluatedState := evaluateResetActionExpectingSuccess(t, ctx)
			testutil.AssertBaselineHashes(t, evaluatedState, "s1", drift.specCurrent, "m1", drift.markerCurrent)
			testutil.AssertResolutionStateCount(t, evaluatedState, 0)
		})
	}

	t.Run("reset_overwrites_stale_resolution", func(t *testing.T) {
		scan := newScanFromBaselines(specs, markers,
			map[string]string{"s1": currentSpecHash},
			map[string]string{"m1": currentMarkerHash})
		ctx := core.CoreAlgorithmContext{
			Specs:           specs,
			Markers:         markers,
			Links:           links,
			ResolutionState: []core.ResolutionState{testutil.NewResolutionState("s1", "m1", "STALE_SPEC", "STALE_MARKER")},
			Action:          core.ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scan},
		}
		evaluatedState := evaluateResetActionExpectingSuccess(t, ctx)
		testutil.AssertResolutionStateCount(t, evaluatedState, 0)
		testutil.AssertBaselineHashes(t, evaluatedState, "s1", currentSpecHash, "m1", currentMarkerHash)
	})

	t.Run("redrift_after_resolution", func(t *testing.T) {
		scanAfterSpecChange := newScanFromBaselines(specs, markers,
			map[string]string{"s1": currentSpecHash}, nil)
		ctxAfterSpecChange := core.CoreAlgorithmContext{
			Specs:   specs,
			Markers: markers,
			Links:   links,
			Action:  core.TodoAction{Scan: scanAfterSpecChange},
		}
		evaluatedStateAfterSpecChange := evaluateTodoActionExpectingSuccess(t, ctxAfterSpecChange)
		testutil.AssertTodoCount(t, evaluatedStateAfterSpecChange, 1)

		resolvedResolutionState := []core.ResolutionState{testutil.NewResolutionState("s1", "m1", currentSpecHash, baselineMarkerHash)}
		ctxWithResolution := core.CoreAlgorithmContext{
			Specs:           specs,
			Markers:         markers,
			Links:           links,
			ResolutionState: resolvedResolutionState,
			Action:          core.TodoAction{Scan: scanAfterSpecChange},
		}
		evaluatedStateWithResolution := evaluateTodoActionExpectingSuccess(t, ctxWithResolution)
		testutil.AssertTodoCount(t, evaluatedStateWithResolution, 0)

		scanAfterFurtherChange := newScanFromBaselines(specs, markers,
			map[string]string{"s1": "cs2"}, nil)
		ctxAfterFurtherChange := core.CoreAlgorithmContext{
			Specs:           specs,
			Markers:         markers,
			Links:           links,
			ResolutionState: resolvedResolutionState,
			Action:          core.TodoAction{Scan: scanAfterFurtherChange},
		}
		evaluatedStateAfterFurtherChange := evaluateTodoActionExpectingSuccess(t, ctxAfterFurtherChange)
		testutil.AssertTodoCount(t, evaluatedStateAfterFurtherChange, 1)
		todo, _ := findTodoByEdge(evaluatedStateAfterFurtherChange, "m1", "s1")
		testutil.AssertTodoDriftFlags(t, todo, true, false)
	})
}

func TestTopologyCollapse(t *testing.T) {
	t.Run("many_specs_1_marker_all_changed_progressive_collapse", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "bs")}
		markers := []core.Marker{testutil.NewMarker("m1", "bm1"), testutil.NewMarker("m2", "bm2")}
		links := []core.Link{testutil.NewLink("s1", "m1"), testutil.NewLink("s1", "m2")}
		scan := newScanFromBaselines(specs, markers,
			map[string]string{"s1": "cs"},
			map[string]string{"m1": "cm1", "m2": "cm2"})

		evaluatedState := evaluateTodoActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs: specs, Markers: markers, Links: links, Action: core.TodoAction{Scan: scan}})
		testutil.AssertTodoCount(t, evaluatedState, 2)

		evaluatedState = evaluateResetActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs: specs, Markers: markers, Links: links,
			Action: core.ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scan}})
		testutil.AssertBaselineHashes(t, evaluatedState, "s1", "bs", "m1", "cm1")
		testutil.AssertBaselineHashes(t, evaluatedState, "", "", "m2", "bm2")
		testutil.AssertResolutionStateEntry(t, evaluatedState, "m1", "s1", "cs", "cm1")
		testutil.AssertResolutionStateCount(t, evaluatedState, 1)

		evaluatedStateTodo := evaluateTodoActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs: evaluatedState.Specs, Markers: evaluatedState.Markers, Links: links,
			ResolutionState: evaluatedState.ResolutionState, Action: core.TodoAction{Scan: scan}})
		testutil.AssertTodoCount(t, evaluatedStateTodo, 1)
		if _, ok := findTodoByEdge(evaluatedStateTodo, "m2", "s1"); !ok {
			t.Fatalf("expected remaining todo for m2:s1")
		}

		evaluatedState = evaluateResetActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs: evaluatedState.Specs, Markers: evaluatedState.Markers, Links: links,
			ResolutionState: evaluatedState.ResolutionState,
			Action:          core.ResetAction{SpecID: "s1", MarkerID: "m2", Scan: scan}})
		testutil.AssertBaselineHashes(t, evaluatedState, "s1", "cs", "m1", "cm1")
		testutil.AssertBaselineHashes(t, evaluatedState, "", "", "m2", "cm2")
		testutil.AssertResolutionStateCount(t, evaluatedState, 0)
	})

	t.Run("1_spec_many_markers_all_changed_progressive_collapse", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "bs1"), testutil.NewSpec("s2", "bs2")}
		markers := []core.Marker{testutil.NewMarker("m1", "bm")}
		links := []core.Link{testutil.NewLink("s1", "m1"), testutil.NewLink("s2", "m1")}
		scan := newScanFromBaselines(specs, markers,
			map[string]string{"s1": "cs1", "s2": "cs2"},
			map[string]string{"m1": "cm"})

		evaluatedState := evaluateTodoActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs: specs, Markers: markers, Links: links, Action: core.TodoAction{Scan: scan}})
		testutil.AssertTodoCount(t, evaluatedState, 2)

		evaluatedState = evaluateResetActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs: specs, Markers: markers, Links: links,
			Action: core.ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scan}})
		testutil.AssertBaselineHashes(t, evaluatedState, "s1", "cs1", "m1", "bm")
		testutil.AssertBaselineHashes(t, evaluatedState, "s2", "bs2", "", "")
		testutil.AssertResolutionStateEntry(t, evaluatedState, "m1", "s1", "cs1", "cm")
		testutil.AssertResolutionStateCount(t, evaluatedState, 1)

		evaluatedState = evaluateResetActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs: evaluatedState.Specs, Markers: evaluatedState.Markers, Links: links,
			ResolutionState: evaluatedState.ResolutionState,
			Action:          core.ResetAction{SpecID: "s2", MarkerID: "m1", Scan: scan}})
		testutil.AssertBaselineHashes(t, evaluatedState, "s1", "cs1", "m1", "cm")
		testutil.AssertBaselineHashes(t, evaluatedState, "s2", "cs2", "", "")
		testutil.AssertResolutionStateCount(t, evaluatedState, 0)
	})

	t.Run("many_specs_many_markers_3x3_docs_regression", func(t *testing.T) {
		specIDs := []string{"validate_amount", "check_fraud_rules", "log_transaction"}
		markerIDs := []string{"a1", "e5", "f9"}
		specs := []core.Spec{}
		for _, id := range specIDs {
			specs = append(specs, testutil.NewSpec(id, "old_"+id))
		}
		markers := []core.Marker{}
		for _, id := range markerIDs {
			markers = append(markers, testutil.NewMarker(id, "old_"+id))
		}
		links := []core.Link{}
		for _, markerID := range markerIDs {
			for _, specID := range specIDs {
				links = append(links, testutil.NewLink(specID, markerID))
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

		evaluatedState := evaluateTodoActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs: specs, Markers: markers, Links: links, Action: core.TodoAction{Scan: scan}})
		testutil.AssertTodoCount(t, evaluatedState, 9)

		currentContext := core.CoreAlgorithmContext{Specs: specs, Markers: markers, Links: links}
		currentContext.Action = core.ResetAction{SpecID: "validate_amount", MarkerID: "a1", Scan: scan}
		evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
		currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

		currentContext.Action = core.ResetAction{SpecID: "check_fraud_rules", MarkerID: "e5", Scan: scan}
		evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
		currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

		testutil.AssertResolutionStateCount(t, evaluatedState, 2)
		testutil.AssertResolutionStateEntry(t, evaluatedState, "a1", "validate_amount", "new_validate_amount", "new_a1")
		testutil.AssertResolutionStateEntry(t, evaluatedState, "e5", "check_fraud_rules", "new_check_fraud_rules", "new_e5")
		for _, specID := range specIDs {
			testutil.AssertBaselineHashes(t, evaluatedState, specID, "old_"+specID, "", "")
		}
		for _, markerID := range markerIDs {
			testutil.AssertBaselineHashes(t, evaluatedState, "", "", markerID, "old_"+markerID)
		}
		evaluatedStateTodo := evaluateTodoActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs: evaluatedState.Specs, Markers: evaluatedState.Markers, Links: links,
			ResolutionState: evaluatedState.ResolutionState, Action: core.TodoAction{Scan: scan}})
		testutil.AssertTodoCount(t, evaluatedStateTodo, 7)

		for _, link := range links {
			currentContext.Action = core.ResetAction{SpecID: link.SpecID, MarkerID: link.MarkerID, Scan: scan}
			evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
			currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState
		}
		testutil.AssertResolutionStateCount(t, evaluatedState, 0)
		for _, specID := range specIDs {
			testutil.AssertBaselineHashes(t, evaluatedState, specID, "new_"+specID, "", "")
		}
		for _, markerID := range markerIDs {
			testutil.AssertBaselineHashes(t, evaluatedState, "", "", markerID, "new_"+markerID)
		}
	})

	t.Run("partial_collapse_marker_only", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "bs1"), testutil.NewSpec("s2", "bs2")}
		markers := []core.Marker{testutil.NewMarker("m1", "bm1"), testutil.NewMarker("m2", "bm2")}
		links := []core.Link{testutil.NewLink("s1", "m1"), testutil.NewLink("s2", "m1"), testutil.NewLink("s1", "m2"), testutil.NewLink("s2", "m2")}
		scan := newScanFromBaselines(specs, markers,
			map[string]string{"s1": "cs1", "s2": "cs2"},
			map[string]string{"m1": "cm1", "m2": "cm2"})

		currentContext := core.CoreAlgorithmContext{Specs: specs, Markers: markers, Links: links}
		currentContext.Action = core.ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scan}
		evaluatedState := evaluateResetActionExpectingSuccess(t, currentContext)
		currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

		currentContext.Action = core.ResetAction{SpecID: "s2", MarkerID: "m1", Scan: scan}
		evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
		currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

		testutil.AssertBaselineHashes(t, evaluatedState, "", "", "m1", "cm1")
		testutil.AssertBaselineHashes(t, evaluatedState, "", "", "m2", "bm2")
		testutil.AssertBaselineHashes(t, evaluatedState, "s1", "bs1", "", "")
		testutil.AssertBaselineHashes(t, evaluatedState, "s2", "bs2", "", "")
		testutil.AssertResolutionStateCount(t, evaluatedState, 2)
		testutil.AssertResolutionStateEntry(t, evaluatedState, "m1", "s1", "cs1", "cm1")
		testutil.AssertResolutionStateEntry(t, evaluatedState, "m1", "s2", "cs2", "cm1")

		evaluatedStateTodo := evaluateTodoActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs: evaluatedState.Specs, Markers: evaluatedState.Markers, Links: links,
			ResolutionState: evaluatedState.ResolutionState, Action: core.TodoAction{Scan: scan}})
		testutil.AssertTodoCount(t, evaluatedStateTodo, 2)
		if _, ok := findTodoByEdge(evaluatedStateTodo, "m2", "s1"); !ok {
			t.Fatalf("expected todo for m2:s1")
		}
		if _, ok := findTodoByEdge(evaluatedStateTodo, "m2", "s2"); !ok {
			t.Fatalf("expected todo for m2:s2")
		}

		currentContext.Action = core.ResetAction{SpecID: "s1", MarkerID: "m2", Scan: scan}
		evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
		currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState
		currentContext.Action = core.ResetAction{SpecID: "s2", MarkerID: "m2", Scan: scan}
		evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)

		testutil.AssertResolutionStateCount(t, evaluatedState, 0)
		testutil.AssertBaselineHashes(t, evaluatedState, "s1", "cs1", "m1", "cm1")
		testutil.AssertBaselineHashes(t, evaluatedState, "s2", "cs2", "m2", "cm2")
	})

	t.Run("reset_idempotent_when_unchanged", func(t *testing.T) {
		specs := []core.Spec{testutil.NewSpec("s1", "bs")}
		markers := []core.Marker{testutil.NewMarker("m1", "bm")}
		links := []core.Link{testutil.NewLink("s1", "m1")}
		scan := newBaselineScan(specs, markers)

		evaluatedState := evaluateResetActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs: specs, Markers: markers, Links: links,
			Action: core.ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scan}})
		testutil.AssertResolutionStateCount(t, evaluatedState, 0)
		testutil.AssertBaselineHashes(t, evaluatedState, "s1", "bs", "m1", "bm")
	})
}

func TestRedriftAfterCollapse(t *testing.T) {
	specs := []core.Spec{testutil.NewSpec("s1", "bs1"), testutil.NewSpec("s2", "bs2")}
	markers := []core.Marker{testutil.NewMarker("m1", "bm1"), testutil.NewMarker("m2", "bm2")}
	links := []core.Link{testutil.NewLink("s1", "m1"), testutil.NewLink("s1", "m2"), testutil.NewLink("s2", "m1"), testutil.NewLink("s2", "m2")}
	scanAllChanged := newScanFromBaselines(specs, markers,
		map[string]string{"s1": "cs1", "s2": "cs2"},
		map[string]string{"m1": "cm1", "m2": "cm2"})

	currentContext := core.CoreAlgorithmContext{Specs: specs, Markers: markers, Links: links}
	currentContext.Action = core.ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scanAllChanged}
	evaluatedState := evaluateResetActionExpectingSuccess(t, currentContext)
	currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

	currentContext.Action = core.ResetAction{SpecID: "s2", MarkerID: "m1", Scan: scanAllChanged}
	evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
	currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

	testutil.AssertBaselineHashes(t, evaluatedState, "", "", "m1", "cm1")
	testutil.AssertResolutionStateCount(t, evaluatedState, 2)

	currentContext.Action = core.ResetAction{SpecID: "s1", MarkerID: "m2", Scan: scanAllChanged}
	evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
	currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

	currentContext.Action = core.ResetAction{SpecID: "s2", MarkerID: "m2", Scan: scanAllChanged}
	evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)
	currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState

	testutil.AssertResolutionStateCount(t, evaluatedState, 0)
	testutil.AssertBaselineHashes(t, evaluatedState, "s1", "cs1", "m1", "cm1")
	testutil.AssertBaselineHashes(t, evaluatedState, "s2", "cs2", "m2", "cm2")

	scanAfterMarkerM1Rechanges := newScanFromBaselines(evaluatedState.Specs, evaluatedState.Markers,
		nil,
		map[string]string{"m1": "cm1_v2"})

	evaluatedStateTodo := evaluateTodoActionExpectingSuccess(t, core.CoreAlgorithmContext{
		Specs: evaluatedState.Specs, Markers: evaluatedState.Markers, Links: links,
		ResolutionState: evaluatedState.ResolutionState,
		Action:          core.TodoAction{Scan: scanAfterMarkerM1Rechanges}})
	testutil.AssertTodoCount(t, evaluatedStateTodo, 2)
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
	specs := []core.Spec{testutil.NewSpec("s1", "baseline_s1"), testutil.NewSpec("s2", "baseline_s2")}
	markers := []core.Marker{testutil.NewMarker("m1", "baseline_m1"), testutil.NewMarker("m2", "baseline_m2")}
	links := []core.Link{testutil.NewLink("s1", "m1"), testutil.NewLink("s1", "m2"), testutil.NewLink("s2", "m1"), testutil.NewLink("s2", "m2")}
	scan := newScanFromBaselines(specs, markers,
		map[string]string{"s1": "current_s1"},
		map[string]string{"m1": "current_m1"})

	t.Run("initial_todos_count_only_drifted_edges", func(t *testing.T) {
		evaluatedState := evaluateTodoActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs: specs, Markers: markers, Links: links, Action: core.TodoAction{Scan: scan}})
		testutil.AssertTodoCount(t, evaluatedState, 3)
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
		currentContext := core.CoreAlgorithmContext{Specs: specs, Markers: markers, Links: links}
		currentContext.Action = core.ResetAction{SpecID: "s2", MarkerID: "m1", Scan: scan}
		evaluatedState := evaluateResetActionExpectingSuccess(t, currentContext)

		testutil.AssertBaselineHashes(t, evaluatedState, "s2", "baseline_s2", "m2", "baseline_m2")
		testutil.AssertBaselineHashes(t, evaluatedState, "s1", "baseline_s1", "m1", "baseline_m1")
		testutil.AssertResolutionStateEntry(t, evaluatedState, "m1", "s2", "baseline_s2", "current_m1")
		testutil.AssertResolutionStateCount(t, evaluatedState, 1)

		currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState
		currentContext.Action = core.ResetAction{SpecID: "s1", MarkerID: "m1", Scan: scan}
		evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)

		testutil.AssertBaselineHashes(t, evaluatedState, "s1", "baseline_s1", "m1", "current_m1")
		testutil.AssertBaselineHashes(t, evaluatedState, "s2", "baseline_s2", "m2", "baseline_m2")
		testutil.AssertResolutionStateEntry(t, evaluatedState, "m1", "s1", "current_s1", "current_m1")
		testutil.AssertResolutionStateCount(t, evaluatedState, 1)

		currentContext.Specs, currentContext.Markers, currentContext.ResolutionState = evaluatedState.Specs, evaluatedState.Markers, evaluatedState.ResolutionState
		currentContext.Action = core.ResetAction{SpecID: "s1", MarkerID: "m2", Scan: scan}
		evaluatedState = evaluateResetActionExpectingSuccess(t, currentContext)

		testutil.AssertResolutionStateCount(t, evaluatedState, 0)
		testutil.AssertBaselineHashes(t, evaluatedState, "s1", "current_s1", "m1", "current_m1")
		testutil.AssertBaselineHashes(t, evaluatedState, "s2", "baseline_s2", "m2", "baseline_m2")

		evaluatedStateTodo := evaluateTodoActionExpectingSuccess(t, core.CoreAlgorithmContext{
			Specs: evaluatedState.Specs, Markers: evaluatedState.Markers, Links: links,
			ResolutionState: evaluatedState.ResolutionState, Action: core.TodoAction{Scan: scan}})
		testutil.AssertTodoCount(t, evaluatedStateTodo, 0)
	})
}

func TestValidate(t *testing.T) {
	baseSpecs := []core.Spec{testutil.NewSpec("s1", "b1"), testutil.NewSpec("s2", "b2")}
	baseMarkers := []core.Marker{testutil.NewMarker("m1", "b1"), testutil.NewMarker("m2", "b2")}
	baseLinks := []core.Link{testutil.NewLink("s1", "m1"), testutil.NewLink("s2", "m2")}

	cases := []struct {
		name    string
		ctx     core.CoreAlgorithmContext
		viaEval bool
		target  error
	}{
		{
			name:   "duplicate_spec_id",
			ctx:    core.CoreAlgorithmContext{Specs: []core.Spec{testutil.NewSpec("s1", "a"), testutil.NewSpec("s1", "b")}},
			target: core.ErrDuplicateSpecID,
		},
		{
			name:   "duplicate_marker_id",
			ctx:    core.CoreAlgorithmContext{Markers: []core.Marker{testutil.NewMarker("m1", "a"), testutil.NewMarker("m1", "b")}},
			target: core.ErrDuplicateMarkerID,
		},
		{
			name:   "duplicate_link",
			ctx:    core.CoreAlgorithmContext{Specs: baseSpecs, Markers: baseMarkers, Links: []core.Link{testutil.NewLink("s1", "m1"), testutil.NewLink("s1", "m1")}},
			target: core.ErrDuplicateLink,
		},
		{
			name:   "link_unknown_spec",
			ctx:    core.CoreAlgorithmContext{Markers: baseMarkers, Links: []core.Link{testutil.NewLink("sX", "m1")}},
			target: core.ErrLinkUnknownSpec,
		},
		{
			name:   "link_unknown_marker",
			ctx:    core.CoreAlgorithmContext{Specs: baseSpecs, Links: []core.Link{testutil.NewLink("s1", "mX")}},
			target: core.ErrLinkUnknownMarker,
		},
		{
			name:   "resolution_unknown_spec",
			ctx:    core.CoreAlgorithmContext{Specs: baseSpecs, Markers: baseMarkers, Links: baseLinks, ResolutionState: []core.ResolutionState{testutil.NewResolutionState("sX", "m1", "x", "y")}},
			target: core.ErrResolutionUnknownSpec,
		},
		{
			name:   "resolution_unknown_marker",
			ctx:    core.CoreAlgorithmContext{Specs: baseSpecs, Markers: baseMarkers, Links: baseLinks, ResolutionState: []core.ResolutionState{testutil.NewResolutionState("s1", "mX", "x", "y")}},
			target: core.ErrResolutionUnknownMarker,
		},
		{
			name: "resolution_edge_not_linked",
			ctx: core.CoreAlgorithmContext{
				Specs:           []core.Spec{testutil.NewSpec("s1", "b"), testutil.NewSpec("s2", "b")},
				Markers:         []core.Marker{testutil.NewMarker("m1", "b"), testutil.NewMarker("m2", "b")},
				Links:           []core.Link{testutil.NewLink("s1", "m1")},
				ResolutionState: []core.ResolutionState{testutil.NewResolutionState("s2", "m2", "x", "y")},
			},
			target: core.ErrResolutionEdgeNotLinked,
		},
		{
			name: "duplicate_resolution",
			ctx: core.CoreAlgorithmContext{
				Specs:           baseSpecs,
				Markers:         baseMarkers,
				Links:           baseLinks,
				ResolutionState: []core.ResolutionState{testutil.NewResolutionState("s1", "m1", "x", "y"), testutil.NewResolutionState("s1", "m1", "z", "w")},
			},
			target: core.ErrDuplicateResolution,
		},
		{
			name: "scan_missing_spec_hash",
			ctx: core.CoreAlgorithmContext{
				Specs:   baseSpecs,
				Markers: baseMarkers,
				Links:   baseLinks,
				Action:  core.TodoAction{Scan: core.Scan{SpecHashes: map[string]string{"s1": "b"}, MarkerHashes: map[string]string{"m1": "b", "m2": "b"}}},
			},
			viaEval: true,
			target:  core.ErrScanMissingSpecHash,
		},
		{
			name: "scan_missing_marker_hash",
			ctx: core.CoreAlgorithmContext{
				Specs:   baseSpecs,
				Markers: baseMarkers,
				Links:   baseLinks,
				Action:  core.TodoAction{Scan: core.Scan{SpecHashes: map[string]string{"s1": "b", "s2": "b"}, MarkerHashes: map[string]string{"m1": "b"}}},
			},
			viaEval: true,
			target:  core.ErrScanMissingMarkerHash,
		},
		{
			name: "scan_unknown_spec_hash",
			ctx: core.CoreAlgorithmContext{
				Specs:   baseSpecs,
				Markers: baseMarkers,
				Links:   baseLinks,
				Action:  core.TodoAction{Scan: core.Scan{SpecHashes: map[string]string{"s1": "b", "s2": "b", "sX": "b"}, MarkerHashes: map[string]string{"m1": "b", "m2": "b"}}},
			},
			viaEval: true,
			target:  core.ErrScanUnknownSpecHash,
		},
		{
			name: "scan_unknown_marker_hash",
			ctx: core.CoreAlgorithmContext{
				Specs:   baseSpecs,
				Markers: baseMarkers,
				Links:   baseLinks,
				Action:  core.TodoAction{Scan: core.Scan{SpecHashes: map[string]string{"s1": "b", "s2": "b"}, MarkerHashes: map[string]string{"m1": "b", "m2": "b", "mX": "b"}}},
			},
			viaEval: true,
			target:  core.ErrScanUnknownMarkerHash,
		},
		{
			name: "unknown_action",
			ctx: core.CoreAlgorithmContext{
				Specs:   baseSpecs,
				Markers: baseMarkers,
				Links:   baseLinks,
				Action:  nil,
			},
			viaEval: true,
			target:  core.ErrUnknownAction,
		},
		{
			name: "reset_edge_not_linked",
			ctx: core.CoreAlgorithmContext{
				Specs:   baseSpecs,
				Markers: baseMarkers,
				Links:   baseLinks,
				Action:  core.ResetAction{SpecID: "s1", MarkerID: "m2", Scan: newBaselineScan(baseSpecs, baseMarkers)},
			},
			viaEval: true,
			target:  core.ErrResetEdgeNotLinked,
		},
		{
			name: "reset_scan_missing_spec_hash",
			ctx: core.CoreAlgorithmContext{
				Specs:   baseSpecs,
				Markers: baseMarkers,
				Links:   baseLinks,
				Action: core.ResetAction{
					SpecID: "s1", MarkerID: "m1",
					Scan: core.Scan{SpecHashes: map[string]string{"s2": "b"}, MarkerHashes: map[string]string{"m1": "b", "m2": "b"}},
				},
			},
			viaEval: true,
			target:  core.ErrScanMissingSpecHash,
		},
	}

	for _, validateCase := range cases {
		t.Run(validateCase.name, func(t *testing.T) {
			var err error
			if validateCase.viaEval {
				_, err = core.NewCoreAlgorithm().EvaluateState(validateCase.ctx)
			} else {
				err = validateCase.ctx.Validate()
			}
			testutil.AssertErrorWraps(t, err, validateCase.target)
		})
	}

	t.Run("valid_context_passes_validate", func(t *testing.T) {
		ctx := core.CoreAlgorithmContext{
			Specs:           baseSpecs,
			Markers:         baseMarkers,
			Links:           baseLinks,
			ResolutionState: []core.ResolutionState{testutil.NewResolutionState("s1", "m1", "x", "y")},
		}
		testutil.AssertNoError(t, ctx.Validate())
	})

	t.Run("empty_context_passes_validate", func(t *testing.T) {
		testutil.AssertNoError(t, core.CoreAlgorithmContext{}.Validate())
	})
}
