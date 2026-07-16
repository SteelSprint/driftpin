package orchestrator

import (
	"fmt"
	"strings"

	"driftpin/core"
	"driftpin/pinstore"
	"driftpin/scanner"
)

var (
	ErrSpecNotFoundOnDisk   = fmt.Errorf("spec in drift.pin not found on disk")
	ErrMarkerNotFoundOnDisk = fmt.Errorf("marker in drift.pin not found on disk")
	ErrLinkMarkerNotFound   = fmt.Errorf("link references unknown marker")
	ErrLinkSpecNotFound     = fmt.Errorf("link references unknown spec")
	ErrLinkAlreadyExists    = fmt.Errorf("link already exists")
	markerSyntax            = "D" + "! id=<shortcode>"
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

	scan := buildScan(scanResult)

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

	scan := buildScan(scanResult)

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

	for id := range pinnedByID {
		if !scannedByID[id] {
			return nil, fmt.Errorf("%w: %q", ErrSpecNotFoundOnDisk, id)
		}
	}

	result := make([]core.Spec, len(scanned))
	for i, s := range scanned {
		if pinned, ok := pinnedByID[s.ID]; ok {
			result[i] = core.Spec{
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

	for id := range pinnedByID {
		if !scannedByID[id] {
			return nil, fmt.Errorf("%w: %q", ErrMarkerNotFoundOnDisk, id)
		}
	}

	result := make([]core.Marker, len(scanned))
	for i, m := range scanned {
		if pinned, ok := pinnedByID[m.ID]; ok {
			result[i] = core.Marker{
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

func buildScan(scanResult scanner.ScanResult) core.Scan {
	specHashes := make(map[string]string, len(scanResult.Specs))
	for _, s := range scanResult.Specs {
		specHashes[s.ID] = s.Hash
	}
	markerHashes := make(map[string]string, len(scanResult.Markers))
	for _, m := range scanResult.Markers {
		markerHashes[m.ID] = m.Hash
	}
	return core.Scan{
		SpecHashes:   specHashes,
		MarkerHashes: markerHashes,
	}
}
