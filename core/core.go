package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type CoreAlgorithm struct{}

func NewCoreAlgorithm() *CoreAlgorithm {
	return &CoreAlgorithm{}
}

type Spec struct {
	Filepath   string
	LineNumber int
	ID         string
	Module     string
	Hash       string
	Deleted    bool
}

type Marker struct {
	Filepath      string
	LineNumber    int
	EndLineNumber int
	ID            string
	Hash          string
	Deleted       bool
}

// Edge is the unified directed relationship between two nodes (specs and/or
// markers). From declares/stores the edge; To is the target. Two
// establishment kinds encoded by endpoint types: marker → spec (drift link)
// and spec → spec (auto-parsed <ref>). Drift propagation is undirected.
type Edge struct {
	From string
	To   string
}

// EdgeResolution records that the (From, To) edge was reviewed at the
// current endpoint hashes.
type EdgeResolution struct {
	From            string
	To              string
	CurrentFromHash string
	CurrentToHash   string
}

type Action interface {
	isAction()
}

type Scan struct {
	SpecHashes   map[string]string
	MarkerHashes map[string]string
	Edges        []Edge
}

type TodoAction struct {
	Scan Scan
}

func (TodoAction) isAction() {}

type ResetAction struct {
	From string
	To   string
	Scan Scan
}

func (ResetAction) isAction() {}

type CoreAlgorithmContext struct {
	Specs       []Spec
	Markers     []Marker
	Edges       []Edge
	Resolutions []EdgeResolution
	Action      Action
}

type TodoKind int

const (
	TodoEdgeDrift TodoKind = iota
	TodoCascade
	TodoEdgeAdded
	TodoEdgeRemoved
	TodoBrokenEdge
)

type Todo struct {
	Kind            TodoKind
	From            string
	To              string
	FromFilepath    string
	FromLineNumber  int
	ToFilepath      string
	ToLineNumber    int
	FromChanged     bool
	ToChanged       bool
	FromDeleted     bool
	ToDeleted       bool
	SourceSpecID    string
	SourceFilepath  string
}

type EvaluatedState struct {
	Specs       []Spec
	Markers     []Marker
	Edges       []Edge
	Resolutions []EdgeResolution
	Todos       []Todo
}

var (
	ErrDuplicateSpecID        = errors.New("duplicate spec id")
	ErrDuplicateMarkerID      = errors.New("duplicate marker id")
	ErrEdgeUnknownFrom        = errors.New("edge references unknown from-node")
	ErrEdgeUnknownTo          = errors.New("edge references unknown to-node")
	ErrDuplicateEdge          = errors.New("duplicate edge")
	ErrEdgeSelfReference      = errors.New("edge references its own source")
	ErrEdgeCycle              = errors.New("edge graph contains a directed cycle")
	ErrResolutionEdgeMissing  = errors.New("resolution references edge not in baseline")
	ErrDuplicateResolution    = errors.New("duplicate resolution entry")
	ErrScanMissingSpecHash    = errors.New("scan missing spec hash")
	ErrScanMissingMarkerHash  = errors.New("scan missing marker hash")
	ErrScanUnknownSpecHash    = errors.New("scan contains unknown spec hash")
	ErrScanUnknownMarkerHash  = errors.New("scan contains unknown marker hash")
	ErrUnknownAction          = errors.New("unknown action")
	ErrResetEdgeNotInBaseline = errors.New("reset target edge not in baseline")
)

// D! id=cval range-start
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
	knownNode := func(id string) bool {
		return seenSpecIDs[id] || seenMarkerIDs[id]
	}
	seenEdgeKeys := make(map[string]bool, len(ctx.Edges))
	for _, edge := range ctx.Edges {
		if !knownNode(edge.From) {
			return fmt.Errorf("%w: %q", ErrEdgeUnknownFrom, edge.From)
		}
		if !knownNode(edge.To) {
			return fmt.Errorf("%w: %q", ErrEdgeUnknownTo, edge.To)
		}
		if edge.From == edge.To {
			return fmt.Errorf("%w: %q", ErrEdgeSelfReference, edge.From)
		}
		key := edge.From + "\x00" + edge.To
		if seenEdgeKeys[key] {
			return fmt.Errorf("%w: from=%q to=%q", ErrDuplicateEdge, edge.From, edge.To)
		}
		seenEdgeKeys[key] = true
	}
	if err := detectEdgeCycle(ctx.Edges); err != nil {
		return err
	}
	seenResolutionKeys := make(map[string]bool, len(ctx.Resolutions))
	for _, res := range ctx.Resolutions {
		if !seenEdgeKeys[res.From+"\x00"+res.To] && !seenEdgeKeys[res.To+"\x00"+res.From] {
			return fmt.Errorf("%w: from=%q to=%q", ErrResolutionEdgeMissing, res.From, res.To)
		}
		key := res.From + "\x00" + res.To
		if seenResolutionKeys[key] {
			return fmt.Errorf("%w: from=%q to=%q", ErrDuplicateResolution, res.From, res.To)
		}
		seenResolutionKeys[key] = true
	}
	return nil
}

// D! id=cval range-end

// D! id=crfv range-start
func detectEdgeCycle(edges []Edge) error {
	cycles := findAllEdgeCycles(edges)
	if len(cycles) == 0 {
		return nil
	}
	var parts []string
	for i, c := range cycles {
		if i > 0 {
			parts = append(parts, "; ")
		}
		parts = append(parts, joinPath(c))
	}
	return fmt.Errorf("%w: %s", ErrEdgeCycle, strings.Join(parts, ""))
}

func findAllEdgeCycles(edges []Edge) [][]string {
	adj := make(map[string][]string)
	for _, e := range edges {
		if !isSpecID(e.From) || !isSpecID(e.To) {
			continue
		}
		adj[e.From] = append(adj[e.From], e.To)
	}
	var cycles [][]string
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)
	var stack []string
	var visit func(node string)
	visit = func(node string) {
		color[node] = gray
		stack = append(stack, node)
		nexts := append([]string(nil), adj[node]...)
		sort.Strings(nexts)
		for _, next := range nexts {
			switch color[next] {
			case gray:
				start := 0
				for i, n := range stack {
					if n == next {
						start = i
						break
					}
				}
				path := append(append([]string{}, stack[start:]...), next)
				cycles = append(cycles, path)
			case white:
				visit(next)
			}
		}
		stack = stack[:len(stack)-1]
		color[node] = black
	}
	var nodes []string
	for n := range adj {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	for _, n := range nodes {
		if color[n] == white {
			visit(n)
		}
	}
	return cycles
}

func joinPath(nodes []string) string {
	out := ""
	for i, n := range nodes {
		if i > 0 {
			out += " → "
		}
		out += n
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

// D! id=crfv range-end

// D! id=cscn range-start
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

// D! id=cscn range-end

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

// D! id=ctodo range-start
func (algorithm *CoreAlgorithm) evaluateTodoAction(ctx CoreAlgorithmContext, action TodoAction) (EvaluatedState, error) {
	if err := validateScanCoversAllNodes(action.Scan, ctx.Specs, ctx.Markers); err != nil {
		return EvaluatedState{}, err
	}
	todos := computeEdgeTodos(
		ctx.Specs, ctx.Markers, ctx.Edges,
		indexResolutionsByEdge(ctx.Resolutions),
		action.Scan,
	)
	return EvaluatedState{
		Specs:       ctx.Specs,
		Markers:     ctx.Markers,
		Edges:       ctx.Edges,
		Resolutions: ctx.Resolutions,
		Todos:       todos,
	}, nil
}

// D! id=ctodo range-end

// D! id=crst range-start
func (algorithm *CoreAlgorithm) evaluateResetAction(ctx CoreAlgorithmContext, action ResetAction) (EvaluatedState, error) {
	if err := validateScanCoversAllNodes(action.Scan, ctx.Specs, ctx.Markers); err != nil {
		return EvaluatedState{}, err
	}
	edgeFound := false
	for _, e := range ctx.Edges {
		if (e.From == action.From && e.To == action.To) ||
			(e.From == action.To && e.To == action.From) {
			edgeFound = true
			break
		}
	}
	if !edgeFound {
		return EvaluatedState{}, fmt.Errorf("%w: from=%q to=%q", ErrResetEdgeNotInBaseline, action.From, action.To)
	}

	specsByID := copySpecsToMutableMap(ctx.Specs)
	markersByID := copyMarkersToMutableMap(ctx.Markers)
	resolutionsByEdge := indexResolutionsByEdge(ctx.Resolutions)

	resolutionsByEdge[action.From+"\x00"+action.To] = EdgeResolution{
		From:            action.From,
		To:              action.To,
		CurrentFromHash: nodeScanHash(action.From, action.Scan, specsByID, markersByID),
		CurrentToHash:   nodeScanHash(action.To, action.Scan, specsByID, markersByID),
	}

	collapsedMarkers := map[string]bool{}
	collapsedSpecs := map[string]bool{}
	collapseResolvedNodes(ctx.Edges, specsByID, markersByID, resolutionsByEdge, action.Scan, collapsedMarkers, collapsedSpecs)

	filteredEdges := filterEdgesByNodes(ctx.Edges, specsByID, markersByID)

	return EvaluatedState{
		Specs:       specsFromMutableMap(specsByID),
		Markers:     markersFromMutableMap(markersByID),
		Edges:       filteredEdges,
		Resolutions: resolutionsFromMutableMap(resolutionsByEdge),
	}, nil
}

// D! id=crst range-end

// D! id=ccol range-start
func collapseResolvedNodes(
	edges []Edge,
	specsByID map[string]*Spec,
	markersByID map[string]*Marker,
	resolutionsByEdge map[string]EdgeResolution,
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
			if nodeHasAllEdgesChecked(markerID, edges, specsByID, markersByID, resolutionsByEdge, scan) {
				if scan.MarkerHashes[markerID] == "" {
					delete(markersByID, markerID)
					collapsedMarkers[markerID] = true
				} else {
					marker.Hash = scan.MarkerHashes[markerID]
					collapsedMarkers[markerID] = true
				}
				anyNodeCollapsed = true
			}
		}
		for specID, spec := range specsByID {
			if collapsedSpecs[specID] {
				continue
			}
			if nodeHasAllEdgesChecked(specID, edges, specsByID, markersByID, resolutionsByEdge, scan) {
				if scan.SpecHashes[specID] == "" {
					delete(specsByID, specID)
					collapsedSpecs[specID] = true
				} else {
					spec.Hash = scan.SpecHashes[specID]
					collapsedSpecs[specID] = true
				}
				anyNodeCollapsed = true
			}
		}
		if !anyNodeCollapsed {
			break
		}
	}
	for nodeID := range collapsedMarkers {
		pruneResolutionsForCollapsedNode(resolutionsByEdge, nodeID, edges, specsByID, markersByID, scan)
	}
	for nodeID := range collapsedSpecs {
		pruneResolutionsForCollapsedNode(resolutionsByEdge, nodeID, edges, specsByID, markersByID, scan)
	}
}

// D! id=ccol range-end

// D! id=nchk range-start
func nodeHasAllEdgesChecked(
	nodeID string,
	edges []Edge,
	specsByID map[string]*Spec,
	markersByID map[string]*Marker,
	resolutionsByEdge map[string]EdgeResolution,
	scan Scan,
) bool {
	for _, e := range edges {
		if e.From != nodeID && e.To != nodeID {
			continue
		}
		if edgeIsUnchecked(e, specsByID, markersByID, resolutionsByEdge, scan) {
			return false
		}
	}
	return true
}

// D! id=nchk range-end

// D! id=cedg range-start
func edgeIsUnchecked(
	edge Edge,
	specsByID map[string]*Spec,
	markersByID map[string]*Marker,
	resolutionsByEdge map[string]EdgeResolution,
	scan Scan,
) bool {
	fromCurrent := nodeScanHash(edge.From, scan, specsByID, markersByID)
	toCurrent := nodeScanHash(edge.To, scan, specsByID, markersByID)
	fromBaseline := nodeBaselineHash(edge.From, specsByID, markersByID)
	toBaseline := nodeBaselineHash(edge.To, specsByID, markersByID)
	if fromBaseline == fromCurrent && toBaseline == toCurrent {
		return false
	}
	if res, ok := resolutionsByEdge[edge.From+"\x00"+edge.To]; ok {
		if res.CurrentFromHash == fromCurrent && res.CurrentToHash == toCurrent {
			return false
		}
	}
	if res, ok := resolutionsByEdge[edge.To+"\x00"+edge.From]; ok {
		if res.CurrentFromHash == toCurrent && res.CurrentToHash == fromCurrent {
			return false
		}
	}
	return true
}

// D! id=cedg range-end

// D! id=prune range-start
func pruneResolutionsForCollapsedNode(
	resolutionsByEdge map[string]EdgeResolution,
	collapsedNodeID string,
	edges []Edge,
	specsByID map[string]*Spec,
	markersByID map[string]*Marker,
	scan Scan,
) {
	nodeDeleted := nodeScanHash(collapsedNodeID, scan, specsByID, markersByID) == ""
	for _, e := range edges {
		if e.From != collapsedNodeID && e.To != collapsedNodeID {
			continue
		}
		other := e.To
		if e.To == collapsedNodeID {
			other = e.From
		}
		for _, key := range []string{e.From + "\x00" + e.To, e.To + "\x00" + e.From} {
			if _, ok := resolutionsByEdge[key]; !ok {
				continue
			}
			if nodeDeleted {
				delete(resolutionsByEdge, key)
				continue
			}
			otherBaseline := nodeBaselineHash(other, specsByID, markersByID)
			otherCurrent := nodeScanHash(other, scan, specsByID, markersByID)
			if otherBaseline == otherCurrent {
				delete(resolutionsByEdge, key)
			}
		}
	}
}

// D! id=prune range-end

// D! id=cetd range-start
func computeEdgeTodos(
	specs []Spec,
	markers []Marker,
	edges []Edge,
	resolutionsByEdge map[string]EdgeResolution,
	scan Scan,
) []Todo {
	specByID := copySpecsToMutableMap(specs)
	markerByID := copyMarkersToMutableMap(markers)
	nodeExists := func(id string) bool {
		if _, ok := scan.SpecHashes[id]; ok {
			return scan.SpecHashes[id] != ""
		}
		if _, ok := scan.MarkerHashes[id]; ok {
			return scan.MarkerHashes[id] != ""
		}
		return false
	}

	baselineEdgeSet := make(map[string]bool, len(edges))
	for _, e := range edges {
		baselineEdgeSet[e.From+"\x00"+e.To] = true
	}
	scanEdgeSet := make(map[string]bool, len(scan.Edges))
	for _, e := range scan.Edges {
		scanEdgeSet[e.From+"\x00"+e.To] = true
	}

	var todos []Todo

	for _, e := range edges {
		fromCurrent := nodeScanHash(e.From, scan, specByID, markerByID)
		toCurrent := nodeScanHash(e.To, scan, specByID, markerByID)
		fromBaseline := nodeBaselineHash(e.From, specByID, markerByID)
		toBaseline := nodeBaselineHash(e.To, specByID, markerByID)
		fromChanged := fromBaseline != fromCurrent
		toChanged := toBaseline != toCurrent
		if !fromChanged && !toChanged {
			continue
		}
		covered := false
		if res, ok := resolutionsByEdge[e.From+"\x00"+e.To]; ok {
			if res.CurrentFromHash == fromCurrent && res.CurrentToHash == toCurrent {
				covered = true
			}
		}
		if !covered {
			if res, ok := resolutionsByEdge[e.To+"\x00"+e.From]; ok {
				if res.CurrentFromHash == toCurrent && res.CurrentToHash == fromCurrent {
					covered = true
				}
			}
		}
		if covered {
			continue
		}
		todos = append(todos, Todo{
			Kind:           TodoEdgeDrift,
			From:           e.From,
			To:             e.To,
			FromFilepath:   nodeFilepath(e.From, specByID, markerByID),
			FromLineNumber: nodeLineNumber(e.From, specByID, markerByID),
			ToFilepath:     nodeFilepath(e.To, specByID, markerByID),
			ToLineNumber:   nodeLineNumber(e.To, specByID, markerByID),
			FromChanged:    fromChanged,
			ToChanged:      toChanged,
			FromDeleted:    fromCurrent == "" && fromChanged,
			ToDeleted:      toCurrent == "" && toChanged,
		})
	}

	for _, e := range scan.Edges {
		if !isSpecID(e.From) || !isSpecID(e.To) {
			continue
		}
		if baselineEdgeSet[e.From+"\x00"+e.To] {
			continue
		}
		todos = append(todos, Todo{
			Kind:         TodoEdgeAdded,
			From:         e.From,
			To:           e.To,
			FromFilepath: nodeFilepath(e.From, specByID, markerByID),
		})
	}
	for _, e := range edges {
		if !isSpecID(e.From) || !isSpecID(e.To) {
			continue
		}
		if scanEdgeSet[e.From+"\x00"+e.To] {
			continue
		}
		todos = append(todos, Todo{
			Kind:         TodoEdgeRemoved,
			From:         e.From,
			To:           e.To,
			FromFilepath: nodeFilepath(e.From, specByID, markerByID),
		})
	}

	for _, e := range scan.Edges {
		if nodeExists(e.To) {
			continue
		}
		todos = append(todos, Todo{
			Kind:         TodoBrokenEdge,
			From:         e.From,
			To:           e.To,
			FromFilepath: nodeFilepath(e.From, specByID, markerByID),
		})
	}

	closureTodos := computeRhizomaticClosureTodos(
		specs, markers, edges, scan.Edges, resolutionsByEdge, scan,
		specByID, markerByID,
	)
	todos = append(todos, closureTodos...)

	return todos
}

// D! id=cetd range-end

// D! id=crhiz range-start
func computeRhizomaticClosureTodos(
	specs []Spec,
	markers []Marker,
	edges []Edge,
	scanEdges []Edge,
	resolutionsByEdge map[string]EdgeResolution,
	scan Scan,
	specByID map[string]*Spec,
	markerByID map[string]*Marker,
) []Todo {
	changedSpecs := make(map[string]bool)
	for _, s := range specs {
		current, ok := scan.SpecHashes[s.ID]
		if !ok || current == "" {
			continue
		}
		if s.Hash != current {
			changedSpecs[s.ID] = true
		}
	}
	if len(changedSpecs) == 0 {
		return nil
	}

	baselineSpecSpec := make(map[string]bool)
	for _, e := range edges {
		if isSpecID(e.From) && isSpecID(e.To) {
			baselineSpecSpec[e.From+"\x00"+e.To] = true
			baselineSpecSpec[e.To+"\x00"+e.From] = true
		}
	}

	neighbors := make(map[string]map[string]bool)
	addNeighbor := func(a, b string) {
		if !isSpecID(a) || !isSpecID(b) {
			return
		}
		if neighbors[a] == nil {
			neighbors[a] = make(map[string]bool)
		}
		if neighbors[b] == nil {
			neighbors[b] = make(map[string]bool)
		}
		neighbors[a][b] = true
		neighbors[b][a] = true
	}
	for _, e := range scanEdges {
		addNeighbor(e.From, e.To)
	}

	markersBySpec := make(map[string][]Marker)
	for _, e := range edges {
		if isSpecID(e.From) {
			continue
		}
		if m, ok := markerByID[e.From]; ok {
			markersBySpec[e.To] = append(markersBySpec[e.To], *m)
		}
	}

	var todos []Todo
	for sourceID := range changedSpecs {
		sourceHash := scan.SpecHashes[sourceID]
		sourceSpec := specByID[sourceID]
		visited := map[string]bool{sourceID: true}
		queue := []string{sourceID}
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			for n := range neighbors[curr] {
				if visited[n] {
					continue
				}
				visited[n] = true
				queue = append(queue, n)

				if changedSpecs[n] {
					continue
				}

				fromCurrent := sourceHash
				toCurrent := scan.SpecHashes[n]
				covered := false
				if res, ok := resolutionsByEdge[n+"\x00"+sourceID]; ok {
					if res.CurrentFromHash == toCurrent && res.CurrentToHash == fromCurrent {
						covered = true
					}
				}
				if !covered {
					if res, ok := resolutionsByEdge[sourceID+"\x00"+n]; ok {
						if res.CurrentFromHash == fromCurrent && res.CurrentToHash == toCurrent {
							covered = true
						}
					}
				}
				if covered {
					continue
				}

				spec := specByID[n]
				if !baselineSpecSpec[n+"\x00"+sourceID] {
					todos = append(todos, Todo{
						Kind:           TodoEdgeDrift,
						From:           n,
						To:             sourceID,
						FromFilepath:   spec.Filepath,
						ToFilepath:     sourceSpec.Filepath,
						ToChanged:      true,
						SourceSpecID:   sourceID,
						SourceFilepath: sourceSpec.Filepath,
					})
				}

				for _, m := range markersBySpec[n] {
					todos = append(todos, Todo{
						Kind:           TodoCascade,
						From:           m.ID,
						To:             n,
						FromFilepath:   m.Filepath,
						FromLineNumber: m.LineNumber,
						ToFilepath:     spec.Filepath,
						SourceSpecID:   sourceID,
						SourceFilepath: sourceSpec.Filepath,
					})
				}
			}
		}
	}

	return todos
}

// D! id=crhiz range-end

// --- helpers ---

func nodeScanHash(id string, scan Scan, specsByID map[string]*Spec, markersByID map[string]*Marker) string {
	if _, ok := specsByID[id]; ok {
		return scan.SpecHashes[id]
	}
	if _, ok := markersByID[id]; ok {
		return scan.MarkerHashes[id]
	}
	if isSpecID(id) {
		return scan.SpecHashes[id]
	}
	return scan.MarkerHashes[id]
}

func nodeBaselineHash(id string, specsByID map[string]*Spec, markersByID map[string]*Marker) string {
	if s := specsByID[id]; s != nil {
		return s.Hash
	}
	if m := markersByID[id]; m != nil {
		return m.Hash
	}
	return ""
}

func nodeFilepath(id string, specsByID map[string]*Spec, markersByID map[string]*Marker) string {
	if s := specsByID[id]; s != nil {
		return s.Filepath
	}
	if m := markersByID[id]; m != nil {
		return m.Filepath
	}
	return ""
}

func nodeLineNumber(id string, specsByID map[string]*Spec, markersByID map[string]*Marker) int {
	if s := specsByID[id]; s != nil {
		return s.LineNumber
	}
	if m := markersByID[id]; m != nil {
		return m.LineNumber
	}
	return 0
}

func indexResolutionsByEdge(resolutions []EdgeResolution) map[string]EdgeResolution {
	out := make(map[string]EdgeResolution, len(resolutions))
	for _, r := range resolutions {
		out[r.From+"\x00"+r.To] = r
	}
	return out
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

func resolutionsFromMutableMap(resolutionsByEdge map[string]EdgeResolution) []EdgeResolution {
	out := make([]EdgeResolution, 0, len(resolutionsByEdge))
	for _, r := range resolutionsByEdge {
		out = append(out, r)
	}
	return out
}

func filterEdgesByNodes(edges []Edge, specsByID map[string]*Spec, markersByID map[string]*Marker) []Edge {
	var out []Edge
	for _, e := range edges {
		fromOK := specsByID[e.From] != nil || markersByID[e.From] != nil
		toOK := specsByID[e.To] != nil || markersByID[e.To] != nil
		if fromOK && toOK {
			out = append(out, e)
		}
	}
	return out
}
