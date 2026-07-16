package orchestrator

import (
	"fmt"
	"strings"

	"driftpin/core"
	"driftpin/pinstore"
	"driftpin/scanner"
)

var (
	ErrLinkMarkerNotFound = fmt.Errorf("link references unknown marker")
	ErrLinkSpecNotFound   = fmt.Errorf("link references unknown spec")
	ErrLinkAlreadyExists  = fmt.Errorf("link already exists")
	ErrUnlinkNotFound     = fmt.Errorf("no link found between marker and spec")
	ErrOrphanNotFound     = fmt.Errorf("no spec or marker found in drift.pin")
	ErrOrphanStillOnDisk  = fmt.Errorf("spec or marker is still on disk")
	ErrOrphanHasLinks     = fmt.Errorf("spec or marker still has links")
	ErrDiffEntityNotFound = fmt.Errorf("no spec or marker found for diff")
	markerSyntax          = "D" + "! id=<shortcode>"
)

type Orchestrator struct {
	pin       pinstore.PinStore
	scanner   scanner.Scanner
	core      *core.CoreAlgorithm
	baselines *pinstore.BaselineStore
}

func NewOrchestrator(pin pinstore.PinStore, scanner scanner.Scanner, baselines *pinstore.BaselineStore) *Orchestrator {
	return &Orchestrator{
		pin:       pin,
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
// spec or marker using its current scanned hash. Best-effort: if the
// BaselineStore is nil (e.g. in tests that don't exercise diff), this
// is a no-op. The scanned hash always equals sha1(current content), so
// the integrity check in BaselineStore.Write is satisfied. For entities
// whose pinned hash differs from the scanned hash (drifted), this creates
// an orphan file at the scanned-hash address — harmless and dedup-safe.
func (o *Orchestrator) writeBaseline(scannedHash, filepath, specID string, startLine, endLine int, isSpec bool) error {
	if o.baselines == nil {
		return nil
	}
	var content string
	var err error
	if isSpec {
		content, err = scanner.ReadSpecContent(filepath, specID)
	} else {
		content, err = scanner.ReadMarkerContent(filepath, startLine, endLine)
	}
	if err != nil {
		return nil // entity may be deleted mid-operation; skip silently
	}
	return o.baselines.Write(scannedHash, content)
}

// D! id=oinit range-start
func (o *Orchestrator) Init() error {
	return o.pin.Save(pinstore.PinState{})
}

// D! id=oinit range-end

// D! id=otodo range-start
func (o *Orchestrator) Todo() (core.EvaluatedState, error) {
	state, err := o.pin.Load()
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
		Specs:           reconciledSpecs,
		Markers:         reconciledMarkers,
		Links:           state.Links,
		ResolutionState: state.ResolutionState,
		Action:          core.TodoAction{Scan: scan},
	}

	return o.core.EvaluateState(ctx)
}

// D! id=otodo range-end

// D! id=orest range-start
func (o *Orchestrator) Reset(markerID, specID string) (core.EvaluatedState, error) {
	state, err := o.pin.Load()
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
		Specs:           reconciledSpecs,
		Markers:         reconciledMarkers,
		Links:           state.Links,
		ResolutionState: state.ResolutionState,
		Action: core.ResetAction{
			SpecID:   specID,
			MarkerID: markerID,
			Scan:     scan,
		},
	}

	evaluated, err := o.core.EvaluateState(ctx)
	if err != nil {
		return core.EvaluatedState{}, err
	}

	err = o.pin.Save(pinstore.PinState{
		Specs:           evaluated.Specs,
		Markers:         evaluated.Markers,
		Links:           evaluated.Links,
		ResolutionState: evaluated.ResolutionState,
	})
	if err != nil {
		return core.EvaluatedState{}, err
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

	return evaluated, nil
}

// D! id=orest range-end

// D! id=crorph range-start
func (o *Orchestrator) ResetOrphan(id string) error {
	state, err := o.pin.Load()
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
		linkCount := 0
		for _, l := range state.Links {
			if l.SpecID == id {
				linkCount++
			}
		}
		if linkCount > 0 {
			return fmt.Errorf("%w: %q still has %d link(s); resolve them first with `drift reset <marker> <spec>`", ErrOrphanHasLinks, id, linkCount)
		}
		newSpecs := make([]core.Spec, 0, len(state.Specs)-1)
		for _, s := range state.Specs {
			if s.ID != id {
				newSpecs = append(newSpecs, s)
			}
		}
		newResolutions := make([]core.ResolutionState, 0, len(state.ResolutionState))
		for _, r := range state.ResolutionState {
			if r.SpecID != id {
				newResolutions = append(newResolutions, r)
			}
		}
		return o.pin.Save(pinstore.PinState{
			Specs:           newSpecs,
			Markers:         state.Markers,
			Links:           state.Links,
			ResolutionState: newResolutions,
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
	linkCount := 0
	for _, l := range state.Links {
		if l.MarkerID == id {
			linkCount++
		}
	}
	if linkCount > 0 {
		return fmt.Errorf("%w: %q still has %d link(s); resolve them first with `drift reset <marker> <spec>`", ErrOrphanHasLinks, id, linkCount)
	}
	newMarkers := make([]core.Marker, 0, len(state.Markers)-1)
	for _, m := range state.Markers {
		if m.ID != id {
			newMarkers = append(newMarkers, m)
		}
	}
	newResolutions := make([]core.ResolutionState, 0, len(state.ResolutionState))
	for _, r := range state.ResolutionState {
		if r.MarkerID != id {
			newResolutions = append(newResolutions, r)
		}
	}
	return o.pin.Save(pinstore.PinState{
		Specs:           state.Specs,
		Markers:         newMarkers,
		Links:           state.Links,
		ResolutionState: newResolutions,
	})
}

// D! id=crorph range-end

// D! id=olink range-start
func (o *Orchestrator) Link(markerID, specID string) error {
	state, err := o.pin.Load()
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

	for _, l := range state.Links {
		if l.MarkerID == markerID && l.SpecID == specID {
			return fmt.Errorf("%w: marker=%q spec=%q", ErrLinkAlreadyExists, markerID, specID)
		}
	}

	if err := o.pin.Save(pinstore.PinState{
		Specs:           reconciledSpecs,
		Markers:         reconciledMarkers,
		Links:           append(state.Links, core.Link{SpecID: specID, MarkerID: markerID}),
		ResolutionState: state.ResolutionState,
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
	state, err := o.pin.Load()
	if err != nil {
		return err
	}

	linkIndex := -1
	for i, l := range state.Links {
		if l.MarkerID == markerID && l.SpecID == specID {
			linkIndex = i
			break
		}
	}
	if linkIndex == -1 {
		return fmt.Errorf("%w: marker=%q spec=%q", ErrUnlinkNotFound, markerID, specID)
	}

	newLinks := make([]core.Link, 0, len(state.Links)-1)
	newLinks = append(newLinks, state.Links[:linkIndex]...)
	newLinks = append(newLinks, state.Links[linkIndex+1:]...)

	newResolutions := make([]core.ResolutionState, 0, len(state.ResolutionState))
	for _, res := range state.ResolutionState {
		if res.MarkerID == markerID && res.SpecID == specID {
			continue
		}
		newResolutions = append(newResolutions, res)
	}

	return o.pin.Save(pinstore.PinState{
		Specs:           state.Specs,
		Markers:         state.Markers,
		Links:           newLinks,
		ResolutionState: newResolutions,
	})
}

// D! id=ounlnk range-end

// D! id=orspc range-start
func reconcileSpecs(pinned []core.Spec, scanned []core.Spec) ([]core.Spec, error) {
	pinnedByID := make(map[string]core.Spec, len(pinned))
	for _, s := range pinned {
		pinnedByID[s.ID] = s
	}

	scannedByID := make(map[string]bool, len(scanned))
	for _, s := range scanned {
		scannedByID[s.ID] = true
	}

	result := make([]core.Spec, 0, len(scanned)+len(pinned))
	for _, s := range scanned {
		if pinned, ok := pinnedByID[s.ID]; ok {
			result = append(result, core.Spec{
				ID:         s.ID,
				Hash:       pinned.Hash,
				Filepath:   s.Filepath,
				LineNumber: s.LineNumber,
				Module:     s.Module,
			})
		} else {
			result = append(result, s)
		}
	}
	for id, p := range pinnedByID {
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
func reconcileMarkers(pinned []core.Marker, scanned []core.Marker) ([]core.Marker, error) {
	pinnedByID := make(map[string]core.Marker, len(pinned))
	for _, m := range pinned {
		pinnedByID[m.ID] = m
	}

	scannedByID := make(map[string]bool, len(scanned))
	for _, m := range scanned {
		scannedByID[m.ID] = true
	}

	result := make([]core.Marker, 0, len(scanned)+len(pinned))
	for _, m := range scanned {
		if pinned, ok := pinnedByID[m.ID]; ok {
			result = append(result, core.Marker{
				ID:            m.ID,
				Hash:          pinned.Hash,
				Filepath:      m.Filepath,
				LineNumber:    m.LineNumber,
				EndLineNumber: m.EndLineNumber,
			})
		} else {
			result = append(result, m)
		}
	}
	for id, p := range pinnedByID {
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
	state, err := o.pin.Load()
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
		if content, err := scanner.ReadSpecContent(spec.Filepath, spec.ID); err == nil {
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
		if content, err := scanner.ReadMarkerContent(marker.Filepath, marker.LineNumber, marker.EndLineNumber); err == nil {
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
	}
}
