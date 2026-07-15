package driftpin

import (
	"fmt"
)

var (
	ErrSpecNotFoundOnDisk    = fmt.Errorf("spec in drift.pin not found on disk")
	ErrMarkerNotFoundOnDisk  = fmt.Errorf("marker in drift.pin not found on disk")
	ErrLinkMarkerNotFound    = fmt.Errorf("link references unknown marker")
	ErrLinkSpecNotFound      = fmt.Errorf("link references unknown spec")
	ErrLinkAlreadyExists     = fmt.Errorf("link already exists")
)

type Orchestrator struct {
	pin     PinStore
	scanner Scanner
	core    *CoreAlgorithm
}

func NewOrchestrator(pin PinStore, scanner Scanner) *Orchestrator {
	return &Orchestrator{
		pin:     pin,
		scanner: scanner,
		core:    NewCoreAlgorithm(),
	}
}

func (o *Orchestrator) Init() error {
	return o.pin.Save(PinState{})
}

func (o *Orchestrator) Todo() (EvaluatedState, error) {
	state, err := o.pin.Load()
	if err != nil {
		return EvaluatedState{}, err
	}

	scanResult, err := o.scanner.Scan()
	if err != nil {
		return EvaluatedState{}, err
	}

	reconciledSpecs, err := reconcileSpecs(state.Specs, scanResult.Specs)
	if err != nil {
		return EvaluatedState{}, err
	}

	reconciledMarkers, err := reconcileMarkers(state.Markers, scanResult.Markers)
	if err != nil {
		return EvaluatedState{}, err
	}

	scan := buildScan(scanResult)

	ctx := CoreAlgorithmContext{
		Specs:           reconciledSpecs,
		Markers:         reconciledMarkers,
		Links:           state.Links,
		ResolutionState: state.ResolutionState,
		Action:          TodoAction{Scan: scan},
	}

	return o.core.EvaluateState(ctx)
}

func (o *Orchestrator) Reset(markerID, specID string) (EvaluatedState, error) {
	state, err := o.pin.Load()
	if err != nil {
		return EvaluatedState{}, err
	}

	scanResult, err := o.scanner.Scan()
	if err != nil {
		return EvaluatedState{}, err
	}

	reconciledSpecs, err := reconcileSpecs(state.Specs, scanResult.Specs)
	if err != nil {
		return EvaluatedState{}, err
	}

	reconciledMarkers, err := reconcileMarkers(state.Markers, scanResult.Markers)
	if err != nil {
		return EvaluatedState{}, err
	}

	scan := buildScan(scanResult)

	ctx := CoreAlgorithmContext{
		Specs:           reconciledSpecs,
		Markers:         reconciledMarkers,
		Links:           state.Links,
		ResolutionState: state.ResolutionState,
		Action: ResetAction{
			SpecID:   specID,
			MarkerID: markerID,
			Scan:     scan,
		},
	}

	evaluated, err := o.core.EvaluateState(ctx)
	if err != nil {
		return EvaluatedState{}, err
	}

	err = o.pin.Save(PinState{
		Specs:           evaluated.Specs,
		Markers:         evaluated.Markers,
		Links:           evaluated.Links,
		ResolutionState: evaluated.ResolutionState,
	})
	if err != nil {
		return EvaluatedState{}, err
	}

	return evaluated, nil
}

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

	markerExists := false
	for _, m := range reconciledMarkers {
		if m.ID == markerID {
			markerExists = true
			break
		}
	}
	if !markerExists {
		return fmt.Errorf("%w: %q", ErrLinkMarkerNotFound, markerID)
	}

	specExists := false
	for _, s := range reconciledSpecs {
		if s.ID == specID {
			specExists = true
			break
		}
	}
	if !specExists {
		return fmt.Errorf("%w: %q", ErrLinkSpecNotFound, specID)
	}

	for _, l := range state.Links {
		if l.MarkerID == markerID && l.SpecID == specID {
			return fmt.Errorf("%w: marker=%q spec=%q", ErrLinkAlreadyExists, markerID, specID)
		}
	}

	return o.pin.Save(PinState{
		Specs:           reconciledSpecs,
		Markers:         reconciledMarkers,
		Links:           append(state.Links, Link{SpecID: specID, MarkerID: markerID}),
		ResolutionState: state.ResolutionState,
	})
}

func reconcileSpecs(pinned []Spec, scanned []Spec) ([]Spec, error) {
	pinnedByID := make(map[string]Spec, len(pinned))
	for _, s := range pinned {
		pinnedByID[s.ID] = s
	}

	scannedByID := make(map[string]bool, len(scanned))
	for _, s := range scanned {
		scannedByID[s.ID] = true
	}

	for id := range pinnedByID {
		if !scannedByID[id] {
			return nil, fmt.Errorf("%w: %q", ErrSpecNotFoundOnDisk, id)
		}
	}

	result := make([]Spec, len(scanned))
	for i, s := range scanned {
		if pinned, ok := pinnedByID[s.ID]; ok {
			result[i] = Spec{
				ID:         s.ID,
				Hash:       pinned.Hash,
				Filepath:   s.Filepath,
				LineNumber: s.LineNumber,
			}
		} else {
			result[i] = s
		}
	}
	return result, nil
}

func reconcileMarkers(pinned []Marker, scanned []Marker) ([]Marker, error) {
	pinnedByID := make(map[string]Marker, len(pinned))
	for _, m := range pinned {
		pinnedByID[m.ID] = m
	}

	scannedByID := make(map[string]bool, len(scanned))
	for _, m := range scanned {
		scannedByID[m.ID] = true
	}

	for id := range pinnedByID {
		if !scannedByID[id] {
			return nil, fmt.Errorf("%w: %q", ErrMarkerNotFoundOnDisk, id)
		}
	}

	result := make([]Marker, len(scanned))
	for i, m := range scanned {
		if pinned, ok := pinnedByID[m.ID]; ok {
			result[i] = Marker{
				ID:         m.ID,
				Hash:       pinned.Hash,
				Filepath:   m.Filepath,
				LineNumber: m.LineNumber,
			}
		} else {
			result[i] = m
		}
	}
	return result, nil
}

func buildScan(scanResult ScanResult) Scan {
	specHashes := make(map[string]string, len(scanResult.Specs))
	for _, s := range scanResult.Specs {
		specHashes[s.ID] = s.Hash
	}
	markerHashes := make(map[string]string, len(scanResult.Markers))
	for _, m := range scanResult.Markers {
		markerHashes[m.ID] = m.Hash
	}
	return Scan{
		SpecHashes:   specHashes,
		MarkerHashes: markerHashes,
	}
}
