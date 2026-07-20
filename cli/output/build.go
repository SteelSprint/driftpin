package output

import (
	"fmt"
	"path/filepath"

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

// BuildShowResult constructs a ShowResult by resolving the entity lookup and
// reading all file content. Returns a non-nil error when content reading fails
// for the primary entity. When the entity is not found, returns a ShowResult
// with nil Spec and Marker (and the caller sets exit code 1).
func BuildShowResult(state core.EvaluatedState, dir, id string) (ShowResult, error) {
	isSpec := isSpecID(id)
	if isSpec {
		return buildShowSpecResult(state, dir, id)
	}
	return buildShowMarkerResult(state, dir, id)
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

func buildShowSpecResult(state core.EvaluatedState, dir, specID string) (ShowResult, error) {
	var spec *core.Spec
	for i := range state.Specs {
		if state.Specs[i].ID == specID {
			spec = &state.Specs[i]
			break
		}
	}
	if spec == nil {
		return ShowResult{IsSpec: true, ID: specID}, nil
	}
	content, err := readSpecContent(dir, spec.Filepath, spec.ID)
	if err != nil {
		return ShowResult{}, fmt.Errorf("error reading spec content: %s", err)
	}
	var linkedMarkers []LinkedMarker
	for _, e := range state.Edges {
		// Link-style edge: From is marker (no dot), To is spec.
		if isSpecID(e.From) {
			continue
		}
		if e.To != specID {
			continue
		}
		for i := range state.Markers {
			if state.Markers[i].ID == e.From {
				m := state.Markers[i]
				markerContent, err := readMarkerContent(dir, m.Filepath, m.LineNumber, m.EndLineNumber)
				if err != nil {
					continue
				}
				linkedMarkers = append(linkedMarkers, LinkedMarker{Marker: m, Content: markerContent})
				break
			}
		}
	}
	// Compute inbound/outbound ref sets from baseline spec-spec edges.
	var outbound, inbound []string
	seenOut := make(map[string]bool)
	seenIn := make(map[string]bool)
	for _, e := range state.Edges {
		if !isSpecID(e.From) || !isSpecID(e.To) {
			continue
		}
		if e.From == specID && !seenOut[e.To] {
			outbound = append(outbound, e.To)
			seenOut[e.To] = true
		}
		if e.To == specID && !seenIn[e.From] {
			inbound = append(inbound, e.From)
			seenIn[e.From] = true
		}
	}
	return ShowResult{
		IsSpec:        true,
		ID:            specID,
		Spec:          spec,
		Content:       content,
		LinkedMarkers: linkedMarkers,
		OutboundRefs:  outbound,
		InboundRefs:   inbound,
	}, nil
}

func buildShowMarkerResult(state core.EvaluatedState, dir, markerID string) (ShowResult, error) {
	var marker *core.Marker
	for i := range state.Markers {
		if state.Markers[i].ID == markerID {
			marker = &state.Markers[i]
			break
		}
	}
	if marker == nil {
		return ShowResult{IsSpec: false, ID: markerID}, nil
	}
	var linkedSpecs []LinkedSpec
	for _, e := range state.Edges {
		// Link-style edge: From is marker.
		if isSpecID(e.From) {
			continue
		}
		if e.From != markerID {
			continue
		}
		for i := range state.Specs {
			if state.Specs[i].ID == e.To {
				s := state.Specs[i]
				content, err := readSpecContent(dir, s.Filepath, s.ID)
				if err != nil {
					continue
				}
				linkedSpecs = append(linkedSpecs, LinkedSpec{Spec: s, Content: content})
				break
			}
		}
	}
	markerContent, err := readMarkerContent(dir, marker.Filepath, marker.LineNumber, marker.EndLineNumber)
	if err != nil {
		return ShowResult{}, fmt.Errorf("error reading marker content: %s", err)
	}
	return ShowResult{
		IsSpec:      false,
		ID:          markerID,
		Marker:      marker,
		Content:     markerContent,
		LinkedSpecs: linkedSpecs,
	}, nil
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
