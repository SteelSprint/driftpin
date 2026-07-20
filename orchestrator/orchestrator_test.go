package orchestrator_test

import (
	"errors"
	"testing"

	"drift/core"
	"drift/internal/fileio"
	"drift/internal/testutil"
	"drift/orchestrator"
	"drift/scanner"
	"drift/statestore"
)

type fakeStateStore struct {
	state          statestore.State
	loadErr        error
	saveErr        error
	saved          []statestore.State
	initialized    bool
	initializedErr error
}

func (f *fakeStateStore) Load(_ *fileio.Session) (statestore.State, error) {
	if f.loadErr != nil {
		return statestore.State{}, f.loadErr
	}
	return f.state, nil
}

func (f *fakeStateStore) Save(_ *fileio.Session, state statestore.State) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.state = state
	f.saved = append(f.saved, state)
	return nil
}

func (f *fakeStateStore) Initialized() (bool, error) {
	if f.initializedErr != nil {
		return false, f.initializedErr
	}
	return f.initialized, nil
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

func (f *fakeScanner) Dir() string {
	return ""
}

// TestOrchestrator_Todo_NoDrift: scan matches baseline → no closures.
func TestOrchestrator_Todo_NoDrift(t *testing.T) {
	spec := testutil.NewSpec("m.a", "hash-a")
	marker := testutil.NewMarker("cval", "hash-m")
	link := testutil.NewLink("m.a", "cval")
	store := &fakeStateStore{state: statestore.State{
		Specs:   []core.Spec{spec},
		Markers: []core.Marker{marker},
		Edges:   []core.Edge{link},
	}}
	sc := &fakeScanner{result: scanner.ScanResult{
		Specs:   []core.Spec{spec},
		Markers: []core.Marker{marker},
	}}
	orch := orchestrator.NewOrchestrator(store, sc, nil)
	state, err := orch.Todo(nil)
	testutil.AssertNoError(t, err)
	testutil.AssertClosureCount(t, state, 0)
}

// TestOrchestrator_Todo_SpecDrift: spec hash differs → closure includes spec + marker.
func TestOrchestrator_Todo_SpecDrift(t *testing.T) {
	baselineSpec := testutil.NewSpec("m.a", "old")
	scanSpec := testutil.NewSpec("m.a", "new")
	marker := testutil.NewMarker("cval", "hash-m")
	link := testutil.NewLink("m.a", "cval")
	store := &fakeStateStore{state: statestore.State{
		Specs:   []core.Spec{baselineSpec},
		Markers: []core.Marker{marker},
		Edges:   []core.Edge{link},
	}}
	sc := &fakeScanner{result: scanner.ScanResult{
		Specs:   []core.Spec{scanSpec},
		Markers: []core.Marker{marker},
	}}
	orch := orchestrator.NewOrchestrator(store, sc, nil)
	state, err := orch.Todo(nil)
	testutil.AssertNoError(t, err)
	testutil.AssertClosureCount(t, state, 1)
	testutil.AssertNodeInClosure(t, state.Closures[0], "m.a")
	testutil.AssertNodeInClosure(t, state.Closures[0], "cval")
}

// TestOrchestrator_ResetClosure: spec drift → reset → baseline synced.
func TestOrchestrator_ResetClosure(t *testing.T) {
	baselineSpec := testutil.NewSpec("m.a", "old")
	scanSpec := testutil.NewSpec("m.a", "new")
	store := &fakeStateStore{state: statestore.State{
		Specs: []core.Spec{baselineSpec},
	}}
	sc := &fakeScanner{result: scanner.ScanResult{
		Specs: []core.Spec{scanSpec},
	}}
	orch := orchestrator.NewOrchestrator(store, sc, nil)
	state, err := orch.Todo(nil)
	testutil.AssertNoError(t, err)
	testutil.AssertClosureCount(t, state, 1)
	hash := state.Closures[0].Hash

	_, err = orch.ResetClosure(nil, hash)
	testutil.AssertNoError(t, err)

	if len(store.saved) != 1 {
		t.Fatalf("expected 1 save, got %d", len(store.saved))
	}
	saved := store.saved[0]
	if len(saved.Specs) != 1 || saved.Specs[0].Hash != "new" {
		t.Fatalf("baseline not synced: %+v", saved.Specs)
	}
}

// TestOrchestrator_ResetClosure_NotFound: bad hash → ErrResetClosureNotFound.
func TestOrchestrator_ResetClosure_NotFound(t *testing.T) {
	spec := testutil.NewSpec("m.a", "x")
	store := &fakeStateStore{state: statestore.State{Specs: []core.Spec{spec}}}
	sc := &fakeScanner{result: scanner.ScanResult{Specs: []core.Spec{spec}}}
	orch := orchestrator.NewOrchestrator(store, sc, nil)
	_, err := orch.ResetClosure(nil, "deadbeef")
	if !errors.Is(err, orchestrator.ErrResetClosureNotFound) {
		t.Fatalf("expected ErrResetClosureNotFound, got %v", err)
	}
}

// TestOrchestrator_Link: link creates an edge.
func TestOrchestrator_Link(t *testing.T) {
	spec := testutil.NewSpec("m.a", "hash-a")
	marker := testutil.NewMarker("cval", "hash-m")
	store := &fakeStateStore{state: statestore.State{
		Specs:   []core.Spec{spec},
		Markers: []core.Marker{marker},
	}}
	sc := &fakeScanner{result: scanner.ScanResult{
		Specs:   []core.Spec{spec},
		Markers: []core.Marker{marker},
	}}
	orch := orchestrator.NewOrchestrator(store, sc, nil)
	if err := orch.Link(nil, "cval", "m.a"); err != nil {
		t.Fatalf("Link: %v", err)
	}
	if len(store.state.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(store.state.Edges))
	}
}

// TestOrchestrator_Unlink: unlink removes the edge.
func TestOrchestrator_Unlink(t *testing.T) {
	spec := testutil.NewSpec("m.a", "hash-a")
	marker := testutil.NewMarker("cval", "hash-m")
	link := testutil.NewLink("m.a", "cval")
	store := &fakeStateStore{state: statestore.State{
		Specs:   []core.Spec{spec},
		Markers: []core.Marker{marker},
		Edges:   []core.Edge{link},
	}}
	sc := &fakeScanner{result: scanner.ScanResult{
		Specs:   []core.Spec{spec},
		Markers: []core.Marker{marker},
	}}
	orch := orchestrator.NewOrchestrator(store, sc, nil)
	if err := orch.Unlink(nil, "cval", "m.a"); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if len(store.state.Edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(store.state.Edges))
	}
}

// TestOrchestrator_Todo_SpecRemoved: spec deleted from scan → closure with NODE_REMOVED event.
func TestOrchestrator_Todo_SpecRemoved(t *testing.T) {
	spec := testutil.NewSpec("m.a", "hash-a")
	store := &fakeStateStore{state: statestore.State{
		Specs: []core.Spec{spec},
	}}
	sc := &fakeScanner{result: scanner.ScanResult{}} // empty scan: spec deleted
	orch := orchestrator.NewOrchestrator(store, sc, nil)
	state, err := orch.Todo(nil)
	testutil.AssertNoError(t, err)
	testutil.AssertClosureCount(t, state, 1)
	if len(state.Closures[0].Events) != 1 {
		t.Fatalf("want 1 event, got %d", len(state.Closures[0].Events))
	}
	if state.Closures[0].Events[0].Kind != core.EventNodeRemoved {
		t.Fatalf("want NODE_REMOVED, got %v", state.Closures[0].Events[0].Kind)
	}
}

// TestOrchestrator_ResetClosure_SpecRemoved: deleting a spec then resetting
// the closure should remove the spec from baseline (no ghost node).
func TestOrchestrator_ResetClosure_SpecRemoved(t *testing.T) {
	spec := testutil.NewSpec("m.a", "hash-a")
	store := &fakeStateStore{state: statestore.State{
		Specs: []core.Spec{spec},
	}}
	sc := &fakeScanner{result: scanner.ScanResult{}}
	orch := orchestrator.NewOrchestrator(store, sc, nil)
	state, err := orch.Todo(nil)
	testutil.AssertNoError(t, err)
	testutil.AssertClosureCount(t, state, 1)
	hash := state.Closures[0].Hash

	_, err = orch.ResetClosure(nil, hash)
	testutil.AssertNoError(t, err)

	if len(store.saved) != 1 {
		t.Fatalf("expected 1 save, got %d", len(store.saved))
	}
	saved := store.saved[0]
	for _, s := range saved.Specs {
		if s.ID == "m.a" {
			t.Fatalf("ghost spec m.a remains in baseline after reset: %+v", saved.Specs)
		}
	}
}

// TestOrchestrator_Todo_NewSpecAdded: spec appears in scan only → closure with NODE_ADDED event.
func TestOrchestrator_Todo_NewSpecAdded(t *testing.T) {
	store := &fakeStateStore{state: statestore.State{}} // empty baseline
	scanSpec := testutil.NewSpec("m.a", "hash-a")
	sc := &fakeScanner{result: scanner.ScanResult{
		Specs: []core.Spec{scanSpec},
	}}
	orch := orchestrator.NewOrchestrator(store, sc, nil)
	state, err := orch.Todo(nil)
	testutil.AssertNoError(t, err)
	testutil.AssertClosureCount(t, state, 1)
	if state.Closures[0].Events[0].Kind != core.EventNodeAdded {
		t.Fatalf("want NODE_ADDED, got %v", state.Closures[0].Events[0].Kind)
	}
}

// TestOrchestrator_ResetClosure_NewSpecAdded: resetting a NODE_ADDED closure
// establishes the baseline hash so the next todo is clean.
func TestOrchestrator_ResetClosure_NewSpecAdded(t *testing.T) {
	store := &fakeStateStore{state: statestore.State{}}
	scanSpec := testutil.NewSpec("m.a", "hash-a")
	sc := &fakeScanner{result: scanner.ScanResult{
		Specs: []core.Spec{scanSpec},
	}}
	orch := orchestrator.NewOrchestrator(store, sc, nil)
	state, err := orch.Todo(nil)
	testutil.AssertNoError(t, err)
	hash := state.Closures[0].Hash

	_, err = orch.ResetClosure(nil, hash)
	testutil.AssertNoError(t, err)

	if len(store.state.Specs) != 1 || store.state.Specs[0].Hash != "hash-a" {
		t.Fatalf("baseline not established: %+v", store.state.Specs)
	}
}

// TestOrchestrator_Todo_StrictDisjoint: S1 and S2 both drift, both cited by S3.
// Two closures; S3 in both. Resetting one doesn't affect the other.
func TestOrchestrator_Todo_StrictDisjoint(t *testing.T) {
	s1 := testutil.NewSpec("m.s1", "old1")
	s2 := testutil.NewSpec("m.s2", "old2")
	s3 := testutil.NewSpec("m.s3", "s3")
	scanS1 := testutil.NewSpec("m.s1", "new1")
	scanS2 := testutil.NewSpec("m.s2", "new2")
	scanS3 := testutil.NewSpec("m.s3", "s3")
	baselineEdges := []core.Edge{
		testutil.NewRef("m.s3", "m.s1"),
		testutil.NewRef("m.s3", "m.s2"),
	}
	store := &fakeStateStore{state: statestore.State{
		Specs: []core.Spec{s1, s2, s3},
		Edges: baselineEdges,
	}}
	sc := &fakeScanner{result: scanner.ScanResult{
		Specs:  []core.Spec{scanS1, scanS2, scanS3},
		Edges:  baselineEdges, // refs preserved in scan
	}}
	orch := orchestrator.NewOrchestrator(store, sc, nil)
	state, err := orch.Todo(nil)
	testutil.AssertNoError(t, err)
	testutil.AssertClosureCount(t, state, 2)
	s1Closure := testutil.FindClosureContainingNode(t, state, "m.s1")
	s2Closure := testutil.FindClosureContainingNode(t, state, "m.s2")
	if s1Closure.Hash == s2Closure.Hash {
		t.Fatalf("strict-disjoint closures should have different hashes")
	}
	testutil.AssertNodeInClosure(t, s1Closure, "m.s3")
	testutil.AssertNodeInClosure(t, s2Closure, "m.s3")

	// Reset only closure_S1. closure_S2 should remain.
	_, err = orch.ResetClosure(nil, s1Closure.Hash)
	testutil.AssertNoError(t, err)
	// saved state has S1 with new hash, S2 still has old hash.
	for _, s := range store.state.Specs {
		if s.ID == "m.s1" && s.Hash != "new1" {
			t.Fatalf("S1 not synced: %+v", s)
		}
		if s.ID == "m.s2" && s.Hash != "old2" {
			t.Fatalf("S2 unexpectedly modified: %+v", s)
		}
	}
}

// TestOrchestrator_DiffClosure: diff a closure by hash returns one DiffResult
// per node in the closure.
func TestOrchestrator_DiffClosure(t *testing.T) {
	spec := testutil.NewSpec("m.a", "old")
	scanSpec := testutil.NewSpec("m.a", "new")
	marker := testutil.NewMarker("cval", "hash-m")
	link := testutil.NewLink("m.a", "cval")
	store := &fakeStateStore{state: statestore.State{
		Specs:   []core.Spec{spec},
		Markers: []core.Marker{marker},
		Edges:   []core.Edge{link},
	}}
	sc := &fakeScanner{result: scanner.ScanResult{
		Specs:   []core.Spec{scanSpec},
		Markers: []core.Marker{marker},
	}}
	orch := orchestrator.NewOrchestrator(store, sc, nil)

	// Get the closure hash via Todo.
	state, err := orch.Todo(nil)
	testutil.AssertNoError(t, err)
	testutil.AssertClosureCount(t, state, 1)
	hash := state.Closures[0].Hash

	// DiffClosure should return one DiffResult per node (spec + marker = 2).
	diffs, err := orch.DiffClosure(nil, hash)
	testutil.AssertNoError(t, err)
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs (spec + marker), got %d", len(diffs))
	}

	// Verify both spec and marker sides are present.
	specCount, markerCount := 0, 0
	for _, d := range diffs {
		if d.Spec != nil {
			specCount++
			if d.Spec.ID != "m.a" {
				t.Fatalf("spec diff ID = %q, want %q", d.Spec.ID, "m.a")
			}
			if d.Spec.BaselineHash != "old" {
				t.Fatalf("spec baseline hash = %q, want %q", d.Spec.BaselineHash, "old")
			}
			if d.Spec.CurrentHash != "new" {
				t.Fatalf("spec current hash = %q, want %q", d.Spec.CurrentHash, "new")
			}
		}
		if d.Marker != nil {
			markerCount++
			if d.Marker.ID != "cval" {
				t.Fatalf("marker diff ID = %q, want %q", d.Marker.ID, "cval")
			}
		}
	}
	if specCount != 1 {
		t.Fatalf("expected 1 spec diff, got %d", specCount)
	}
	if markerCount != 1 {
		t.Fatalf("expected 1 marker diff, got %d", markerCount)
	}
}

// TestOrchestrator_DiffClosure_NotFound: unknown hash returns ErrDiffClosureNotFound.
func TestOrchestrator_DiffClosure_NotFound(t *testing.T) {
	spec := testutil.NewSpec("m.a", "x")
	store := &fakeStateStore{state: statestore.State{Specs: []core.Spec{spec}}}
	sc := &fakeScanner{result: scanner.ScanResult{Specs: []core.Spec{spec}}}
	orch := orchestrator.NewOrchestrator(store, sc, nil)
	_, err := orch.DiffClosure(nil, "deadbeef")
	if !errors.Is(err, orchestrator.ErrDiffClosureNotFound) {
		t.Fatalf("expected ErrDiffClosureNotFound, got %v", err)
	}
}

// TestOrchestrator_DiffAll: returns one ClosureDiff per drift closure,
// each carrying its hash, events, and per-node diffs.
func TestOrchestrator_DiffAll(t *testing.T) {
	s1 := testutil.NewSpec("m.s1", "old1")
	s2 := testutil.NewSpec("m.s2", "old2")
	scanS1 := testutil.NewSpec("m.s1", "new1")
	scanS2 := testutil.NewSpec("m.s2", "new2")
	store := &fakeStateStore{state: statestore.State{
		Specs: []core.Spec{s1, s2},
	}}
	sc := &fakeScanner{result: scanner.ScanResult{
		Specs: []core.Spec{scanS1, scanS2},
	}}
	orch := orchestrator.NewOrchestrator(store, sc, nil)

	// Two drifts = two closures.
	state, err := orch.Todo(nil)
	testutil.AssertNoError(t, err)
	testutil.AssertClosureCount(t, state, 2)

	closures, _, err := orch.DiffAll(nil)
	testutil.AssertNoError(t, err)
	if len(closures) != 2 {
		t.Fatalf("expected 2 ClosureDiff entries, got %d", len(closures))
	}

	seenHashes := map[string]bool{}
	for _, cd := range closures {
		if seenHashes[cd.Hash] {
			t.Fatalf("duplicate closure hash in DiffAll output: %q", cd.Hash)
		}
		seenHashes[cd.Hash] = true
		if len(cd.Events) != 1 {
			t.Fatalf("closure %q: expected 1 event, got %d", cd.Hash, len(cd.Events))
		}
		if cd.Events[0].Kind != core.EventNodeChanged {
			t.Fatalf("closure %q: expected NODE_CHANGED, got %v", cd.Hash, cd.Events[0].Kind)
		}
		// Each closure has one spec node, so 1 diff entry.
		if len(cd.Diffs) != 1 {
			t.Fatalf("closure %q: expected 1 diff, got %d", cd.Hash, len(cd.Diffs))
		}
		if cd.Diffs[0].Spec == nil {
			t.Fatalf("closure %q: expected Spec diff, got nil", cd.Hash)
		}
	}
}

// TestOrchestrator_DiffAll_NoDrift: no drift → empty result, clean state.
func TestOrchestrator_DiffAll_NoDrift(t *testing.T) {
	spec := testutil.NewSpec("m.a", "x")
	store := &fakeStateStore{state: statestore.State{Specs: []core.Spec{spec}}}
	sc := &fakeScanner{result: scanner.ScanResult{Specs: []core.Spec{spec}}}
	orch := orchestrator.NewOrchestrator(store, sc, nil)

	closures, state, err := orch.DiffAll(nil)
	testutil.AssertNoError(t, err)
	if len(closures) != 0 {
		t.Fatalf("expected 0 closures on clean tree, got %d", len(closures))
	}
	testutil.AssertClosureCount(t, state, 0)
}

// TestOrchestrator_ClosureMerge_DefensiveOnly: documents that the
// closure-merge case (two seeds → same hash → combined events) is
// unreachable in practice under Validate's directed-cycle rejection.
//
// Two seeds produce the same closure hash iff they have identical closure
// memberships, which requires each seed to be a transitive citer of the
// other — i.e., a directed cycle. Validate rejects directed cycles among
// spec-spec edges, and markers can't be cited at all, so the merge path
// in DeriveClosures is defensive-only. This test exists to document that
// invariant and catch regressions if cycle rejection is ever relaxed.
func TestOrchestrator_ClosureMerge_DefensiveOnly(t *testing.T) {
	// If we could construct two seeds with identical closure memberships,
	// they'd merge. Under directed-cycle rejection, this is impossible.
	// Verify by constructing the closest realistic case — two drifted
	// specs both cited by a third — and confirming they stay disjoint.
	s1 := testutil.NewSpec("m.s1", "old1")
	s2 := testutil.NewSpec("m.s2", "old2")
	s3 := testutil.NewSpec("m.s3", "s3")
	scanS1 := testutil.NewSpec("m.s1", "new1")
	scanS2 := testutil.NewSpec("m.s2", "new2")
	scanS3 := testutil.NewSpec("m.s3", "s3")
	edges := []core.Edge{
		testutil.NewRef("m.s3", "m.s1"),
		testutil.NewRef("m.s3", "m.s2"),
	}
	store := &fakeStateStore{state: statestore.State{
		Specs: []core.Spec{s1, s2, s3},
		Edges: edges,
	}}
	sc := &fakeScanner{result: scanner.ScanResult{
		Specs: []core.Spec{scanS1, scanS2, scanS3},
		Edges: edges,
	}}
	orch := orchestrator.NewOrchestrator(store, sc, nil)
	state, err := orch.Todo(nil)
	testutil.AssertNoError(t, err)
	// Two drifted specs cited by a shared citer → two strictly disjoint
	// closures (the merge path is NOT taken).
	testutil.AssertClosureCount(t, state, 2)
	if state.Closures[0].Hash == state.Closures[1].Hash {
		t.Fatalf("two seeds with shared citer should not merge: both hash = %q", state.Closures[0].Hash)
	}
}
