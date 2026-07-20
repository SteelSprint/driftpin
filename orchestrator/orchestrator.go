package orchestrator

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"drift/core"
	"drift/internal/fileio"
	"drift/scanner"
	"drift/statestore"
)

var (
	ErrLinkMarkerNotFound      = fmt.Errorf("link references unknown marker")
	ErrLinkSpecNotFound        = fmt.Errorf("link references unknown spec")
	ErrLinkAlreadyExists       = fmt.Errorf("link already exists")
	ErrUnlinkNotFound          = fmt.Errorf("no link found between marker and spec")
	ErrDiffClosureNotFound     = fmt.Errorf("closure hash not found in current drift")
	ErrDiffNodeNotFound        = fmt.Errorf("no spec or marker found for diff")
	ErrAlreadyInitialized      = fmt.Errorf("project already initialized")
	ErrResetClosureNotFound    = fmt.Errorf("closure hash not found; run `drift todo` to list current closures")
	ErrResetClosureOnlyBroken  = fmt.Errorf("closure contains only broken-edge events; fix the spec text or restore the missing spec")
	markerSyntax               = "D" + "! id=<shortcode>"
)

type Orchestrator struct {
	stateStore statestore.StateStore
	scanner    scanner.Scanner
	core       *core.CoreAlgorithm
	baselines  *statestore.BaselineStore
}

func NewOrchestrator(stateStore statestore.StateStore, scanner scanner.Scanner, baselines *statestore.BaselineStore) *Orchestrator {
	return &Orchestrator{
		stateStore: stateStore,
		scanner:    scanner,
		core:       core.NewCoreAlgorithm(),
		baselines:  baselines,
	}
}

// DiffSide describes one side (spec or marker) of a drift event for diffing.
type DiffSide struct {
	ID           string
	Filepath     string
	Lines        string // "start-end" for markers, "" for specs
	BaselineHash string
	CurrentHash  string
	Baseline     string
	Current      string
	HasBaseline  bool
	Deleted      bool
}

// DiffResult holds a single spec/marker side. Closures may include both
// specs and markers; each is diffed independently. IsSeed reports whether
// this side's node ID is one of the closure's seeds (originated the
// closure) versus a transitively-reached citer. See output.diff_seed_label.
type DiffResult struct {
	Spec   *DiffSide
	Marker *DiffSide
	IsSeed bool
}

// ChangeSummary describes the mutations a state-changing operation would
// apply (or did apply, in dry-run vs post-apply modes). Returned by
// ResetClosure, Link, and Unlink in both preview and post-apply forms.
// See cli.reset_format, cli.link_format, cli.unlink_format.
type ChangeSummary struct {
	Operation   string        // human-readable: "resolve closure a3f7b2c1" / "link cval → main.validate"
	NodeChanges []NodeChange
	EdgeChanges []EdgeChange
}

// NodeChange describes a single spec/marker baseline change.
type NodeChange struct {
	ID      string
	Kind    string // "changed" / "added" / "removed"
	OldHash string // truncated to 8 chars at render time
	NewHash string
}

// EdgeChange describes a single edge add/remove.
type EdgeChange struct {
	From string
	To   string
	Kind string // "added" / "removed"
}

// writeBaseline writes a content-addressed baseline file for the given
// spec or marker using its current scanned hash. Best-effort.
func (o *Orchestrator) writeBaseline(scannedHash, filepath, specID string, startLine, endLine int, isSpec bool) error {
	if o.baselines == nil {
		return nil
	}
	absPath := o.resolvePath(filepath)
	var content string
	var err error
	if isSpec {
		content, err = scanner.ReadSpecContent(absPath, specID)
	} else {
		content, err = scanner.ReadMarkerContent(absPath, startLine, endLine)
	}
	if err != nil {
		return nil
	}
	return o.baselines.Write(scannedHash, content)
}

func (o *Orchestrator) resolvePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(o.scanner.Dir(), p)
}

// D! id=oinit range-start
func (o *Orchestrator) Init(sess *fileio.Session) error {
	initialized, err := o.stateStore.Initialized()
	if err != nil {
		return fmt.Errorf("check initialized state: %w", err)
	}
	if initialized {
		return ErrAlreadyInitialized
	}
	return o.stateStore.Save(sess, statestore.State{})
}

// D! id=oinit range-end

// D! id=otodo range-start
func (o *Orchestrator) Todo(sess *fileio.Session) (core.EvaluatedState, error) {
	state, err := o.stateStore.Load(sess)
	if err != nil {
		return core.EvaluatedState{}, err
	}

	scanResult, err := o.scanner.Scan()
	if err != nil {
		return core.EvaluatedState{}, err
	}

	reconciledSpecs, err := reconcileSpecs(state.Specs, scanResult.Specs)
	if err != nil {
		return core.EvaluatedState{}, err
	}

	reconciledMarkers, err := reconcileMarkers(state.Markers, scanResult.Markers)
	if err != nil {
		return core.EvaluatedState{}, err
	}

	scan := buildScan(scanResult, reconciledSpecs, reconciledMarkers)

	ctx := core.CoreAlgorithmContext{
		Specs:   reconciledSpecs,
		Markers: reconciledMarkers,
		Edges:   state.Edges,
		Action:  core.TodoAction{Scan: scan},
	}

	return o.core.EvaluateState(ctx)
}

// D! id=otodo range-end

// D! id=orest range-start
// ResetClosure locates the closure by hash, applies its seed events to
// baseline (sync baseline hash → scan hash for NODE_CHANGED, add/remove
// edges for EDGE_ADDED/REMOVED, remove node for NODE_REMOVED), and saves.
// Broken-edge events are no-ops; closures with only broken-edge events
// are refused.
func (o *Orchestrator) ResetClosure(sess *fileio.Session, hash string) (core.EvaluatedState, error) {
	evaluated, _, err := o.ResetClosureWithSummary(sess, hash)
	return evaluated, err
}

// ResetClosureWithSummary does the same work as ResetClosure and additionally
// returns a ChangeSummary describing what mutated. Used by the CLI to print
// the per-event change summary after applying (see cli.reset_format).
func (o *Orchestrator) ResetClosureWithSummary(sess *fileio.Session, hash string) (core.EvaluatedState, ChangeSummary, error) {
	return o.resetClosureInner(sess, hash, true)
}

// PreviewResetClosure computes the ChangeSummary that ResetClosure would
// apply, WITHOUT saving state. Used by --dry-run. See cli.reset_format.
func (o *Orchestrator) PreviewResetClosure(sess *fileio.Session, hash string) (ChangeSummary, error) {
	_, summary, err := o.resetClosureInner(sess, hash, false)
	return summary, err
}

func (o *Orchestrator) resetClosureInner(sess *fileio.Session, hash string, save bool) (core.EvaluatedState, ChangeSummary, error) {
	beforeState, err := o.stateStore.Load(sess)
	if err != nil {
		return core.EvaluatedState{}, ChangeSummary{}, err
	}

	scanResult, err := o.scanner.Scan()
	if err != nil {
		return core.EvaluatedState{}, ChangeSummary{}, err
	}

	reconciledSpecs, err := reconcileSpecs(beforeState.Specs, scanResult.Specs)
	if err != nil {
		return core.EvaluatedState{}, ChangeSummary{}, err
	}

	reconciledMarkers, err := reconcileMarkers(beforeState.Markers, scanResult.Markers)
	if err != nil {
		return core.EvaluatedState{}, ChangeSummary{}, err
	}

	scan := buildScan(scanResult, reconciledSpecs, reconciledMarkers)

	ctx := core.CoreAlgorithmContext{
		Specs:   reconciledSpecs,
		Markers: reconciledMarkers,
		Edges:   beforeState.Edges,
		Action:  core.ResetClosureAction{Hash: hash, Scan: scan},
	}

	evaluated, err := o.core.EvaluateState(ctx)
	if err != nil {
		if errors.Is(err, core.ErrClosureNotFound) {
			return core.EvaluatedState{}, ChangeSummary{}, ErrResetClosureNotFound
		}
		if errors.Is(err, core.ErrBrokenEdgeNotResettable) {
			return core.EvaluatedState{}, ChangeSummary{}, ErrResetClosureOnlyBroken
		}
		return core.EvaluatedState{}, ChangeSummary{}, err
	}

	savedEdges := mergeScannedEdges(evaluated.Edges, scanResult.Edges)
	afterState := statestore.State{
		Specs:   evaluated.Specs,
		Markers: evaluated.Markers,
		Edges:   savedEdges,
	}

	if save {
		if err := o.stateStore.Save(sess, afterState); err != nil {
			return core.EvaluatedState{}, ChangeSummary{}, err
		}
		// Best-effort: refresh baseline files for any node whose hash changed.
		for _, ev := range evaluated.Closures {
			if ev.Hash != hash {
				continue
			}
			for _, e := range ev.Events {
				switch e.Kind {
				case core.EventNodeChanged, core.EventNodeAdded:
					if s, ok := findSpecByID(scanResult.Specs, e.NodeID); ok {
						_ = o.writeBaseline(s.Hash, s.Filepath, s.ID, 0, 0, true)
					}
					if m, ok := findMarkerByID(scanResult.Markers, e.NodeID); ok {
						_ = o.writeBaseline(m.Hash, m.Filepath, "", m.LineNumber, m.EndLineNumber, false)
					}
				}
			}
		}
	}

	summary := computeChangeSummary(beforeState, afterState, "resolve closure "+hash)
	return evaluated, summary, nil
}

// D! id=orest range-end

func findSpecByID(specs []core.Spec, id string) (core.Spec, bool) {
	for _, s := range specs {
		if s.ID == id {
			return s, true
		}
	}
	return core.Spec{}, false
}

func findMarkerByID(markers []core.Marker, id string) (core.Marker, bool) {
	for _, m := range markers {
		if m.ID == id {
			return m, true
		}
	}
	return core.Marker{}, false
}

// D! id=olink range-start
// Link constructs a link-style Edge (marker stores edge to spec) and appends
// it to baseline. The edge kind is implicit from endpoint types: marker IDs
// contain no dot, spec IDs contain exactly one.
func (o *Orchestrator) Link(sess *fileio.Session, markerID, specID string) error {
	_, err := o.LinkWithSummary(sess, markerID, specID)
	return err
}

// LinkWithSummary does the same work as Link and returns the ChangeSummary.
func (o *Orchestrator) LinkWithSummary(sess *fileio.Session, markerID, specID string) (ChangeSummary, error) {
	return o.linkInner(sess, markerID, specID, true)
}

// PreviewLink computes the ChangeSummary Link would apply, WITHOUT saving.
func (o *Orchestrator) PreviewLink(sess *fileio.Session, markerID, specID string) (ChangeSummary, error) {
	return o.linkInner(sess, markerID, specID, false)
}

func (o *Orchestrator) linkInner(sess *fileio.Session, markerID, specID string, save bool) (ChangeSummary, error) {
	beforeState, err := o.stateStore.Load(sess)
	if err != nil {
		return ChangeSummary{}, err
	}

	scanResult, err := o.scanner.Scan()
	if err != nil {
		return ChangeSummary{}, err
	}

	reconciledSpecs, err := reconcileSpecs(beforeState.Specs, scanResult.Specs)
	if err != nil {
		return ChangeSummary{}, err
	}

	reconciledMarkers, err := reconcileMarkers(beforeState.Markers, scanResult.Markers)
	if err != nil {
		return ChangeSummary{}, err
	}

	// D! id=cperr range-start
	markerExists := false
	for _, m := range reconciledMarkers {
		if m.ID == markerID {
			markerExists = true
			break
		}
	}
	// D! id=cperr range-end
	if !markerExists {
		var available []string
		for _, m := range reconciledMarkers {
			available = append(available, m.ID)
		}
		return ChangeSummary{}, fmt.Errorf("link references unknown marker %q.\nMarkers must be %s comment lines in code files.\nAvailable markers: %s", markerID, markerSyntax, strings.Join(available, ", "))
	}

	specExists := false
	for _, s := range reconciledSpecs {
		if s.ID == specID {
			specExists = true
			break
		}
	}
	if !specExists {
		var available []string
		for _, s := range reconciledSpecs {
			available = append(available, s.ID)
		}
		return ChangeSummary{}, fmt.Errorf("link references unknown spec %q.\nSpec IDs are module-qualified: <module>.<specId> (e.g. main.example or core.validate).\nAvailable specs: %s", specID, strings.Join(available, ", "))
	}

	for _, e := range beforeState.Edges {
		if e.From == markerID && e.To == specID {
			return ChangeSummary{}, fmt.Errorf("%w: marker=%q spec=%q", ErrLinkAlreadyExists, markerID, specID)
		}
	}

	mergedEdges := mergeScannedEdges(beforeState.Edges, scanResult.Edges)

	scanSpecHashes := make(map[string]string, len(scanResult.Specs))
	for _, s := range scanResult.Specs {
		scanSpecHashes[s.ID] = s.Hash
	}
	scanMarkerHashes := make(map[string]string, len(scanResult.Markers))
	for _, m := range scanResult.Markers {
		scanMarkerHashes[m.ID] = m.Hash
	}
	for i := range reconciledSpecs {
		if reconciledSpecs[i].ID == specID && scanSpecHashes[specID] != "" {
			reconciledSpecs[i].Hash = scanSpecHashes[specID]
		}
	}
	for i := range reconciledMarkers {
		if reconciledMarkers[i].ID == markerID && scanMarkerHashes[markerID] != "" {
			reconciledMarkers[i].Hash = scanMarkerHashes[markerID]
		}
	}

	afterState := statestore.State{
		Specs:   reconciledSpecs,
		Markers: reconciledMarkers,
		Edges:   append(mergedEdges, core.Edge{From: markerID, To: specID}),
	}

	if save {
		if err := o.stateStore.Save(sess, afterState); err != nil {
			return ChangeSummary{}, err
		}
		for _, s := range scanResult.Specs {
			if s.ID == specID {
				_ = o.writeBaseline(s.Hash, s.Filepath, specID, 0, 0, true)
				break
			}
		}
		for _, m := range scanResult.Markers {
			if m.ID == markerID {
				_ = o.writeBaseline(m.Hash, m.Filepath, "", m.LineNumber, m.EndLineNumber, false)
				break
			}
		}
	}

	summary := computeChangeSummary(beforeState, afterState, fmt.Sprintf("link %s → %s", markerID, specID))
	return summary, nil
}

// D! id=olink range-end

// D! id=ounlnk range-start
func (o *Orchestrator) Unlink(sess *fileio.Session, markerID, specID string) error {
	_, err := o.UnlinkWithSummary(sess, markerID, specID)
	return err
}

// UnlinkWithSummary does the same work as Unlink and returns the ChangeSummary.
func (o *Orchestrator) UnlinkWithSummary(sess *fileio.Session, markerID, specID string) (ChangeSummary, error) {
	return o.unlinkInner(sess, markerID, specID, true)
}

// PreviewUnlink computes the ChangeSummary Unlink would apply, WITHOUT saving.
func (o *Orchestrator) PreviewUnlink(sess *fileio.Session, markerID, specID string) (ChangeSummary, error) {
	return o.unlinkInner(sess, markerID, specID, false)
}

func (o *Orchestrator) unlinkInner(sess *fileio.Session, markerID, specID string, save bool) (ChangeSummary, error) {
	beforeState, err := o.stateStore.Load(sess)
	if err != nil {
		return ChangeSummary{}, err
	}

	edgeIndex := -1
	for i, e := range beforeState.Edges {
		if e.From == markerID && e.To == specID {
			edgeIndex = i
			break
		}
	}
	if edgeIndex == -1 {
		return ChangeSummary{}, fmt.Errorf("%w: marker=%q spec=%q", ErrUnlinkNotFound, markerID, specID)
	}

	newEdges := make([]core.Edge, 0, len(beforeState.Edges)-1)
	newEdges = append(newEdges, beforeState.Edges[:edgeIndex]...)
	newEdges = append(newEdges, beforeState.Edges[edgeIndex+1:]...)

	afterState := statestore.State{
		Specs:   beforeState.Specs,
		Markers: beforeState.Markers,
		Edges:   newEdges,
	}
	if save {
		if err := o.stateStore.Save(sess, afterState); err != nil {
			return ChangeSummary{}, err
		}
	}
	summary := computeChangeSummary(beforeState, afterState, fmt.Sprintf("unlink %s → %s", markerID, specID))
	return summary, nil
}

// computeChangeSummary diffs two statestore.State values and returns a
// ChangeSummary describing the mutations. Used by reset/link/unlink in both
// preview (dry-run) and post-apply forms. See output.change_summary_format.
func computeChangeSummary(before, after statestore.State, operation string) ChangeSummary {
	summary := ChangeSummary{Operation: operation}

	beforeSpecs := make(map[string]core.Spec, len(before.Specs))
	for _, s := range before.Specs {
		beforeSpecs[s.ID] = s
	}
	afterSpecs := make(map[string]core.Spec, len(after.Specs))
	for _, s := range after.Specs {
		afterSpecs[s.ID] = s
	}
	for id, a := range afterSpecs {
		if b, ok := beforeSpecs[id]; ok {
			if b.Hash != a.Hash {
				summary.NodeChanges = append(summary.NodeChanges, NodeChange{
					ID: id, Kind: "changed", OldHash: b.Hash, NewHash: a.Hash,
				})
			}
		} else {
			summary.NodeChanges = append(summary.NodeChanges, NodeChange{
				ID: id, Kind: "added", NewHash: a.Hash,
			})
		}
	}
	for id, b := range beforeSpecs {
		if _, ok := afterSpecs[id]; !ok {
			summary.NodeChanges = append(summary.NodeChanges, NodeChange{
				ID: id, Kind: "removed", OldHash: b.Hash,
			})
		}
	}

	beforeMarkers := make(map[string]core.Marker, len(before.Markers))
	for _, m := range before.Markers {
		beforeMarkers[m.ID] = m
	}
	afterMarkers := make(map[string]core.Marker, len(after.Markers))
	for _, m := range after.Markers {
		afterMarkers[m.ID] = m
	}
	for id, a := range afterMarkers {
		if b, ok := beforeMarkers[id]; ok {
			if b.Hash != a.Hash {
				summary.NodeChanges = append(summary.NodeChanges, NodeChange{
					ID: id, Kind: "changed", OldHash: b.Hash, NewHash: a.Hash,
				})
			}
		} else {
			summary.NodeChanges = append(summary.NodeChanges, NodeChange{
				ID: id, Kind: "added", NewHash: a.Hash,
			})
		}
	}
	for id, b := range beforeMarkers {
		if _, ok := afterMarkers[id]; !ok {
			summary.NodeChanges = append(summary.NodeChanges, NodeChange{
				ID: id, Kind: "removed", OldHash: b.Hash,
			})
		}
	}

	beforeEdgeSet := make(map[string]bool, len(before.Edges))
	for _, e := range before.Edges {
		beforeEdgeSet[e.From+"\x00"+e.To] = true
	}
	afterEdgeSet := make(map[string]bool, len(after.Edges))
	for _, e := range after.Edges {
		afterEdgeSet[e.From+"\x00"+e.To] = true
	}
	for _, e := range after.Edges {
		if !beforeEdgeSet[e.From+"\x00"+e.To] {
			summary.EdgeChanges = append(summary.EdgeChanges, EdgeChange{
				From: e.From, To: e.To, Kind: "added",
			})
		}
	}
	for _, e := range before.Edges {
		if !afterEdgeSet[e.From+"\x00"+e.To] {
			summary.EdgeChanges = append(summary.EdgeChanges, EdgeChange{
				From: e.From, To: e.To, Kind: "removed",
			})
		}
	}

	return summary
}

// D! id=ounlnk range-end

// D! id=orspc range-start
func reconcileSpecs(baselined []core.Spec, scanned []core.Spec) ([]core.Spec, error) {
	baselinedByID := make(map[string]core.Spec, len(baselined))
	for _, s := range baselined {
		baselinedByID[s.ID] = s
	}

	scannedByID := make(map[string]bool, len(scanned))
	for _, s := range scanned {
		scannedByID[s.ID] = true
	}

	result := make([]core.Spec, 0, len(scanned)+len(baselined))
	for _, s := range scanned {
		if b, ok := baselinedByID[s.ID]; ok {
			result = append(result, core.Spec{
				ID:         s.ID,
				Hash:       b.Hash,
				Filepath:   s.Filepath,
				LineNumber: s.LineNumber,
				Module:     s.Module,
			})
		} else {
			// New spec in scan only — no baseline hash yet. Set Hash="" so
			// DeriveClosures detects it as a NODE_CHANGED event (baseline
			// empty vs scan non-empty).
			result = append(result, core.Spec{
				ID:         s.ID,
				Hash:       "",
				Filepath:   s.Filepath,
				LineNumber: s.LineNumber,
				Module:     s.Module,
			})
		}
	}
	for id, p := range baselinedByID {
		if !scannedByID[id] {
			result = append(result, core.Spec{
				ID:         p.ID,
				Hash:       p.Hash,
				Filepath:   p.Filepath,
				LineNumber: p.LineNumber,
				Module:     p.Module,
				Deleted:    true,
			})
		}
	}
	return result, nil
}

// D! id=orspc range-end

// D! id=ormrk range-start
func reconcileMarkers(baselined []core.Marker, scanned []core.Marker) ([]core.Marker, error) {
	baselinedByID := make(map[string]core.Marker, len(baselined))
	for _, m := range baselined {
		baselinedByID[m.ID] = m
	}

	scannedByID := make(map[string]bool, len(scanned))
	for _, m := range scanned {
		scannedByID[m.ID] = true
	}

	result := make([]core.Marker, 0, len(scanned)+len(baselined))
	for _, m := range scanned {
		if b, ok := baselinedByID[m.ID]; ok {
			result = append(result, core.Marker{
				ID:            m.ID,
				Hash:          b.Hash,
				Filepath:      m.Filepath,
				LineNumber:    m.LineNumber,
				EndLineNumber: m.EndLineNumber,
			})
		} else {
			// New marker in scan only — no baseline hash yet.
			result = append(result, core.Marker{
				ID:            m.ID,
				Hash:          "",
				Filepath:      m.Filepath,
				LineNumber:    m.LineNumber,
				EndLineNumber: m.EndLineNumber,
			})
		}
	}
	for id, p := range baselinedByID {
		if !scannedByID[id] {
			result = append(result, core.Marker{
				ID:            p.ID,
				Hash:          p.Hash,
				Filepath:      p.Filepath,
				LineNumber:    p.LineNumber,
				EndLineNumber: p.EndLineNumber,
				Deleted:       true,
			})
		}
	}
	return result, nil
}

// D! id=ormrk range-end

// D! id=odiff range-start
// DiffClosure returns per-node diff results for every node referenced by
// the closure: each spec node produces a DiffResult with Spec set, each
// marker node produces a DiffResult with Marker set.
func (o *Orchestrator) DiffClosure(sess *fileio.Session, hash string) ([]DiffResult, error) {
	state, err := o.stateStore.Load(sess)
	if err != nil {
		return nil, err
	}

	scanResult, err := o.scanner.Scan()
	if err != nil {
		return nil, err
	}

	reconciledSpecs, err := reconcileSpecs(state.Specs, scanResult.Specs)
	if err != nil {
		return nil, err
	}
	reconciledMarkers, err := reconcileMarkers(state.Markers, scanResult.Markers)
	if err != nil {
		return nil, err
	}

	scan := buildScan(scanResult, reconciledSpecs, reconciledMarkers)
	closures := core.DeriveClosures(reconciledSpecs, reconciledMarkers, state.Edges, scan)

	var target *core.Closure
	for i := range closures {
		if closures[i].Hash == hash {
			target = &closures[i]
			break
		}
	}
	if target == nil {
		return nil, ErrDiffClosureNotFound
	}

	seen := map[string]bool{}
	seedSet := make(map[string]bool, len(target.Seeds))
	for _, s := range target.Seeds {
		seedSet[s] = true
	}
	var out []DiffResult
	for _, n := range target.Nodes {
		if seen[n.ID] {
			continue
		}
		seen[n.ID] = true
		isSeed := seedSet[n.ID]
		if n.IsSpec {
			side := o.buildSpecDiffSide(n.ID, reconciledSpecs, scanResult)
			if side != nil {
				out = append(out, DiffResult{Spec: side, IsSeed: isSeed})
			}
		} else {
			side := o.buildMarkerDiffSide(n.ID, reconciledMarkers, scanResult)
			if side != nil {
				out = append(out, DiffResult{Marker: side, IsSeed: isSeed})
			}
		}
	}
	return out, nil
}

// DiffAll returns per-closure diff results, one entry per closure.
type ClosureDiff struct {
	Hash   string
	Events []core.DriftEvent
	Diffs  []DiffResult
}

func (o *Orchestrator) DiffAll(sess *fileio.Session) ([]ClosureDiff, core.EvaluatedState, error) {
	state, err := o.stateStore.Load(sess)
	if err != nil {
		return nil, core.EvaluatedState{}, err
	}

	scanResult, err := o.scanner.Scan()
	if err != nil {
		return nil, core.EvaluatedState{}, err
	}

	reconciledSpecs, err := reconcileSpecs(state.Specs, scanResult.Specs)
	if err != nil {
		return nil, core.EvaluatedState{}, err
	}
	reconciledMarkers, err := reconcileMarkers(state.Markers, scanResult.Markers)
	if err != nil {
		return nil, core.EvaluatedState{}, err
	}

	scan := buildScan(scanResult, reconciledSpecs, reconciledMarkers)
	closures := core.DeriveClosures(reconciledSpecs, reconciledMarkers, state.Edges, scan)

	evaluated := core.EvaluatedState{
		Specs:    reconciledSpecs,
		Markers:  reconciledMarkers,
		Edges:    state.Edges,
		Closures: closures,
	}

	out := make([]ClosureDiff, 0, len(closures))
	for _, c := range closures {
		var diffs []DiffResult
		seen := map[string]bool{}
		seedSet := make(map[string]bool, len(c.Seeds))
		for _, s := range c.Seeds {
			seedSet[s] = true
		}
		for _, n := range c.Nodes {
			if seen[n.ID] {
				continue
			}
			seen[n.ID] = true
			isSeed := seedSet[n.ID]
			if n.IsSpec {
				if side := o.buildSpecDiffSide(n.ID, reconciledSpecs, scanResult); side != nil {
					diffs = append(diffs, DiffResult{Spec: side, IsSeed: isSeed})
				}
			} else {
				if side := o.buildMarkerDiffSide(n.ID, reconciledMarkers, scanResult); side != nil {
					diffs = append(diffs, DiffResult{Marker: side, IsSeed: isSeed})
				}
			}
		}
		out = append(out, ClosureDiff{
			Hash:   c.Hash,
			Events: c.Events,
			Diffs:  diffs,
		})
	}
	return out, evaluated, nil
}

func (o *Orchestrator) buildSpecDiffSide(specID string, reconciledSpecs []core.Spec, scanResult scanner.ScanResult) *DiffSide {
	var spec *core.Spec
	for i := range reconciledSpecs {
		if reconciledSpecs[i].ID == specID {
			spec = &reconciledSpecs[i]
			break
		}
	}
	if spec == nil {
		return nil
	}
	side := &DiffSide{
		ID:           spec.ID,
		Filepath:     spec.Filepath,
		BaselineHash: spec.Hash,
		CurrentHash:  scanHashForSpec(scanResult, spec.ID),
		Deleted:      spec.Deleted,
	}
	if !spec.Deleted {
		if content, err := scanner.ReadSpecContent(o.resolvePath(spec.Filepath), spec.ID); err == nil {
			side.Current = content
		}
	}
	if o.baselines != nil {
		if content, ok := o.baselines.Read(spec.Hash); ok {
			side.Baseline = content
			side.HasBaseline = true
		}
	}
	return side
}

func (o *Orchestrator) buildMarkerDiffSide(markerID string, reconciledMarkers []core.Marker, scanResult scanner.ScanResult) *DiffSide {
	var marker *core.Marker
	for i := range reconciledMarkers {
		if reconciledMarkers[i].ID == markerID {
			marker = &reconciledMarkers[i]
			break
		}
	}
	if marker == nil {
		return nil
	}
	side := &DiffSide{
		ID:           marker.ID,
		Filepath:     marker.Filepath,
		Lines:        fmt.Sprintf("%d-%d", marker.LineNumber, marker.EndLineNumber),
		BaselineHash: marker.Hash,
		CurrentHash:  scanHashForMarker(scanResult, marker.ID),
		Deleted:      marker.Deleted,
	}
	if !marker.Deleted {
		if content, err := scanner.ReadMarkerContent(o.resolvePath(marker.Filepath), marker.LineNumber, marker.EndLineNumber); err == nil {
			side.Current = content
		}
	}
	if o.baselines != nil {
		if content, ok := o.baselines.Read(marker.Hash); ok {
			side.Baseline = content
			side.HasBaseline = true
		}
	}
	return side
}

func scanHashForSpec(scanResult scanner.ScanResult, id string) string {
	for _, s := range scanResult.Specs {
		if s.ID == id {
			return s.Hash
		}
	}
	return ""
}

func scanHashForMarker(scanResult scanner.ScanResult, id string) string {
	for _, m := range scanResult.Markers {
		if m.ID == id {
			return m.Hash
		}
	}
	return ""
}

// D! id=odiff range-end

// mergeScannedEdges returns baseline edges with all spec-spec edges replaced
// by the scan's spec-spec edges. Link-style edges (marker-spec) are preserved
// from baseline because they are user-curated, not auto-discovered.
func mergeScannedEdges(baselineEdges, scanEdges []core.Edge) []core.Edge {
	out := make([]core.Edge, 0, len(baselineEdges)+len(scanEdges))
	for _, e := range baselineEdges {
		if !isSpecID(e.From) {
			out = append(out, e)
		}
	}
	for _, e := range scanEdges {
		if isSpecID(e.From) && isSpecID(e.To) {
			out = append(out, e)
		}
	}
	return out
}

func isSpecID(id string) bool {
	first := strings.Index(id, ".")
	if first < 0 {
		return false
	}
	return strings.Index(id[first+1:], ".") < 0
}

func buildScan(scanResult scanner.ScanResult, reconciledSpecs []core.Spec, reconciledMarkers []core.Marker) core.Scan {
	specHashes := make(map[string]string, len(scanResult.Specs))
	for _, s := range scanResult.Specs {
		specHashes[s.ID] = s.Hash
	}
	scannedSpecIDs := make(map[string]bool, len(scanResult.Specs))
	for _, s := range scanResult.Specs {
		scannedSpecIDs[s.ID] = true
	}
	for _, s := range reconciledSpecs {
		if !scannedSpecIDs[s.ID] {
			specHashes[s.ID] = ""
		}
	}

	markerHashes := make(map[string]string, len(scanResult.Markers))
	for _, m := range scanResult.Markers {
		markerHashes[m.ID] = m.Hash
	}
	scannedMarkerIDs := make(map[string]bool, len(scanResult.Markers))
	for _, m := range scanResult.Markers {
		scannedMarkerIDs[m.ID] = true
	}
	for _, m := range reconciledMarkers {
		if !scannedMarkerIDs[m.ID] {
			markerHashes[m.ID] = ""
		}
	}

	return core.Scan{
		SpecHashes:   specHashes,
		MarkerHashes: markerHashes,
		Edges:        scanResult.Edges,
	}
}
