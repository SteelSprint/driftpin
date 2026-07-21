package output

import (
	"fmt"
	"sort"
	"strings"

	"drift/core"
	"drift/internal/diff"
	"drift/orchestrator"
)

// D! id=oplain range-start

// PlainPresenter is the byte-identical continuation of pre-output-layer output.
type PlainPresenter struct{}

// markerSyntax is the user-facing shorthand for marker comments.
var markerSyntax = "D" + "! id=<markerid>"

// D! id=oplain range-end

// D! id=cfmt range-start
func (p PlainPresenter) Todo(r TodoResult) string {
	state := r.State
	if len(state.Specs) == 0 && len(state.Markers) == 0 {
		return "Nothing to check: no specs or markers registered.\nCreate spec files (*.drift.xml) and place " + markerSyntax + " markers in your code,\nthen run `drift link <marker> <module.spec>` to connect them."
	}

	var sb strings.Builder

	if len(state.Closures) == 0 {
		sb.WriteString(fmt.Sprintf("No changes detected. %d specs, %d markers, %d edges in sync.", len(state.Specs), len(state.Markers), len(state.Edges)))
	} else {
		sb.WriteString(fmt.Sprintf("%d closure(s) with drift.\n\n", len(state.Closures)))
		for _, c := range state.Closures {
			sb.WriteString(p.formatClosure(c))
		}
	}

	if warning := unlinkedMarkerWarning(state); warning != "" {
		sb.WriteString("\n")
		sb.WriteString(warning)
	}

	return strings.TrimRight(sb.String(), "\n")
}

func (p PlainPresenter) formatClosure(c core.Closure) string {
	var sb strings.Builder
	specNodes, markerNodes := 0, 0
	for _, n := range c.Nodes {
		if n.IsSpec {
			specNodes++
		} else {
			markerNodes++
		}
	}
	sb.WriteString(fmt.Sprintf("Closure %s  (%d nodes: %d specs, %d markers; %d edges)\n",
		c.Hash, len(c.Nodes), specNodes, markerNodes, len(c.Edges)))
	sb.WriteString("  Events:\n")
	for _, ev := range c.Events {
		sb.WriteString("    " + p.formatEvent(ev) + "\n")
	}
	if len(c.Nodes) > 0 {
		sb.WriteString("  Members:\n")
		// Re-sort for display: specs first then markers, alphabetical.
		var specs, markers []core.NodeRef
		for _, n := range c.Nodes {
			if n.IsSpec {
				specs = append(specs, n)
			} else {
				markers = append(markers, n)
			}
		}
		if len(specs) > 0 {
			sb.WriteString("    specs:   ")
			names := make([]string, 0, len(specs))
			for _, n := range specs {
				names = append(names, n.ID)
			}
			sb.WriteString(strings.Join(names, ", ") + "\n")
		}
		if len(markers) > 0 {
			sb.WriteString("    markers: ")
			names := make([]string, 0, len(markers))
			for _, n := range markers {
				names = append(names, n.ID)
			}
			sb.WriteString(strings.Join(names, ", ") + "\n")
		}
	}
	sb.WriteString(fmt.Sprintf("  Inspect: drift diff %s\n", c.Hash))
	sb.WriteString(fmt.Sprintf("  Resolve: drift reset %s\n", c.Hash))
	sb.WriteString("\n")
	return sb.String()
}

func (p PlainPresenter) formatEvent(ev core.DriftEvent) string {
	kindLabel := eventKindLabel(ev.Kind)
	switch ev.Kind {
	case core.EventNodeChanged:
		return fmt.Sprintf("[%s] %s %q  baseline: %s → scan: %s",
			kindLabel, nodeKindFor(ev.NodeID), ev.NodeID,
			shortHash(ev.OldHash), shortHash(ev.NewHash))
	case core.EventNodeAdded:
		return fmt.Sprintf("[%s] %s %q", kindLabel, nodeKindFor(ev.NodeID), ev.NodeID)
	case core.EventNodeRemoved:
		return fmt.Sprintf("[%s] %s %q", kindLabel, nodeKindFor(ev.NodeID), ev.NodeID)
	case core.EventEdgeAdded:
		if ev.Edge != nil {
			return fmt.Sprintf("[%s] new edge declared: %q → %q", kindLabel, ev.Edge.From, ev.Edge.To)
		}
	case core.EventEdgeRemoved:
		if ev.Edge != nil {
			return fmt.Sprintf("[%s] edge removed: %q → %q", kindLabel, ev.Edge.From, ev.Edge.To)
		}
	case core.EventEdgeBroken:
		if ev.Edge != nil {
			return fmt.Sprintf("[%s] edge to nonexistent node: %q → %q (fix scan: add missing spec or remove the ref)", kindLabel, ev.Edge.From, ev.Edge.To)
		}
	}
	return fmt.Sprintf("[%s] unknown event", kindLabel)
}

func eventKindLabel(k core.EventKind) string {
	switch k {
	case core.EventNodeChanged:
		return "NODE-CHANGED"
	case core.EventNodeAdded:
		return "NODE-ADDED"
	case core.EventNodeRemoved:
		return "NODE-REMOVED"
	case core.EventEdgeAdded:
		return "EDGE-ADDED"
	case core.EventEdgeRemoved:
		return "EDGE-REMOVED"
	case core.EventEdgeBroken:
		return "BROKEN-EDGE"
	}
	return "UNKNOWN"
}

func nodeKindFor(id string) string {
	if isSpecIDOutput(id) {
		return "spec"
	}
	return "marker"
}

func shortHash(h string) string {
	if len(h) <= 8 {
		return h
	}
	return h[:8]
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
		if isSpecIDOutput(e.From) {
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

	driftedNodes := make(map[string]bool)
	for _, c := range state.Closures {
		for _, n := range c.Nodes {
			driftedNodes[n.ID] = true
		}
	}

	linkedSpecs := make(map[string]bool)
	linkedMarkers := make(map[string]bool)
	for _, e := range state.Edges {
		if isSpecIDOutput(e.From) {
			continue
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
		if driftedNodes[spec.ID] {
			linkFlag += " [DRIFTED]"
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
		if driftedNodes[marker.ID] {
			linkFlag += " [DRIFTED]"
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
		sortedEdges := append([]core.Edge(nil), state.Edges...)
		sortEdgesByFromTo(sortedEdges)
		sb.WriteString(fmt.Sprintf("\nEdges (%d):\n", len(sortedEdges)))
		for _, e := range sortedEdges {
			status := "[synced]"
			if driftedNodes[e.From] || driftedNodes[e.To] {
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
	var sb strings.Builder

	// Locate seed node.
	var seed *ShowNode
	for i := range r.Nodes {
		if r.Nodes[i].ID == r.ID {
			seed = &r.Nodes[i]
			break
		}
	}
	if seed == nil {
		return fmt.Sprintf("%s %q not found", map[bool]string{true: "spec", false: "marker"}[r.IsSpec], r.ID)
	}

	// Classify specs as ancestors (transitively cite the seed), descendants
	// (transitively cited by the seed), or the seed itself. Markers are
	// printed in their own section.
	ancestors, descendants := classifyClosureSpecs(r, r.ID)

	sb.WriteString(fmt.Sprintf("=== Citation closure: %s ===\n", r.ID))
	sb.WriteString(fmt.Sprintf("Seed: %s (%s)\n", r.ID, seed.Filepath))
	if seed.Lines != "" {
		sb.WriteString(fmt.Sprintf("Lines: %s\n", seed.Lines))
	}
	sb.WriteString(fmt.Sprintf("Hash: %s\n", seed.Hash))
	if seed.Deleted {
		sb.WriteString("Status: deleted from disk\n")
	}
	sb.WriteString("\n--- Seed content ---\n")
	sb.WriteString(seed.Content)

	if len(ancestors) > 0 {
		sb.WriteString(fmt.Sprintf("\n\n=== Ancestors (%d, specs that transitively cite %s) ===\n", len(ancestors), r.ID))
		for _, n := range ancestors {
			renderSpecSection(&sb, n)
		}
	}

	if len(descendants) > 0 {
		sb.WriteString(fmt.Sprintf("\n=== Descendants (%d, specs that %s transitively cites) ===\n", len(descendants), r.ID))
		for _, n := range descendants {
			renderSpecSection(&sb, n)
		}
	}

	// Markers in closure (excluding the seed when seed is itself a marker).
	var markers []ShowNode
	for _, n := range r.Nodes {
		if n.Kind != "marker" {
			continue
		}
		if n.ID == r.ID {
			continue
		}
		markers = append(markers, n)
	}
	if len(markers) > 0 {
		sb.WriteString(fmt.Sprintf("\n=== Markers in closure (%d) ===\n", len(markers)))
		for _, m := range markers {
			renderMarkerSection(&sb, m)
		}
	}

	if len(r.Edges) > 0 {
		sb.WriteString(fmt.Sprintf("\n=== Edges (%d total) ===\n", len(r.Edges)))
		for _, e := range r.Edges {
			sb.WriteString(fmt.Sprintf("%s → %s\n", e.From, e.To))
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// classifyClosureSpecs walks the closure edges from the seed in both directions
// to identify which specs are ancestors (can reach the seed via outgoing edges)
// and which are descendants (reachable from the seed via outgoing edges).
// Returns ancestor and descendant node lists, sorted by ID, excluding the seed.
func classifyClosureSpecs(r ShowResult, seedID string) (ancestors, descendants []ShowNode) {
	outgoing := map[string]map[string]bool{}
	incoming := map[string]map[string]bool{}
	for _, e := range r.Edges {
		if outgoing[e.From] == nil {
			outgoing[e.From] = map[string]bool{}
		}
		outgoing[e.From][e.To] = true
		if incoming[e.To] == nil {
			incoming[e.To] = map[string]bool{}
		}
		incoming[e.To][e.From] = true
	}

	// Descendants: BFS from seed over outgoing edges.
	desc := map[string]bool{}
	queue := []string{seedID}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for next := range outgoing[curr] {
			if !desc[next] && next != seedID {
				desc[next] = true
				queue = append(queue, next)
			}
		}
	}
	// Ancestors: BFS from seed over incoming edges.
	anc := map[string]bool{}
	queue = []string{seedID}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for next := range incoming[curr] {
			if !anc[next] && next != seedID {
				anc[next] = true
				queue = append(queue, next)
			}
		}
	}

	nodeByID := map[string]ShowNode{}
	for _, n := range r.Nodes {
		if n.Kind == "spec" {
			nodeByID[n.ID] = n
		}
	}
	for id := range anc {
		if n, ok := nodeByID[id]; ok {
			ancestors = append(ancestors, n)
		}
	}
	for id := range desc {
		if n, ok := nodeByID[id]; ok {
			descendants = append(descendants, n)
		}
	}
	sort.Slice(ancestors, func(i, j int) bool { return ancestors[i].ID < ancestors[j].ID })
	sort.Slice(descendants, func(i, j int) bool { return descendants[i].ID < descendants[j].ID })
	return ancestors, descendants
}

func renderSpecSection(sb *strings.Builder, n ShowNode) {
	sb.WriteString(fmt.Sprintf("\n%s (%s)\n", n.ID, n.Filepath))
	sb.WriteString(fmt.Sprintf("Hash: %s\n", n.Hash))
	if n.Deleted {
		sb.WriteString("Status: deleted from disk\n")
		return
	}
	sb.WriteString("--- content ---\n")
	sb.WriteString(n.Content)
	sb.WriteString("\n")
}

func renderMarkerSection(sb *strings.Builder, n ShowNode) {
	sb.WriteString(fmt.Sprintf("\n%s (%s:%s)\n", n.ID, n.Filepath, n.Lines))
	sb.WriteString(fmt.Sprintf("Hash: %s\n", n.Hash))
	if n.Deleted {
		sb.WriteString("Status: deleted from disk\n")
		return
	}
	sb.WriteString("--- content ---\n")
	sb.WriteString(n.Content)
	sb.WriteString("\n")
}

// D! id=cdifffmt range-start
func (p PlainPresenter) DiffClosure(r DiffClosureResult) string {
	if len(r.Diffs) == 0 {
		return fmt.Sprintf("Closure %s: no diffable content.", r.Hash)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== Closure %s ===\n\n", r.Hash))
	for i, d := range r.Diffs {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		if d.Spec != nil {
			sb.WriteString(p.formatDiffSide("Spec", *d.Spec, d.IsSeed))
		} else if d.Marker != nil {
			sb.WriteString(p.formatDiffSide("Marker", *d.Marker, d.IsSeed))
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (p PlainPresenter) formatDiffSide(label string, side orchestrator.DiffSide, isSeed bool) string {
	roleLabel := "[citer]"
	if isSeed {
		roleLabel = "[SEED]"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s: %s %s", label, side.ID, roleLabel))
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

// D! id=cdall range-start
func (p PlainPresenter) DiffAll(r DiffAllResult) string {
	if len(r.Closures) == 0 {
		return "No drift detected."
	}

	var sb strings.Builder
	for i, c := range r.Closures {
		if i > 0 {
			sb.WriteString("\n\n===\n\n")
		}
		sb.WriteString(fmt.Sprintf("=== Closure %s ===\n\n", c.Hash))
		for j, d := range c.Diffs {
			if j > 0 {
				sb.WriteString("\n---\n\n")
			}
			if d.Spec != nil {
				sb.WriteString(p.formatDiffSide("Spec", *d.Spec, d.IsSeed))
			} else if d.Marker != nil {
				sb.WriteString(p.formatDiffSide("Marker", *d.Marker, d.IsSeed))
			}
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// D! id=cdall range-end
// D! id=cdifffmt range-end

// D! id=ocsum range-start
// formatChangeSummary renders a ChangeSummary as a stable text block.
// Used by both --dry-run preview (with banner) and post-apply printing
// (with optional Message lead-in). Hashes are truncated to 8 chars.
func formatChangeSummary(r ChangeSummaryResult) string {
	var sb strings.Builder
	if r.Preview {
		sb.WriteString("Preview — no changes written\n")
	}
	if r.Message != "" {
		sb.WriteString(r.Message + "\n")
	}
	sb.WriteString(fmt.Sprintf("  operation: %s\n", r.Summary.Operation))
	for _, nc := range r.Summary.NodeChanges {
		old := shortHash(nc.OldHash)
		new := shortHash(nc.NewHash)
		switch nc.Kind {
		case "changed":
			sb.WriteString(fmt.Sprintf("  %-8s %s  %s → %s\n", nc.Kind, nc.ID, old, new))
		case "added":
			sb.WriteString(fmt.Sprintf("  %-8s %s  → %s\n", nc.Kind, nc.ID, new))
		case "removed":
			sb.WriteString(fmt.Sprintf("  %-8s %s  %s →\n", nc.Kind, nc.ID, old))
		}
	}
	for _, ec := range r.Summary.EdgeChanges {
		sb.WriteString(fmt.Sprintf("  edge %-8s %s → %s\n", ec.Kind, ec.From, ec.To))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (p PlainPresenter) ChangeSummary(r ChangeSummaryResult) string {
	return formatChangeSummary(r)
}

// D! id=ocsum range-end

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
