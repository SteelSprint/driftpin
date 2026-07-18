package output

import (
	"fmt"
	"strconv"
	"strings"

	"drift/core"
	"drift/internal/diff"
	"drift/orchestrator"
)

// PlainPresenter is the byte-identical continuation of pre-output-layer output.
type PlainPresenter struct{}

// markerSyntax is the user-facing shorthand for marker comments.
var markerSyntax = "D" + "! id=<markerid>"

// D! id=cfmt range-start
func (p PlainPresenter) Todo(r TodoResult) string {
	state := r.State
	if len(state.Specs) == 0 && len(state.Markers) == 0 {
		return "Nothing to check: no specs or markers registered.\nCreate spec files (*.drift.xml) and place " + markerSyntax + " markers in your code,\nthen run `drift link <marker> <module.spec>` to connect them."
	}

	var sb strings.Builder

	if len(state.Todos) == 0 {
		sb.WriteString(fmt.Sprintf("No changes detected. %d specs, %d markers, %d edges in sync.", len(state.Specs), len(state.Markers), len(state.Edges)))
	} else {
		// Bucket todos by kind for the summary line.
		edgeDrift, cascade, edgeAdded, edgeRemoved, broken := 0, 0, 0, 0, 0
		changedMarkers := make(map[string]bool)
		changedSpecs := make(map[string]bool)
		for _, todo := range state.Todos {
			switch todo.Kind {
			case core.TodoEdgeDrift:
				edgeDrift++
				if todo.FromChanged {
					changedMarkers[todo.From] = true
				}
				if todo.ToChanged {
					changedSpecs[todo.To] = true
				}
			case core.TodoCascade:
				cascade++
			case core.TodoEdgeAdded:
				edgeAdded++
			case core.TodoEdgeRemoved:
				edgeRemoved++
			case core.TodoBrokenEdge:
				broken++
			}
		}

		var parts []string
		if edgeDrift > 0 {
			if n := len(changedMarkers); n > 0 {
				if n == 1 {
					parts = append(parts, "1 marker has unchecked changes")
				} else {
					parts = append(parts, fmt.Sprintf("%d markers have unchecked changes", n))
				}
			}
			if n := len(changedSpecs); n > 0 {
				if n == 1 {
					parts = append(parts, "1 spec item has unchecked changes")
				} else {
					parts = append(parts, fmt.Sprintf("%d spec items have unchecked changes", n))
				}
			}
		}
		if cascade > 0 {
			parts = append(parts, fmt.Sprintf("%d cascade drift item(s)", cascade))
		}
		if edgeAdded > 0 {
			parts = append(parts, fmt.Sprintf("%d new edge(s)", edgeAdded))
		}
		if edgeRemoved > 0 {
			parts = append(parts, fmt.Sprintf("%d removed edge(s)", edgeRemoved))
		}
		if broken > 0 {
			parts = append(parts, fmt.Sprintf("%d broken edge(s)", broken))
		}
		if len(parts) > 0 {
			sb.WriteString(strings.Join(parts, ", ") + ".\n")
		}

		sb.WriteString("\n")

		for i, todo := range state.Todos {
			sb.WriteString(p.formatTodo(i+1, todo))
		}
	}

	if warning := unlinkedMarkerWarning(state); warning != "" {
		sb.WriteString("\n")
		sb.WriteString(warning)
	}

	return strings.TrimRight(sb.String(), "\n")
}

// formatTodo renders a single todo entry, dispatching on Kind.
func (p PlainPresenter) formatTodo(n int, todo core.Todo) string {
	var sb strings.Builder
	switch todo.Kind {
	case core.TodoEdgeDrift:
		if todo.SourceSpecID != "" {
			// Chain-drift: From is the chain-drifted spec, To is the changed source.
			sb.WriteString(fmt.Sprintf("%d. [EDGE-DRIFT] Spec %q in %q drifted because transitively-connected spec %q in %q changed. Review whether this spec still aligns with the new upstream. Once satisfied, run `drift reset %s %s`.\n",
				n, todo.From, todo.FromFilepath, todo.To, todo.ToFilepath, todo.From, todo.To))
		} else {
			sb.WriteString(p.formatDirectEdgeTodo(n, todo))
		}
	case core.TodoCascade:
		markLoc := todo.FromFilepath + ":" + strconv.Itoa(todo.FromLineNumber)
		sb.WriteString(fmt.Sprintf("%d. [CASCADE] Marker %q in %q drifted because transitively-connected spec %q in %q changed. Resolve the upstream edge drift to clear this; cascade drift is derived, not independently resettable. (Upstream reset: `drift reset <spec> %s`.)\n",
			n, todo.From, markLoc, todo.SourceSpecID, todo.SourceFilepath, todo.SourceSpecID))
	case core.TodoEdgeAdded:
		sb.WriteString(fmt.Sprintf("%d. [EDGE-ADDED] New edge declared: %q → %q. Confirm the new edge is intentional. Once satisfied, run `drift reset %s %s`.\n",
			n, todo.From, todo.To, todo.From, todo.To))
	case core.TodoEdgeRemoved:
		sb.WriteString(fmt.Sprintf("%d. [EDGE-REMOVED] Edge removed: %q no longer points to %q. Confirm the removal is intentional. Once satisfied, run `drift reset %s %s`.\n",
			n, todo.From, todo.To, todo.From, todo.To))
	case core.TodoBrokenEdge:
		sb.WriteString(fmt.Sprintf("%d. [BROKEN-EDGE] Spec %q refs %q, but no node with that ID exists. Fix the target in the spec text, or restore the missing spec. Once fixed, run `drift reset %s %s`.\n",
			n, todo.From, todo.To, todo.From, todo.To))
	default:
		sb.WriteString(fmt.Sprintf("%d. [UNKNOWN] Unrecognized todo kind %v.\n", n, todo.Kind))
	}
	return sb.String()
}

// formatDirectEdgeTodo renders a TodoEdgeDrift where From and To are direct
// baseline edge endpoints. The classic edge-drift view, endpoint-aware so
// spec-spec edges display correctly (not as "marker").
func (p PlainPresenter) formatDirectEdgeTodo(n int, todo core.Todo) string {
	var driftDescription string
	switch {
	case todo.ToDeleted:
		driftDescription = "The spec term has been deleted from disk. If this was intentional, run the reset command below to acknowledge the removal."
	case todo.FromDeleted:
		driftDescription = "The marker has been deleted from disk. If this was intentional, run the reset command below to acknowledge the removal."
	case todo.FromChanged && todo.ToChanged:
		driftDescription = "Both endpoints have changed. Please check whether the two sides still align and make any modifications necessary on either side."
	case todo.FromChanged:
		driftDescription = "The From endpoint has changed but not the To endpoint. Please check whether the changed side still aligns with the other."
	default:
		driftDescription = "The To endpoint has changed but not the From endpoint. Please check whether the new version is still reflected on the other side."
	}

	fromLocation := todo.FromFilepath + ":" + strconv.Itoa(todo.FromLineNumber)
	toLocation := todo.ToFilepath + ":" + strconv.Itoa(todo.ToLineNumber)

	// Endpoint-aware wording. For link-style edges (marker → spec) preserve
	// the original "marker / spec term" wording. For spec-spec edges use the
	// neutral "spec / spec" wording.
	fromLabel, toLabel := "marker", "spec term"
	if isSpecIDOutput(todo.From) {
		fromLabel = "spec"
	}
	if !isSpecIDOutput(todo.To) {
		toLabel = "marker"
	}

	return fmt.Sprintf("%d. [TODO] Edge between %s %q in %q and %s %q in %q. %s Once you are satisfied, run `drift reset %s %s` to mark this todo item as complete.\n  → Run 'drift diff %s %s' to see what changed.\n",
		n, fromLabel, todo.From, fromLocation, toLabel, todo.To, toLocation, driftDescription, todo.From, todo.To, todo.From, todo.To)
}

// isSpecIDOutput reports whether id looks like a module-qualified spec ID
// (contains exactly one dot). Used by presenters to choose wording.
func isSpecIDOutput(id string) bool {
	first := strings.Index(id, ".")
	if first < 0 {
		return false
	}
	return strings.Index(id[first+1:], ".") < 0
}

// unlinkedMarkerWarning returns the one-line warning summary for non-deleted
// markers that have no link-style edges, or "" when there are none.
func unlinkedMarkerWarning(state core.EvaluatedState) string {
	linkedMarkers := make(map[string]bool)
	for _, e := range state.Edges {
		// Link-style edge: From is marker.
		if isSpecID(e.From) {
			continue
		}
		linkedMarkers[e.From] = true
	}
	unlinked := 0
	for _, m := range state.Markers {
		if m.Deleted {
			continue
		}
		if !linkedMarkers[m.ID] {
			unlinked++
		}
	}
	if unlinked == 0 {
		return ""
	}
	if unlinked == 1 {
		return "1 unlinked marker found — run `drift list` to review."
	}
	return fmt.Sprintf("%d unlinked markers found — run `drift list` to review.", unlinked)
}

// D! id=cfmt range-end

// D! id=ofmtl range-start
func (p PlainPresenter) List(r ListResult) string {
	state := r.State
	if len(state.Specs) == 0 && len(state.Markers) == 0 {
		return "No specs or markers registered.\nRun `drift init` to get started, then create spec files (*.drift.xml) and place " + markerSyntax + " markers in your code."
	}

	driftedEdges := make(map[string]bool)
	for _, todo := range state.Todos {
		if todo.Kind == core.TodoEdgeDrift && todo.SourceSpecID == "" {
			driftedEdges[todo.From+"\x00"+todo.To] = true
		}
	}

	linkedSpecs := make(map[string]bool)
	linkedMarkers := make(map[string]bool)
	for _, e := range state.Edges {
		if isSpecID(e.From) {
			continue // ref-style edge; doesn't count for "unlinked"
		}
		linkedMarkers[e.From] = true
		linkedSpecs[e.To] = true
	}

	var sb strings.Builder

	sortedSpecs := make([]core.Spec, len(state.Specs))
	copy(sortedSpecs, state.Specs)
	sortSpecsByID(sortedSpecs)

	sb.WriteString(fmt.Sprintf("Specs (%d):\n", len(sortedSpecs)))
	for _, spec := range sortedSpecs {
		linkFlag := ""
		if spec.Deleted {
			linkFlag = " [deleted]"
		} else if !linkedSpecs[spec.ID] {
			linkFlag = " [unlinked]"
		}
		sb.WriteString(fmt.Sprintf("  %-30s %s%s\n", spec.ID, spec.Filepath, linkFlag))
		if r.Verbose && !spec.Deleted {
			if content, ok := r.SpecContents[spec.ID]; ok && len(content) > 0 {
				preview := content
				if len(preview) > 80 {
					preview = preview[:80] + "..."
				}
				sb.WriteString(fmt.Sprintf("    %s\n", preview))
			}
		}
	}

	sortedMarkers := make([]core.Marker, len(state.Markers))
	copy(sortedMarkers, state.Markers)
	sortMarkersByID(sortedMarkers)

	sb.WriteString(fmt.Sprintf("\nMarkers (%d):\n", len(sortedMarkers)))
	for _, marker := range sortedMarkers {
		linkFlag := ""
		if marker.Deleted {
			linkFlag = " [deleted]"
		} else if !linkedMarkers[marker.ID] {
			linkFlag = " [unlinked]"
		}
		sb.WriteString(fmt.Sprintf("  %-30s %s:%d-%d%s\n", marker.ID, marker.Filepath, marker.LineNumber, marker.EndLineNumber, linkFlag))
		if r.Verbose && !marker.Deleted {
			if content, ok := r.MarkerContents[marker.ID]; ok && len(content) > 0 {
				firstLine := strings.Split(content, "\n")[0]
				if len(firstLine) > 80 {
					firstLine = firstLine[:80] + "..."
				}
				if firstLine != "" {
					sb.WriteString(fmt.Sprintf("    %s\n", firstLine))
				}
			}
		}
	}

	if len(state.Edges) > 0 {
		sb.WriteString(fmt.Sprintf("\nEdges (%d):\n", len(state.Edges)))
		for _, e := range state.Edges {
			status := "[synced]"
			if driftedEdges[e.From+"\x00"+e.To] || driftedEdges[e.To+"\x00"+e.From] {
				status = "[DRIFTED]"
			}
			sb.WriteString(fmt.Sprintf("  %-30s → %-30s %s\n", e.From, e.To, status))
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

func sortSpecsByID(specs []core.Spec) {
	for i := 1; i < len(specs); i++ {
		for j := i; j > 0 && specs[j-1].ID > specs[j].ID; j-- {
			specs[j], specs[j-1] = specs[j-1], specs[j]
		}
	}
}

func sortMarkersByID(markers []core.Marker) {
	for i := 1; i < len(markers); i++ {
		for j := i; j > 0 && markers[j-1].ID > markers[j].ID; j-- {
			markers[j], markers[j-1] = markers[j-1], markers[j]
		}
	}
}

// D! id=ofmtl range-end

func (p PlainPresenter) Show(r ShowResult) string {
	if r.IsSpec {
		if r.Spec == nil {
			return fmt.Sprintf("spec %q not found", r.ID)
		}
		return p.showSpec(r)
	}
	if r.Marker == nil {
		return fmt.Sprintf("marker %q not found", r.ID)
	}
	return p.showMarker(r)
}

func (p PlainPresenter) showSpec(r ShowResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("=== Spec: %s ===\n", r.ID))
	sb.WriteString(fmt.Sprintf("File: %s\n", r.Spec.Filepath))
	sb.WriteString(fmt.Sprintf("Hash: %s\n\n", r.Spec.Hash))
	sb.WriteString(r.Content)
	sb.WriteString("\n")

	if len(r.OutboundRefs) > 0 || len(r.InboundRefs) > 0 {
		sb.WriteString("\n")
		if len(r.OutboundRefs) > 0 {
			sb.WriteString(fmt.Sprintf("Outbound refs (%d): %s\n", len(r.OutboundRefs), strings.Join(r.OutboundRefs, ", ")))
		}
		if len(r.InboundRefs) > 0 {
			sb.WriteString(fmt.Sprintf("Inbound refs (%d): %s\n", len(r.InboundRefs), strings.Join(r.InboundRefs, ", ")))
		}
	}

	for _, m := range r.LinkedMarkers {
		sb.WriteString(fmt.Sprintf("\n=== Marker: %s ===\n", m.Marker.ID))
		sb.WriteString(fmt.Sprintf("File: %s\n", m.Marker.Filepath))
		sb.WriteString(fmt.Sprintf("Lines: %d-%d\n", m.Marker.LineNumber, m.Marker.EndLineNumber))
		sb.WriteString(fmt.Sprintf("Hash: %s\n\n", m.Marker.Hash))
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

func (p PlainPresenter) showMarker(r ShowResult) string {
	var sb strings.Builder

	for _, s := range r.LinkedSpecs {
		sb.WriteString(fmt.Sprintf("=== Spec: %s ===\n", s.Spec.ID))
		sb.WriteString(fmt.Sprintf("File: %s\n", s.Spec.Filepath))
		sb.WriteString(fmt.Sprintf("Hash: %s\n\n", s.Spec.Hash))
		sb.WriteString(s.Content)
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf("=== Marker: %s ===\n", r.ID))
	sb.WriteString(fmt.Sprintf("File: %s\n", r.Marker.Filepath))
	sb.WriteString(fmt.Sprintf("Lines: %d-%d\n", r.Marker.LineNumber, r.Marker.EndLineNumber))
	sb.WriteString(fmt.Sprintf("Hash: %s\n\n", r.Marker.Hash))
	sb.WriteString(r.Content)

	return strings.TrimRight(sb.String(), "\n")
}

// D! id=cdifffmt range-start
func (p PlainPresenter) DiffEdge(r DiffEdgeResult) string {
	var sb strings.Builder
	sb.WriteString(p.formatDiffSide("Spec", r.Result.Spec))
	sb.WriteString("\n---\n")
	sb.WriteString(p.formatDiffSide("Marker", r.Result.Marker))
	return strings.TrimRight(sb.String(), "\n")
}

func (p PlainPresenter) formatDiffSide(label string, side orchestrator.DiffSide) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s: %s", label, side.ID))
	if side.Filepath != "" {
		sb.WriteString(fmt.Sprintf(" (%s", side.Filepath))
		if side.Lines != "" {
			sb.WriteString(":" + side.Lines)
		}
		sb.WriteString(")")
	}
	sb.WriteString("\n")

	if side.Deleted {
		sb.WriteString("Status: deleted from disk\n")
	} else if !side.HasBaseline {
		sb.WriteString(fmt.Sprintf("Status: no baseline snapshot (hash %s)\n", side.BaselineHash))
	} else if side.BaselineHash == side.CurrentHash && side.CurrentHash != "" {
		sb.WriteString("Status: in sync\n")
	} else {
		sb.WriteString(fmt.Sprintf("Baseline: %s   Current: %s\n", side.BaselineHash, side.CurrentHash))
	}

	if !side.HasBaseline {
		return strings.TrimRight(sb.String(), "\n")
	}
	if side.Baseline == side.Current {
		return strings.TrimRight(sb.String(), "\n")
	}

	sb.WriteString("\n--- baseline\n+++ current\n")
	patch := diff.UnifiedDiff(side.Baseline, side.Current)
	if patch != "" {
		sb.WriteString(patch)
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (p PlainPresenter) DiffExpanded(r DiffExpandedResult) string {
	var sb strings.Builder
	for i, edge := range r.Edges {
		if i > 0 {
			sb.WriteString("\n\n===\n\n")
		}
		out := p.DiffEdge(DiffEdgeResult{Result: edge})
		sb.WriteString(out)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// D! id=cdall range-start
func (p PlainPresenter) DiffAll(r DiffAllResult) string {
	if len(r.Edges) == 0 {
		return "No drift detected."
	}

	var sb strings.Builder
	for i, edge := range r.Edges {
		if i > 0 {
			sb.WriteString("\n\n===\n\n")
		}
		out := p.DiffEdge(DiffEdgeResult{Result: edge})
		sb.WriteString(out)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// D! id=cdall range-end
// D! id=cdifffmt range-end

func (p PlainPresenter) Ok(r OkResult) string {
	return r.Message
}

func (p PlainPresenter) Error(r ErrorResult) string {
	if r.Hint != "" {
		return r.Message + "\n" + r.Hint
	}
	return r.Message
}

func (p PlainPresenter) Text(r TextResult) string {
	return r.Text
}

func (p PlainPresenter) Version(r VersionResult) string {
	return "drift version " + r.Version
}
