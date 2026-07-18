package orchestrator

import (
	"fmt"
	"path/filepath"
	"strings"

	"drift/core"
	"drift/statestore"
	"drift/scanner"
)

var (
	ErrLinkMarkerNotFound    = fmt.Errorf("link references unknown marker")
	ErrLinkSpecNotFound      = fmt.Errorf("link references unknown spec")
	ErrLinkAlreadyExists     = fmt.Errorf("link already exists")
	ErrUnlinkNotFound        = fmt.Errorf("no link found between marker and spec")
	ErrOrphanNotFound        = fmt.Errorf("no spec or marker found in state.xml")
	ErrOrphanStillOnDisk     = fmt.Errorf("spec or marker is still on disk")
	ErrOrphanHasEdges        = fmt.Errorf("spec or marker still has edges")
	ErrDiffEntityNotFound    = fmt.Errorf("no spec or marker found for diff")
	ErrAlreadyInitialized    = fmt.Errorf("project already initialized")
	ErrBrokenEdgeNotResettable = fmt.Errorf("broken edge cannot be resolved by reset; fix the spec text or restore the missing spec")
	markerSyntax             = "D" + "! id=<shortcode>"
)

type Orchestrator struct {
	stateStore statestore.StateStore
	scanner   scanner.Scanner
	core      *core.CoreAlgorithm
	baselines *statestore.BaselineStore
}

func NewOrchestrator(stateStore statestore.StateStore, scanner scanner.Scanner, baselines *statestore.BaselineStore) *Orchestrator {
	return &Orchestrator{
		stateStore:       stateStore,
		scanner:   scanner,
		core:      core.NewCoreAlgorithm(),
		baselines: baselines,
	}
}

// DiffSide describes one side (spec or marker) of a drift edge for diffing.
type DiffSide struct {
	ID           string
	Filepath     string
	Lines        string // "start-end" for markers, "" for specs
	BaselineHash string // hash stored in state.xml (the baseline)
	CurrentHash  string // scanned hash of current on-disk content; "" if deleted
	Baseline     string // baseline content; "" if no snapshot
	Current      string // current on-disk content; "" if deleted
	HasBaseline  bool   // false when no baseline snapshot exists
	Deleted      bool   // true when the entity was removed from disk
}

// DiffResult holds both sides of a drift edge.
type DiffResult struct {
	Spec   DiffSide
	Marker DiffSide
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
func (o *Orchestrator) Init() error {
	initialized, err := o.stateStore.Initialized()
	if err != nil {
		return fmt.Errorf("check initialized state: %w", err)
	}
	if initialized {
		return ErrAlreadyInitialized
	}
	return o.stateStore.Save(statestore.State{})
}

// D! id=oinit range-end

// D! id=otodo range-start
func (o *Orchestrator) Todo() (core.EvaluatedState, error) {
	state, err := o.stateStore.Load()
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
		Specs:       reconciledSpecs,
		Markers:     reconciledMarkers,
		Edges:       state.Edges,
		Resolutions: state.Resolutions,
		Action:      core.TodoAction{Scan: scan},
	}

	return o.core.EvaluateState(ctx)
}

// D! id=otodo range-end

// D! id=orest range-start
// Reset resolves one drifted edge. The from/to arguments name the edge
// endpoints; the CLI dispatches based on dots in the IDs (marker-spec for
// link-style, spec-spec for ref-style). The unified core path finds the
// edge in baseline (in either direction), stamps an EdgeResolution at the
// current endpoint hashes, and collapses any nodes whose every edge is
// consistent or resolved.
func (o *Orchestrator) Reset(fromID, toID string) (core.EvaluatedState, error) {
	unlock, err := o.stateStore.Lock()
	if err != nil {
		return core.EvaluatedState{}, err
	}
	defer unlock()

	state, err := o.stateStore.Load()
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

	// Spec-spec ref-edge reset (both args have dots): if the edge exists in
	// scan but not baseline (TodoEdgeAdded), add it to baseline; if it exists
	// in baseline but not scan (TodoEdgeRemoved), drop it from baseline and
	// prune its resolutions. Otherwise stamp a resolution and let collapse
	// handle it.
	edges := state.Edges
	if isSpecID(fromID) && isSpecID(toID) {
		// Refuse broken-edge resets: target spec must exist on disk.
		specExists := false
		for _, s := range scanResult.Specs {
			if s.ID == toID {
				specExists = true
				break
			}
		}
		if !specExists {
			return core.EvaluatedState{}, fmt.Errorf("%w: from=%q to=%q", ErrBrokenEdgeNotResettable, fromID, toID)
		}
		edges = applyRefEdgeReset(state.Edges, scanResult.Edges, fromID, toID)
	}

	ctx := core.CoreAlgorithmContext{
		Specs:       reconciledSpecs,
		Markers:     reconciledMarkers,
		Edges:       edges,
		Resolutions: state.Resolutions,
		Action: core.ResetAction{
			From: fromID,
			To:   toID,
			Scan: scan,
		},
	}

	evaluated, err := o.core.EvaluateState(ctx)
	if err != nil {
		return core.EvaluatedState{}, err
	}

	// For ref-edge removal, drop resolutions touching the removed edge in
	// either direction (Validate would otherwise reject the next Load).
	if isSpecID(fromID) && isSpecID(toID) {
		scanHasEdge := false
		for _, e := range scanResult.Edges {
			if (e.From == fromID && e.To == toID) || (e.From == toID && e.To == fromID) {
				scanHasEdge = true
				break
			}
		}
		if !scanHasEdge {
			evaluated.Resolutions = pruneResolutionsTouchingPair(evaluated.Resolutions, fromID, toID)
		}
	}

	err = o.stateStore.Save(statestore.State{
		Specs:       evaluated.Specs,
		Markers:     evaluated.Markers,
		Edges:       evaluated.Edges,
		Resolutions: evaluated.Resolutions,
	})
	if err != nil {
		return core.EvaluatedState{}, err
	}

	// Best-effort: refresh baseline files for the resolved endpoints.
	for _, s := range scanResult.Specs {
		if s.ID == fromID || s.ID == toID {
			_ = o.writeBaseline(s.Hash, s.Filepath, s.ID, 0, 0, true)
		}
	}
	for _, m := range scanResult.Markers {
		if m.ID == fromID || m.ID == toID {
			_ = o.writeBaseline(m.Hash, m.Filepath, "", m.LineNumber, m.EndLineNumber, false)
		}
	}

	return evaluated, nil
}

// D! id=orest range-end

// D! id=crorph range-start
func (o *Orchestrator) ResetOrphan(id string) error {
	unlock, err := o.stateStore.Lock()
	if err != nil {
		return err
	}
	defer unlock()

	state, err := o.stateStore.Load()
	if err != nil {
		return err
	}

	scanResult, err := o.scanner.Scan()
	if err != nil {
		return err
	}

	scannedSpecIDs := make(map[string]bool, len(scanResult.Specs))
	for _, s := range scanResult.Specs {
		scannedSpecIDs[s.ID] = true
	}
	scannedMarkerIDs := make(map[string]bool, len(scanResult.Markers))
	for _, m := range scanResult.Markers {
		scannedMarkerIDs[m.ID] = true
	}

	isSpec := strings.Contains(id, ".")

	if isSpec {
		specFound := false
		for _, s := range state.Specs {
			if s.ID == id {
				specFound = true
				break
			}
		}
		if !specFound {
			return fmt.Errorf("%w: %q", ErrOrphanNotFound, id)
		}
		if scannedSpecIDs[id] {
			return fmt.Errorf("%w: %q", ErrOrphanStillOnDisk, id)
		}
		edgeCount := 0
		for _, e := range state.Edges {
			if e.From == id || e.To == id {
				edgeCount++
			}
		}
		if edgeCount > 0 {
			return fmt.Errorf("%w: %q still has %d edge(s); resolve them first with `drift reset`", ErrOrphanHasEdges, id, edgeCount)
		}
		newSpecs := make([]core.Spec, 0, len(state.Specs)-1)
		for _, s := range state.Specs {
			if s.ID != id {
				newSpecs = append(newSpecs, s)
			}
		}
		// Prune any resolutions touching the orphaned spec.
		newResolutions := make([]core.EdgeResolution, 0, len(state.Resolutions))
		for _, r := range state.Resolutions {
			if r.From == id || r.To == id {
				continue
			}
			newResolutions = append(newResolutions, r)
		}
		return o.stateStore.Save(statestore.State{
			Specs:       newSpecs,
			Markers:     state.Markers,
			Edges:       state.Edges,
			Resolutions: newResolutions,
		})
	}

	markerFound := false
	for _, m := range state.Markers {
		if m.ID == id {
			markerFound = true
			break
		}
	}
	if !markerFound {
		return fmt.Errorf("%w: %q", ErrOrphanNotFound, id)
	}
	if scannedMarkerIDs[id] {
		return fmt.Errorf("%w: %q", ErrOrphanStillOnDisk, id)
	}
	edgeCount := 0
	for _, e := range state.Edges {
		if e.From == id || e.To == id {
			edgeCount++
		}
	}
	if edgeCount > 0 {
		return fmt.Errorf("%w: %q still has %d edge(s); resolve them first with `drift reset`", ErrOrphanHasEdges, id, edgeCount)
	}
	newMarkers := make([]core.Marker, 0, len(state.Markers)-1)
	for _, m := range state.Markers {
		if m.ID != id {
			newMarkers = append(newMarkers, m)
		}
	}
	newResolutions := make([]core.EdgeResolution, 0, len(state.Resolutions))
	for _, r := range state.Resolutions {
		if r.From == id || r.To == id {
			continue
		}
		newResolutions = append(newResolutions, r)
	}
	return o.stateStore.Save(statestore.State{
		Specs:       state.Specs,
		Markers:     newMarkers,
		Edges:       state.Edges,
		Resolutions: newResolutions,
	})
}

// D! id=crorph range-end

// D! id=olink range-start
// Link constructs a link-style Edge (marker stores edge to spec) and appends
// it to baseline. The edge kind is implicit from endpoint types: marker IDs
// contain no dot, spec IDs contain exactly one.
func (o *Orchestrator) Link(markerID, specID string) error {
	unlock, err := o.stateStore.Lock()
	if err != nil {
		return err
	}
	defer unlock()

	state, err := o.stateStore.Load()
	if err != nil {
		return err
	}

	scanResult, err := o.scanner.Scan()
	if err != nil {
		return err
	}

	reconciledSpecs, err := reconcileSpecs(state.Specs, scanResult.Specs)
	if err != nil {
		return err
	}

	reconciledMarkers, err := reconcileMarkers(state.Markers, scanResult.Markers)
	if err != nil {
		return err
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
		return fmt.Errorf("link references unknown marker %q.\nMarkers must be %s comment lines in code files.\nAvailable markers: %s", markerID, markerSyntax, strings.Join(available, ", "))
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
		return fmt.Errorf("link references unknown spec %q.\nSpec IDs are module-qualified: <module>.<specId> (e.g. main.example or core.validate).\nAvailable specs: %s", specID, strings.Join(available, ", "))
	}

	for _, e := range state.Edges {
		if e.From == markerID && e.To == specID {
			return fmt.Errorf("%w: marker=%q spec=%q", ErrLinkAlreadyExists, markerID, specID)
		}
	}

	// Merge scanned ref-edges (spec-spec) into the baseline so the new
	// state.xml carries both link-style and ref-style edges.
	mergedEdges := mergeScannedEdges(state.Edges, scanResult.Edges)

	if err := o.stateStore.Save(statestore.State{
		Specs:       reconciledSpecs,
		Markers:     reconciledMarkers,
		Edges:       append(mergedEdges, core.Edge{From: markerID, To: specID}),
		Resolutions: state.Resolutions,
	}); err != nil {
		return err
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
	return nil
}

// D! id=olink range-end

// D! id=ounlnk range-start
func (o *Orchestrator) Unlink(markerID, specID string) error {
	unlock, err := o.stateStore.Lock()
	if err != nil {
		return err
	}
	defer unlock()

	state, err := o.stateStore.Load()
	if err != nil {
		return err
	}

	edgeIndex := -1
	for i, e := range state.Edges {
		if e.From == markerID && e.To == specID {
			edgeIndex = i
			break
		}
	}
	if edgeIndex == -1 {
		return fmt.Errorf("%w: marker=%q spec=%q", ErrUnlinkNotFound, markerID, specID)
	}

	newEdges := make([]core.Edge, 0, len(state.Edges)-1)
	newEdges = append(newEdges, state.Edges[:edgeIndex]...)
	newEdges = append(newEdges, state.Edges[edgeIndex+1:]...)

	newResolutions := make([]core.EdgeResolution, 0, len(state.Resolutions))
	for _, res := range state.Resolutions {
		if (res.From == markerID && res.To == specID) ||
			(res.From == specID && res.To == markerID) {
			continue
		}
		newResolutions = append(newResolutions, res)
	}

	return o.stateStore.Save(statestore.State{
		Specs:       state.Specs,
		Markers:     state.Markers,
		Edges:       newEdges,
		Resolutions: newResolutions,
	})
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
			result = append(result, s)
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
			result = append(result, m)
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
func (o *Orchestrator) Diff(markerID, specID string) (DiffResult, error) {
	state, err := o.stateStore.Load()
	if err != nil {
		return DiffResult{}, err
	}

	scanResult, err := o.scanner.Scan()
	if err != nil {
		return DiffResult{}, err
	}

	reconciledSpecs, err := reconcileSpecs(state.Specs, scanResult.Specs)
	if err != nil {
		return DiffResult{}, err
	}
	reconciledMarkers, err := reconcileMarkers(state.Markers, scanResult.Markers)
	if err != nil {
		return DiffResult{}, err
	}

	scannedSpecHashes := make(map[string]string, len(scanResult.Specs))
	for _, s := range scanResult.Specs {
		scannedSpecHashes[s.ID] = s.Hash
	}
	scannedMarkerHashes := make(map[string]string, len(scanResult.Markers))
	for _, m := range scanResult.Markers {
		scannedMarkerHashes[m.ID] = m.Hash
	}

	var spec *core.Spec
	for i := range reconciledSpecs {
		if reconciledSpecs[i].ID == specID {
			spec = &reconciledSpecs[i]
			break
		}
	}
	var marker *core.Marker
	for i := range reconciledMarkers {
		if reconciledMarkers[i].ID == markerID {
			marker = &reconciledMarkers[i]
			break
		}
	}
	if spec == nil || marker == nil {
		return DiffResult{}, fmt.Errorf("%w: marker=%q spec=%q", ErrDiffEntityNotFound, markerID, specID)
	}

	result := DiffResult{}

	result.Spec = DiffSide{
		ID:           spec.ID,
		Filepath:     spec.Filepath,
		BaselineHash: spec.Hash,
		CurrentHash:  scannedSpecHashes[spec.ID],
		Deleted:      spec.Deleted,
	}
	if !spec.Deleted {
		if content, err := scanner.ReadSpecContent(o.resolvePath(spec.Filepath), spec.ID); err == nil {
			result.Spec.Current = content
		}
	}
	if o.baselines != nil {
		if content, ok := o.baselines.Read(spec.Hash); ok {
			result.Spec.Baseline = content
			result.Spec.HasBaseline = true
		}
	}

	result.Marker = DiffSide{
		ID:           marker.ID,
		Filepath:     marker.Filepath,
		Lines:        fmt.Sprintf("%d-%d", marker.LineNumber, marker.EndLineNumber),
		BaselineHash: marker.Hash,
		CurrentHash:  scannedMarkerHashes[marker.ID],
		Deleted:      marker.Deleted,
	}
	if !marker.Deleted {
		if content, err := scanner.ReadMarkerContent(o.resolvePath(marker.Filepath), marker.LineNumber, marker.EndLineNumber); err == nil {
			result.Marker.Current = content
		}
	}
	if o.baselines != nil {
		if content, ok := o.baselines.Read(marker.Hash); ok {
			result.Marker.Baseline = content
			result.Marker.HasBaseline = true
		}
	}

	return result, nil
}

// D! id=odiff range-end

// applyRefEdgeReset merges baseline edges with the scan result for the
// (fromID, toID) pair: if the edge is new in scan, add it; if it's gone
// from scan, drop it; otherwise leave baseline unchanged.
func applyRefEdgeReset(baselineEdges []core.Edge, scanEdges []core.Edge, fromID, toID string) []core.Edge {
	scanHasEdge := false
	var scanEdge core.Edge
	for _, e := range scanEdges {
		if (e.From == fromID && e.To == toID) || (e.From == toID && e.To == fromID) {
			scanHasEdge = true
			scanEdge = e
			break
		}
	}
	baselineHasEdge := false
	for _, e := range baselineEdges {
		if (e.From == fromID && e.To == toID) || (e.From == toID && e.To == fromID) {
			baselineHasEdge = true
			break
		}
	}
	switch {
	case scanHasEdge && !baselineHasEdge:
		return append(append([]core.Edge{}, baselineEdges...), scanEdge)
	case !scanHasEdge && baselineHasEdge:
		out := make([]core.Edge, 0, len(baselineEdges))
		for _, e := range baselineEdges {
			if (e.From == fromID && e.To == toID) || (e.From == toID && e.To == fromID) {
				continue
			}
			out = append(out, e)
		}
		return out
	default:
		return baselineEdges
	}
}

// pruneResolutionsTouchingPair drops resolutions referencing either
// direction of the (a, b) pair. Used when a ref-edge is removed and the
// resolutions would otherwise dangle.
func pruneResolutionsTouchingPair(resolutions []core.EdgeResolution, a, b string) []core.EdgeResolution {
	out := make([]core.EdgeResolution, 0, len(resolutions))
	for _, r := range resolutions {
		if (r.From == a && r.To == b) || (r.From == b && r.To == a) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// mergeScannedEdges returns baseline edges with all spec-spec edges replaced
// by the scan's spec-spec edges. Link-style edges (marker-spec) are preserved
// from baseline because they are user-curated, not auto-discovered.
func mergeScannedEdges(baselineEdges, scanEdges []core.Edge) []core.Edge {
	out := make([]core.Edge, 0, len(baselineEdges)+len(scanEdges))
	// Keep all baseline link-style edges (marker → spec).
	for _, e := range baselineEdges {
		if !isSpecID(e.From) {
			out = append(out, e)
		}
	}
	// Append all scan spec-spec edges (ref-style).
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
