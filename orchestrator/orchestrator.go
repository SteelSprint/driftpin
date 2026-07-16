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
	markerSyntax          = "D" + "! id=<shortcode>"
)

type Orchestrator struct {
	pin     pinstore.PinStore
	scanner scanner.Scanner
	core    *core.CoreAlgorithm
}

func NewOrchestrator(pin pinstore.PinStore, scanner scanner.Scanner) *Orchestrator {
	return &Orchestrator{
		pin:     pin,
		scanner: scanner,
		core:    core.NewCoreAlgorithm(),
	}
}

// D! id=oinit
func (o *Orchestrator) Init() error {
	return o.pin.Save(pinstore.PinState{})
}

// D! id=otodo
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

// D! id=orest
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

	return evaluated, nil
}

// D! id=crorph
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

// D! id=olink
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

	// D! id=cperr
	markerExists := false
	for _, m := range reconciledMarkers {
		if m.ID == markerID {
			markerExists = true
			break
		}
	}
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

	return o.pin.Save(pinstore.PinState{
		Specs:           reconciledSpecs,
		Markers:         reconciledMarkers,
		Links:           append(state.Links, core.Link{SpecID: specID, MarkerID: markerID}),
		ResolutionState: state.ResolutionState,
	})
}

// D! id=ounlnk
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

// D! id=orspc
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

// D! id=ormrk
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
				ID:         m.ID,
				Hash:       pinned.Hash,
				Filepath:   m.Filepath,
				LineNumber: m.LineNumber,
			})
		} else {
			result = append(result, m)
		}
	}
	for id, p := range pinnedByID {
		if !scannedByID[id] {
			result = append(result, core.Marker{
				ID:         p.ID,
				Hash:       p.Hash,
				Filepath:   p.Filepath,
				LineNumber: p.LineNumber,
				Deleted:    true,
			})
		}
	}
	return result, nil
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
	}
}
