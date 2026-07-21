package output

import (
	"fmt"
	"path/filepath"
	"sort"

	"drift/core"
	"drift/scanner"
)

// This file contains the dispatch-side helpers that construct Results from
// evaluated state plus file I/O. Presenters are pure (no I/O); all content
// resolution happens here so that every Presenter implementation — Plain,
// Color, JSON — formats from identical, fully-resolved data.

// D! id=obldl range-start

// resolvePath joins dir with a relative path, or returns p unchanged if it's
// already absolute.
func resolvePath(dir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}

// readSpecContent reads the text inside <spec id="..."> for the given spec ID.
func readSpecContent(dir, specFilepath, specID string) (string, error) {
	return scanner.ReadSpecContent(resolvePath(dir, specFilepath), specID)
}

// readMarkerContent reads the lines between range-start and range-end for the
// given marker.
func readMarkerContent(dir, markerFilepath string, startLine, endLine int) (string, error) {
	return scanner.ReadMarkerContent(resolvePath(dir, markerFilepath), startLine, endLine)
}

// BuildListResult constructs a ListResult, pre-resolving verbose content
// previews when verbose is true. When verbose is false, the content maps are
// nil and the presenter skips previews.
func BuildListResult(state core.EvaluatedState, dir string, verbose bool) ListResult {
	result := ListResult{State: state, Verbose: verbose}
	if !verbose {
		return result
	}
	result.SpecContents = make(map[string]string)
	result.MarkerContents = make(map[string]string)
	for _, spec := range state.Specs {
		if spec.Deleted {
			continue
		}
		content, err := readSpecContent(dir, spec.Filepath, spec.ID)
		if err == nil {
			result.SpecContents[spec.ID] = content
		}
	}
	for _, marker := range state.Markers {
		if marker.Deleted {
			continue
		}
		content, err := readMarkerContent(dir, marker.Filepath, marker.LineNumber, marker.EndLineNumber)
		if err == nil {
			result.MarkerContents[marker.ID] = content
		}
	}
	return result
}

// D! id=obldl range-end

// D! id=oblds range-start

// BuildShowResult constructs a ShowResult containing the full citation closure
// reachable from the seed (a spec or marker ID). The closure walks spec-spec
// edges in both directions to fixpoint, includes every marker linked to any
// reached spec, and resolves content for every node. The 1-hop behavior is
// deprecated; show always returns the full transitive closure.
func BuildShowResult(state core.EvaluatedState, dir, id string) (ShowResult, error) {
	isSpec := isSpecID(id)

	// Locate the seed in state to confirm it exists. The caller maps
	// not-found to exit code 1 via EntityExists; here we just produce a
	// minimal ShowResult if the seed isn't present.
	seedExists := false
	if isSpec {
		for _, s := range state.Specs {
			if s.ID == id {
				seedExists = true
				break
			}
		}
	} else {
		for _, m := range state.Markers {
			if m.ID == id {
				seedExists = true
				break
			}
		}
	}

	// Build incoming and outgoing spec-spec edge maps.
	incoming := map[string]map[string]bool{}
	outgoing := map[string]map[string]bool{}
	addIncoming := func(to, from string) {
		if incoming[to] == nil {
			incoming[to] = map[string]bool{}
		}
		incoming[to][from] = true
	}
	addOutgoing := func(from, to string) {
		if outgoing[from] == nil {
			outgoing[from] = map[string]bool{}
		}
		outgoing[from][to] = true
	}
	for _, e := range state.Edges {
		if !isSpecID(e.From) || !isSpecID(e.To) {
			continue
		}
		addOutgoing(e.From, e.To)
		addIncoming(e.To, e.From)
	}

	// Seed the BFS reachability set. If the seed is a marker, expand to its
	// linked specs first (markers cannot be intermediate nodes in the spec
	// graph; they're leaves).
	reached := map[string]bool{}
	queue := []string{}
	if seedExists {
		reached[id] = true
		queue = append(queue, id)
		if !isSpec {
			for _, e := range state.Edges {
				if e.From == id && isSpecID(e.To) {
					if !reached[e.To] {
						reached[e.To] = true
						queue = append(queue, e.To)
					}
				}
			}
		}
	}

	// Bidirectional BFS to fixpoint over the spec-spec graph.
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for next := range outgoing[curr] {
			if !reached[next] {
				reached[next] = true
				queue = append(queue, next)
			}
		}
		for next := range incoming[curr] {
			if !reached[next] {
				reached[next] = true
				queue = append(queue, next)
			}
		}
	}

	// Add markers linked to any reached spec. Markers are leaf attendees;
	// they don't seed further BFS expansion.
	for _, e := range state.Edges {
		if isSpecID(e.From) {
			continue // not a marker→spec edge
		}
		if !isSpecID(e.To) {
			continue
		}
		if reached[e.To] {
			reached[e.From] = true
		}
	}

	// Resolve content for every reached spec.
	var nodes []ShowNode
	for _, s := range state.Specs {
		if !reached[s.ID] {
			continue
		}
		node := ShowNode{
			Kind:     "spec",
			ID:       s.ID,
			Filepath: s.Filepath,
			Hash:     s.Hash,
			Deleted:  s.Deleted,
		}
		if !s.Deleted {
			if content, err := readSpecContent(dir, s.Filepath, s.ID); err == nil {
				node.Content = content
			}
		}
		nodes = append(nodes, node)
	}
	// Resolve content for every reached marker.
	for _, m := range state.Markers {
		if !reached[m.ID] {
			continue
		}
		node := ShowNode{
			Kind:     "marker",
			ID:       m.ID,
			Filepath: m.Filepath,
			Hash:     m.Hash,
			Lines:    fmt.Sprintf("%d-%d", m.LineNumber, m.EndLineNumber),
			Deleted:  m.Deleted,
		}
		if !m.Deleted {
			if content, err := readMarkerContent(dir, m.Filepath, m.LineNumber, m.EndLineNumber); err == nil {
				node.Content = content
			}
		}
		nodes = append(nodes, node)
	}

	// Collect edges among reached nodes.
	var edges []core.Edge
	for _, e := range state.Edges {
		if reached[e.From] && reached[e.To] {
			edges = append(edges, e)
		}
	}

	// Sort nodes by ID and edges by (From, To) for stable output.
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})

	return ShowResult{
		IsSpec: isSpec,
		ID:     id,
		Nodes:  nodes,
		Edges:  edges,
	}, nil
}

func isSpecID(id string) bool {
	return containsDot(id)
}

func containsDot(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return true
		}
	}
	return false
}

// D! id=oblds range-end

// D! id=oblde range-start

// EntityExists reports whether the given ID exists in state as a spec or
// marker. Used by the dispatch to set the exit code for `drift show`.
func EntityExists(state core.EvaluatedState, id string) bool {
	if isSpecID(id) {
		for _, s := range state.Specs {
			if s.ID == id {
				return true
			}
		}
		return false
	}
	for _, m := range state.Markers {
		if m.ID == id {
			return true
		}
	}
	return false
}

// DiffEdgeExists reports whether a given marker/spec pair has a diff edge
// available (i.e., both exist in state).
func DiffEdgeExists(state core.EvaluatedState, markerID, specID string) bool {
	hasMarker := false
	for _, m := range state.Markers {
		if m.ID == markerID {
			hasMarker = true
			break
		}
	}
	if !hasMarker {
		return false
	}
	for _, s := range state.Specs {
		if s.ID == specID {
			return true
		}
	}
	return false
}

// sortEdgesByFromTo sorts a slice of edges lexicographically by (From, To).
// Used by the List presenters to produce diff-stable output across runs
// (state-store mutations may otherwise shuffle the slice). See
// cli.list_format.
func sortEdgesByFromTo(edges []core.Edge) {
	for i := 1; i < len(edges); i++ {
		for j := i; j > 0; j-- {
			a, b := edges[j-1], edges[j]
			if a.From > b.From || (a.From == b.From && a.To > b.To) {
				edges[j-1], edges[j] = edges[j], edges[j-1]
			} else {
				break
			}
		}
	}
}

// D! id=oblde range-end
