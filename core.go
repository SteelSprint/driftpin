package main

import (
	"errors"
	"fmt"
)

type CoreAlgorithm struct{}

func NewCoreAlgorithm() *CoreAlgorithm {
	return &CoreAlgorithm{}
}

type Spec struct {
	Filepath   string
	LineNumber int
	ID         string
	Hash       string
}

type Marker struct {
	Filepath   string
	LineNumber int
	ID         string
	Hash       string
}

type Link struct {
	SpecID   string
	MarkerID string
}

type ResolutionState struct {
	SpecID            string
	MarkerID          string
	CurrentSpecHash   string
	CurrentMarkerHash string
}

type Action interface {
	isAction()
}

type Scan struct {
	SpecHashes   map[string]string
	MarkerHashes map[string]string
}

type TodoAction struct {
	Scan Scan
}

func (TodoAction) isAction() {}

type ResetAction struct {
	SpecID   string
	MarkerID string
	Scan     Scan
}

func (ResetAction) isAction() {}

type CoreAlgorithmContext struct {
	Specs           []Spec
	Markers         []Marker
	Links           []Link
	ResolutionState []ResolutionState
	Action          Action
}

type Todo struct {
	SpecID           string
	MarkerID         string
	SpecFilepath     string
	SpecLineNumber   int
	MarkerFilepath   string
	MarkerLineNumber int
	SpecChanged      bool
	MarkerChanged    bool
}

type EvaluatedState struct {
	Specs           []Spec
	Markers         []Marker
	Links           []Link
	ResolutionState []ResolutionState
	Todos           []Todo
}

var (
	ErrDuplicateSpecID          = errors.New("duplicate spec id")
	ErrDuplicateMarkerID       = errors.New("duplicate marker id")
	ErrDuplicateLink           = errors.New("duplicate link")
	ErrLinkUnknownSpec         = errors.New("link references unknown spec")
	ErrLinkUnknownMarker       = errors.New("link references unknown marker")
	ErrResolutionUnknownSpec   = errors.New("resolution references unknown spec")
	ErrResolutionUnknownMarker = errors.New("resolution references unknown marker")
	ErrResolutionEdgeNotLinked = errors.New("resolution references edge not in links")
	ErrDuplicateResolution     = errors.New("duplicate resolution entry")
	ErrScanMissingSpecHash     = errors.New("scan missing spec hash")
	ErrScanMissingMarkerHash   = errors.New("scan missing marker hash")
	ErrScanUnknownSpecHash     = errors.New("scan contains unknown spec hash")
	ErrScanUnknownMarkerHash   = errors.New("scan contains unknown marker hash")
	ErrUnknownAction           = errors.New("unknown action")
	ErrResetEdgeNotLinked      = errors.New("reset target edge not in links")
)

func (ctx CoreAlgorithmContext) Validate() error {
	seenSpecIDs := make(map[string]bool, len(ctx.Specs))
	for _, spec := range ctx.Specs {
		if seenSpecIDs[spec.ID] {
			return fmt.Errorf("%w: %q", ErrDuplicateSpecID, spec.ID)
		}
		seenSpecIDs[spec.ID] = true
	}
	seenMarkerIDs := make(map[string]bool, len(ctx.Markers))
	for _, marker := range ctx.Markers {
		if seenMarkerIDs[marker.ID] {
			return fmt.Errorf("%w: %q", ErrDuplicateMarkerID, marker.ID)
		}
		seenMarkerIDs[marker.ID] = true
	}
	seenLinkKeys := make(map[string]bool, len(ctx.Links))
	for _, link := range ctx.Links {
		if !seenSpecIDs[link.SpecID] {
			return fmt.Errorf("%w: %q", ErrLinkUnknownSpec, link.SpecID)
		}
		if !seenMarkerIDs[link.MarkerID] {
			return fmt.Errorf("%w: %q", ErrLinkUnknownMarker, link.MarkerID)
		}
		linkKey := link.SpecID + "\x00" + link.MarkerID
		if seenLinkKeys[linkKey] {
			return fmt.Errorf("%w: spec=%q marker=%q", ErrDuplicateLink, link.SpecID, link.MarkerID)
		}
		seenLinkKeys[linkKey] = true
	}
	seenResolutionKeys := make(map[string]bool, len(ctx.ResolutionState))
	for _, res := range ctx.ResolutionState {
		if !seenSpecIDs[res.SpecID] {
			return fmt.Errorf("%w: %q", ErrResolutionUnknownSpec, res.SpecID)
		}
		if !seenMarkerIDs[res.MarkerID] {
			return fmt.Errorf("%w: %q", ErrResolutionUnknownMarker, res.MarkerID)
		}
		linkKey := res.SpecID + "\x00" + res.MarkerID
		if !seenLinkKeys[linkKey] {
			return fmt.Errorf("%w: spec=%q marker=%q", ErrResolutionEdgeNotLinked, res.SpecID, res.MarkerID)
		}
		resolutionKey := resolutionEdgeKey(res.MarkerID, res.SpecID)
		if seenResolutionKeys[resolutionKey] {
			return fmt.Errorf("%w: spec=%q marker=%q", ErrDuplicateResolution, res.SpecID, res.MarkerID)
		}
		seenResolutionKeys[resolutionKey] = true
	}
	return nil
}

func validateScanCoversAllNodes(scan Scan, specs []Spec, markers []Marker) error {
	for _, spec := range specs {
		if _, ok := scan.SpecHashes[spec.ID]; !ok {
			return fmt.Errorf("%w: %q", ErrScanMissingSpecHash, spec.ID)
		}
	}
	for _, marker := range markers {
		if _, ok := scan.MarkerHashes[marker.ID]; !ok {
			return fmt.Errorf("%w: %q", ErrScanMissingMarkerHash, marker.ID)
		}
	}
	for specID := range scan.SpecHashes {
		found := false
		for _, spec := range specs {
			if spec.ID == specID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: %q", ErrScanUnknownSpecHash, specID)
		}
	}
	for markerID := range scan.MarkerHashes {
		found := false
		for _, marker := range markers {
			if marker.ID == markerID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: %q", ErrScanUnknownMarkerHash, markerID)
		}
	}
	return nil
}

func (algorithm *CoreAlgorithm) EvaluateState(ctx CoreAlgorithmContext) (EvaluatedState, error) {
	if err := ctx.Validate(); err != nil {
		return EvaluatedState{}, err
	}
	switch action := ctx.Action.(type) {
	case TodoAction:
		return algorithm.evaluateTodoAction(ctx, action)
	case ResetAction:
		return algorithm.evaluateResetAction(ctx, action)
	default:
		return EvaluatedState{}, fmt.Errorf("%w: %T", ErrUnknownAction, ctx.Action)
	}
}

func (algorithm *CoreAlgorithm) evaluateTodoAction(ctx CoreAlgorithmContext, action TodoAction) (EvaluatedState, error) {
	if err := validateScanCoversAllNodes(action.Scan, ctx.Specs, ctx.Markers); err != nil {
		return EvaluatedState{}, err
	}
	todos := computeTodoList(
		ctx.Links,
		indexSpecsByID(ctx.Specs),
		indexMarkersByID(ctx.Markers),
		indexResolutionStateByEdge(ctx.ResolutionState),
		action.Scan,
	)
	return EvaluatedState{
		Specs:           ctx.Specs,
		Markers:         ctx.Markers,
		Links:           ctx.Links,
		ResolutionState: ctx.ResolutionState,
		Todos:           todos,
	}, nil
}

func (algorithm *CoreAlgorithm) evaluateResetAction(ctx CoreAlgorithmContext, action ResetAction) (EvaluatedState, error) {
	if err := validateScanCoversAllNodes(action.Scan, ctx.Specs, ctx.Markers); err != nil {
		return EvaluatedState{}, err
	}
	linkExists := false
	for _, link := range ctx.Links {
		if link.SpecID == action.SpecID && link.MarkerID == action.MarkerID {
			linkExists = true
			break
		}
	}
	if !linkExists {
		return EvaluatedState{}, fmt.Errorf("%w: spec=%q marker=%q", ErrResetEdgeNotLinked, action.SpecID, action.MarkerID)
	}

	specsByID := copySpecsToMutableMap(ctx.Specs)
	markersByID := copyMarkersToMutableMap(ctx.Markers)
	resolutionStateByEdge := indexResolutionStateByEdge(ctx.ResolutionState)

	resolutionStateByEdge[resolutionEdgeKey(action.MarkerID, action.SpecID)] = ResolutionState{
		SpecID:            action.SpecID,
		MarkerID:          action.MarkerID,
		CurrentSpecHash:   action.Scan.SpecHashes[action.SpecID],
		CurrentMarkerHash: action.Scan.MarkerHashes[action.MarkerID],
	}

	collapsedMarkers := map[string]bool{}
	collapsedSpecs := map[string]bool{}
	collapseResolvedNodes(ctx.Links, specsByID, markersByID, resolutionStateByEdge, action.Scan, collapsedMarkers, collapsedSpecs)

	return EvaluatedState{
		Specs:           specsFromMutableMap(specsByID),
		Markers:         markersFromMutableMap(markersByID),
		Links:           ctx.Links,
		ResolutionState: resolutionStateFromMutableMap(resolutionStateByEdge),
	}, nil
}

func collapseResolvedNodes(
	links []Link,
	specsByID map[string]*Spec,
	markersByID map[string]*Marker,
	resolutionStateByEdge map[string]ResolutionState,
	scan Scan,
	collapsedMarkers map[string]bool,
	collapsedSpecs map[string]bool,
) {
	for {
		anyNodeCollapsed := false
		for markerID, marker := range markersByID {
			if collapsedMarkers[markerID] {
				continue
			}
			if markerHasAllEdgesChecked(markerID, links, specsByID, markersByID, resolutionStateByEdge, scan) {
				marker.Hash = scan.MarkerHashes[markerID]
				collapsedMarkers[markerID] = true
				anyNodeCollapsed = true
				pruneResolutionEntriesForCollapsedMarker(resolutionStateByEdge, markerID, links, specsByID, scan)
			}
		}
		for specID, spec := range specsByID {
			if collapsedSpecs[specID] {
				continue
			}
			if specHasAllEdgesChecked(specID, links, specsByID, markersByID, resolutionStateByEdge, scan) {
				spec.Hash = scan.SpecHashes[specID]
				collapsedSpecs[specID] = true
				anyNodeCollapsed = true
				pruneResolutionEntriesForCollapsedSpec(resolutionStateByEdge, specID, links, markersByID, scan)
			}
		}
		if !anyNodeCollapsed {
			break
		}
	}
}

func markerHasAllEdgesChecked(
	markerID string,
	links []Link,
	specsByID map[string]*Spec,
	markersByID map[string]*Marker,
	resolutionStateByEdge map[string]ResolutionState,
	scan Scan,
) bool {
	for _, link := range links {
		if link.MarkerID != markerID {
			continue
		}
		if edgeIsUnchecked(link, specsByID, markersByID, resolutionStateByEdge, scan) {
			return false
		}
	}
	return true
}

func specHasAllEdgesChecked(
	specID string,
	links []Link,
	specsByID map[string]*Spec,
	markersByID map[string]*Marker,
	resolutionStateByEdge map[string]ResolutionState,
	scan Scan,
) bool {
	for _, link := range links {
		if link.SpecID != specID {
			continue
		}
		if edgeIsUnchecked(link, specsByID, markersByID, resolutionStateByEdge, scan) {
			return false
		}
	}
	return true
}

func edgeIsUnchecked(
	link Link,
	specsByID map[string]*Spec,
	markersByID map[string]*Marker,
	resolutionStateByEdge map[string]ResolutionState,
	scan Scan,
) bool {
	currentMarkerHash := scan.MarkerHashes[link.MarkerID]
	currentSpecHash := scan.SpecHashes[link.SpecID]
	marker := markersByID[link.MarkerID]
	spec := specsByID[link.SpecID]
	edgeIsConsistent := marker.Hash == currentMarkerHash && spec.Hash == currentSpecHash
	if edgeIsConsistent {
		return false
	}
	if res, ok := resolutionStateByEdge[resolutionEdgeKey(link.MarkerID, link.SpecID)]; ok {
		if res.CurrentMarkerHash == currentMarkerHash && res.CurrentSpecHash == currentSpecHash {
			return false
		}
	}
	return true
}

func pruneResolutionEntriesForCollapsedMarker(
	resolutionStateByEdge map[string]ResolutionState,
	collapsedMarkerID string,
	links []Link,
	specsByID map[string]*Spec,
	scan Scan,
) {
	for _, link := range links {
		if link.MarkerID != collapsedMarkerID {
			continue
		}
		edgeKey := resolutionEdgeKey(link.MarkerID, link.SpecID)
		if _, ok := resolutionStateByEdge[edgeKey]; !ok {
			continue
		}
		if spec := specsByID[link.SpecID]; spec.Hash == scan.SpecHashes[link.SpecID] {
			delete(resolutionStateByEdge, edgeKey)
		}
	}
}

func pruneResolutionEntriesForCollapsedSpec(
	resolutionStateByEdge map[string]ResolutionState,
	collapsedSpecID string,
	links []Link,
	markersByID map[string]*Marker,
	scan Scan,
) {
	for _, link := range links {
		if link.SpecID != collapsedSpecID {
			continue
		}
		edgeKey := resolutionEdgeKey(link.MarkerID, link.SpecID)
		if _, ok := resolutionStateByEdge[edgeKey]; !ok {
			continue
		}
		if marker := markersByID[link.MarkerID]; marker.Hash == scan.MarkerHashes[link.MarkerID] {
			delete(resolutionStateByEdge, edgeKey)
		}
	}
}

func computeTodoList(
	links []Link,
	specsByID map[string]Spec,
	markersByID map[string]Marker,
	resolutionStateByEdge map[string]ResolutionState,
	scan Scan,
) []Todo {
	var todos []Todo
	for _, link := range links {
		spec := specsByID[link.SpecID]
		marker := markersByID[link.MarkerID]
		currentSpecHash := scan.SpecHashes[link.SpecID]
		currentMarkerHash := scan.MarkerHashes[link.MarkerID]
		specChanged := spec.Hash != currentSpecHash
		markerChanged := marker.Hash != currentMarkerHash
		if !specChanged && !markerChanged {
			continue
		}
		if res, ok := resolutionStateByEdge[resolutionEdgeKey(link.MarkerID, link.SpecID)]; ok {
			if res.CurrentMarkerHash == currentMarkerHash && res.CurrentSpecHash == currentSpecHash {
				continue
			}
		}
		todos = append(todos, Todo{
			SpecID:           link.SpecID,
			MarkerID:         link.MarkerID,
			SpecFilepath:     spec.Filepath,
			SpecLineNumber:   spec.LineNumber,
			MarkerFilepath:   marker.Filepath,
			MarkerLineNumber: marker.LineNumber,
			SpecChanged:      specChanged,
			MarkerChanged:    markerChanged,
		})
	}
	return todos
}

func resolutionEdgeKey(markerID, specID string) string {
	return markerID + ":" + specID
}

func indexSpecsByID(specs []Spec) map[string]Spec {
	specsByID := make(map[string]Spec, len(specs))
	for _, spec := range specs {
		specsByID[spec.ID] = spec
	}
	return specsByID
}

func indexMarkersByID(markers []Marker) map[string]Marker {
	markersByID := make(map[string]Marker, len(markers))
	for _, marker := range markers {
		markersByID[marker.ID] = marker
	}
	return markersByID
}

func indexResolutionStateByEdge(resolutionState []ResolutionState) map[string]ResolutionState {
	resolutionStateByEdge := make(map[string]ResolutionState, len(resolutionState))
	for _, res := range resolutionState {
		resolutionStateByEdge[resolutionEdgeKey(res.MarkerID, res.SpecID)] = res
	}
	return resolutionStateByEdge
}

func copySpecsToMutableMap(specs []Spec) map[string]*Spec {
	specsByID := make(map[string]*Spec, len(specs))
	for i := range specs {
		spec := specs[i]
		specsByID[spec.ID] = &spec
	}
	return specsByID
}

func copyMarkersToMutableMap(markers []Marker) map[string]*Marker {
	markersByID := make(map[string]*Marker, len(markers))
	for i := range markers {
		marker := markers[i]
		markersByID[marker.ID] = &marker
	}
	return markersByID
}

func specsFromMutableMap(specsByID map[string]*Spec) []Spec {
	out := make([]Spec, 0, len(specsByID))
	for _, spec := range specsByID {
		out = append(out, *spec)
	}
	return out
}

func markersFromMutableMap(markersByID map[string]*Marker) []Marker {
	out := make([]Marker, 0, len(markersByID))
	for _, marker := range markersByID {
		out = append(out, *marker)
	}
	return out
}

func resolutionStateFromMutableMap(resolutionStateByEdge map[string]ResolutionState) []ResolutionState {
	out := make([]ResolutionState, 0, len(resolutionStateByEdge))
	for _, res := range resolutionStateByEdge {
		out = append(out, res)
	}
	return out
}
