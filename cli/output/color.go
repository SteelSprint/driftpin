package output

import (
	"fmt"
	"strings"

	"drift/core"
	"drift/internal/diff"
	"drift/orchestrator"
)

// D! id=ocol range-start
// ColorPresenter formats Result data using a Theme.
type ColorPresenter struct {
	Theme Theme
}

var _ Presenter = ColorPresenter{Theme: DefaultTheme}

// colorizeCode applies syntax highlighting to a single line of code using the
// theme's code elements.
func (p ColorPresenter) colorizeCode(line string) string {
	t := p.Theme
	tokens := tokenizeLine(line)
	var sb strings.Builder
	for _, tok := range tokens {
		switch tok.Type {
		case "comment":
			sb.WriteString(t.CodeComment.Apply(tok.Text))
		case "string":
			sb.WriteString(t.CodeString.Apply(tok.Text))
		case "keyword":
			sb.WriteString(t.CodeKeyword.Apply(tok.Text))
		case "number":
			sb.WriteString(t.CodeNumber.Apply(tok.Text))
		default:
			sb.WriteString(tok.Text)
		}
	}
	return sb.String()
}

// colorizeCodeBlock applies syntax highlighting to multi-line content.
func (p ColorPresenter) colorizeCodeBlock(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = p.colorizeCode(line)
	}
	return strings.Join(lines, "\n")
}

// colorizePatch applies theme colors to unified diff lines.
func (p ColorPresenter) colorizePatch(patch string) string {
	t := p.Theme
	lines := strings.Split(patch, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "@@"):
			lines[i] = t.DiffHunk.Apply(line)
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			content := p.colorizeCode(line[1:])
			lines[i] = t.DiffAdd.Apply("+" + content)
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			content := p.colorizeCode(line[1:])
			lines[i] = t.DiffRemove.Apply("-" + content)
		}
	}
	return strings.Join(lines, "\n")
}

// --- Todo ---

func (p ColorPresenter) Todo(r TodoResult) string {
	t := p.Theme
	state := r.State
	if len(state.Specs) == 0 && len(state.Markers) == 0 {
		return "Nothing to check: no specs or markers registered.\nCreate spec files (*.drift.xml) and place " + markerSyntax + " markers in your code,\nthen run `drift link <marker> <module.spec>` to connect them."
	}

	var sb strings.Builder

	if len(state.Closures) == 0 {
		sb.WriteString(t.StatusOK.Apply(fmt.Sprintf("No changes detected. %d specs, %d markers, %d edges in sync.", len(state.Specs), len(state.Markers), len(state.Edges))))
	} else {
		sb.WriteString(t.StatusWarn.Apply(fmt.Sprintf("%d closure(s) with drift.", len(state.Closures))) + "\n\n")
		for _, c := range state.Closures {
			sb.WriteString(p.formatClosureColor(c))
		}
	}

	if warning := unlinkedMarkerWarning(state); warning != "" {
		sb.WriteString("\n")
		sb.WriteString(t.StatusWarn.Apply(warning))
	}

	return strings.TrimRight(sb.String(), "\n")
}

func (p ColorPresenter) formatClosureColor(c core.Closure) string {
	t := p.Theme
	var sb strings.Builder
	specNodes, markerNodes := 0, 0
	for _, n := range c.Nodes {
		if n.IsSpec {
			specNodes++
		} else {
			markerNodes++
		}
	}
	sb.WriteString(fmt.Sprintf("%s %s  (%d nodes: %d specs, %d markers; %d edges)\n",
		t.SectionHeader.Apply("Closure"),
		t.Hash.Apply(c.Hash),
		len(c.Nodes), specNodes, markerNodes, len(c.Edges)))
	sb.WriteString(fmt.Sprintf("  %s\n", t.SectionHeader.Apply("Events:")))
	for _, ev := range c.Events {
		sb.WriteString("    " + p.formatEventColor(ev) + "\n")
	}
	if len(c.Nodes) > 0 {
		sb.WriteString(fmt.Sprintf("  %s\n", t.SectionHeader.Apply("Members:")))
		var specs, markers []string
		for _, n := range c.Nodes {
			if n.IsSpec {
				specs = append(specs, n.ID)
			} else {
				markers = append(markers, n.ID)
			}
		}
		if len(specs) > 0 {
			sb.WriteString(fmt.Sprintf("    %s %s\n", t.SectionHeader.Apply("specs:  "), t.SpecID.Apply(strings.Join(specs, ", "))))
		}
		if len(markers) > 0 {
			sb.WriteString(fmt.Sprintf("    %s %s\n", t.SectionHeader.Apply("markers:"), t.MarkerID.Apply(strings.Join(markers, ", "))))
		}
	}
	sb.WriteString(fmt.Sprintf("  %s %s\n", t.SectionHeader.Apply("Inspect:"), t.Command.Apply("drift diff "+c.Hash)))
	sb.WriteString(fmt.Sprintf("  %s %s\n", t.SectionHeader.Apply("Resolve:"), t.Command.Apply("drift reset "+c.Hash)))
	sb.WriteString("\n")
	return sb.String()
}

func (p ColorPresenter) formatEventColor(ev core.DriftEvent) string {
	t := p.Theme
	kindLabel := eventKindLabel(ev.Kind)
	var labelStyled string
	switch ev.Kind {
	case core.EventEdgeBroken:
		labelStyled = t.StatusError.Apply("[" + kindLabel + "]")
	default:
		labelStyled = t.StatusWarn.Apply("[" + kindLabel + "]")
	}
	idStyled := t.MarkerID.Apply(fmt.Sprintf("%q", ev.NodeID))
	if isSpecIDOutput(ev.NodeID) {
		idStyled = t.SpecID.Apply(fmt.Sprintf("%q", ev.NodeID))
	}
	switch ev.Kind {
	case core.EventNodeChanged:
		return fmt.Sprintf("%s %s %s  baseline: %s → scan: %s",
			labelStyled, t.SectionHeader.Apply(nodeKindFor(ev.NodeID)), idStyled,
			t.Hash.Apply(shortHash(ev.OldHash)), t.Hash.Apply(shortHash(ev.NewHash)))
	case core.EventNodeAdded:
		return fmt.Sprintf("%s %s %s", labelStyled, t.SectionHeader.Apply(nodeKindFor(ev.NodeID)), idStyled)
	case core.EventNodeRemoved:
		return fmt.Sprintf("%s %s %s", labelStyled, t.SectionHeader.Apply(nodeKindFor(ev.NodeID)), idStyled)
	case core.EventEdgeAdded:
		if ev.Edge != nil {
			return fmt.Sprintf("%s new edge declared: %s → %s", labelStyled,
				t.SpecID.Apply(fmt.Sprintf("%q", ev.Edge.From)),
				t.SpecID.Apply(fmt.Sprintf("%q", ev.Edge.To)))
		}
	case core.EventEdgeRemoved:
		if ev.Edge != nil {
			return fmt.Sprintf("%s edge removed: %s → %s", labelStyled,
				t.SpecID.Apply(fmt.Sprintf("%q", ev.Edge.From)),
				t.SpecID.Apply(fmt.Sprintf("%q", ev.Edge.To)))
		}
	case core.EventEdgeBroken:
		if ev.Edge != nil {
			return fmt.Sprintf("%s edge to nonexistent node: %s → %s (fix scan: add missing spec or remove the ref)", labelStyled,
				t.SpecID.Apply(fmt.Sprintf("%q", ev.Edge.From)),
				t.SpecID.Apply(fmt.Sprintf("%q", ev.Edge.To)))
		}
	}
	return fmt.Sprintf("%s unknown event", labelStyled)
}

// --- List ---

func (p ColorPresenter) List(r ListResult) string {
	t := p.Theme
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

	sb.WriteString(t.SectionHeader.Apply(fmt.Sprintf("Specs (%d):", len(sortedSpecs))) + "\n")
	for _, spec := range sortedSpecs {
		linkFlag := ""
		if spec.Deleted {
			linkFlag = " " + t.StatusError.Apply("[deleted]")
		} else if !linkedSpecs[spec.ID] {
			linkFlag = " " + t.StatusWarn.Apply("[unlinked]")
		}
		if driftedNodes[spec.ID] {
			linkFlag += " " + t.StatusError.Apply("[DRIFTED]")
		}
		sb.WriteString(fmt.Sprintf("  %s %s%s\n", t.SpecID.Apply(fmt.Sprintf("%-30s", spec.ID)), t.Filepath.Apply(spec.Filepath), linkFlag))
		if r.Verbose && !spec.Deleted {
			if content, ok := r.SpecContents[spec.ID]; ok && len(content) > 0 {
				preview := content
				if len(preview) > 80 {
					preview = preview[:80] + "..."
				}
				sb.WriteString(fmt.Sprintf("    %s\n", p.colorizeCode(preview)))
			}
		}
	}

	sortedMarkers := make([]core.Marker, len(state.Markers))
	copy(sortedMarkers, state.Markers)
	sortMarkersByID(sortedMarkers)

	sb.WriteString("\n" + t.SectionHeader.Apply(fmt.Sprintf("Markers (%d):", len(sortedMarkers))) + "\n")
	for _, marker := range sortedMarkers {
		linkFlag := ""
		if marker.Deleted {
			linkFlag = " " + t.StatusError.Apply("[deleted]")
		} else if !linkedMarkers[marker.ID] {
			linkFlag = " " + t.StatusWarn.Apply("[unlinked]")
		}
		if driftedNodes[marker.ID] {
			linkFlag += " " + t.StatusError.Apply("[DRIFTED]")
		}
		loc := fmt.Sprintf("%s:%d-%d", marker.Filepath, marker.LineNumber, marker.EndLineNumber)
		sb.WriteString(fmt.Sprintf("  %s %s%s\n", t.MarkerID.Apply(fmt.Sprintf("%-30s", marker.ID)), t.Filepath.Apply(loc), linkFlag))
		if r.Verbose && !marker.Deleted {
			if content, ok := r.MarkerContents[marker.ID]; ok && len(content) > 0 {
				firstLine := strings.Split(content, "\n")[0]
				if len(firstLine) > 80 {
					firstLine = firstLine[:80] + "..."
				}
				if firstLine != "" {
					sb.WriteString(fmt.Sprintf("    %s\n", p.colorizeCode(firstLine)))
				}
			}
		}
	}

	if len(state.Edges) > 0 {
		sortedEdges := append([]core.Edge(nil), state.Edges...)
		sortEdgesByFromTo(sortedEdges)
		sb.WriteString("\n" + t.SectionHeader.Apply(fmt.Sprintf("Edges (%d):", len(sortedEdges))) + "\n")
		for _, e := range sortedEdges {
			var status string
			if driftedNodes[e.From] || driftedNodes[e.To] {
				status = t.StatusError.Apply("[DRIFTED]")
			} else {
				status = t.StatusOK.Apply("[synced]")
			}
			fromStyled := t.MarkerID.Apply(fmt.Sprintf("%-30s", e.From))
			if isSpecIDOutput(e.From) {
				fromStyled = t.SpecID.Apply(fmt.Sprintf("%-30s", e.From))
			}
			toStyled := t.SpecID.Apply(fmt.Sprintf("%-30s", e.To))
			sb.WriteString(fmt.Sprintf("  %s → %s %s\n", fromStyled, toStyled, status))
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// --- Show ---

func (p ColorPresenter) Show(r ShowResult) string {
	t := p.Theme
	var sb strings.Builder

	var seed *ShowNode
	for i := range r.Nodes {
		if r.Nodes[i].ID == r.ID {
			seed = &r.Nodes[i]
			break
		}
	}
	if seed == nil {
		return t.StatusError.Apply(fmt.Sprintf("%s %q not found", map[bool]string{true: "spec", false: "marker"}[r.IsSpec], r.ID))
	}

	ancestors, descendants := classifyClosureSpecs(r, r.ID)

	seedLabel := t.SpecID
	if !r.IsSpec {
		seedLabel = t.MarkerID
	}
	sb.WriteString(t.SectionHeader.Apply(fmt.Sprintf("=== Citation closure: %s ===", seedLabel.Apply(r.ID))) + "\n")
	sb.WriteString(fmt.Sprintf("Seed: %s (%s)\n", seedLabel.Apply(r.ID), t.Filepath.Apply(seed.Filepath)))
	if seed.Lines != "" {
		sb.WriteString(fmt.Sprintf("Lines: %s\n", t.LineNumber.Apply(seed.Lines)))
	}
	sb.WriteString(fmt.Sprintf("Hash: %s\n", t.Hash.Apply(seed.Hash)))
	if seed.Deleted {
		sb.WriteString(t.StatusWarn.Apply("Status: deleted from disk") + "\n")
	}
	sb.WriteString("\n" + t.SectionHeader.Apply("--- Seed content ---") + "\n")
	sb.WriteString(p.colorizeCodeBlock(seed.Content))

	if len(ancestors) > 0 {
		sb.WriteString(fmt.Sprintf("\n\n%s\n", t.SectionHeader.Apply(fmt.Sprintf("=== Ancestors (%d, specs that transitively cite %s) ===", len(ancestors), r.ID))))
		for _, n := range ancestors {
			p.renderColorSpecSection(&sb, n)
		}
	}

	if len(descendants) > 0 {
		sb.WriteString(fmt.Sprintf("\n%s\n", t.SectionHeader.Apply(fmt.Sprintf("=== Descendants (%d, specs that %s transitively cites) ===", len(descendants), r.ID))))
		for _, n := range descendants {
			p.renderColorSpecSection(&sb, n)
		}
	}

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
		sb.WriteString(fmt.Sprintf("\n%s\n", t.SectionHeader.Apply(fmt.Sprintf("=== Markers in closure (%d) ===", len(markers)))))
		for _, m := range markers {
			p.renderColorMarkerSection(&sb, m)
		}
	}

	if len(r.Edges) > 0 {
		sb.WriteString(fmt.Sprintf("\n%s\n", t.SectionHeader.Apply(fmt.Sprintf("=== Edges (%d total) ===", len(r.Edges)))))
		for _, e := range r.Edges {
			from := t.MarkerID.Apply(e.From)
			if isSpecID(e.From) {
				from = t.SpecID.Apply(e.From)
			}
			to := t.MarkerID.Apply(e.To)
			if isSpecID(e.To) {
				to = t.SpecID.Apply(e.To)
			}
			sb.WriteString(fmt.Sprintf("%s → %s\n", from, to))
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

func (p ColorPresenter) renderColorSpecSection(sb *strings.Builder, n ShowNode) {
	t := p.Theme
	sb.WriteString("\n" + fmt.Sprintf("%s (%s)\n", t.SpecID.Apply(n.ID), t.Filepath.Apply(n.Filepath)))
	sb.WriteString(fmt.Sprintf("Hash: %s\n", t.Hash.Apply(n.Hash)))
	if n.Deleted {
		sb.WriteString(t.StatusWarn.Apply("Status: deleted from disk") + "\n")
		return
	}
	sb.WriteString(t.SectionHeader.Apply("--- content ---") + "\n")
	sb.WriteString(p.colorizeCodeBlock(n.Content))
	sb.WriteString("\n")
}

func (p ColorPresenter) renderColorMarkerSection(sb *strings.Builder, n ShowNode) {
	t := p.Theme
	sb.WriteString("\n" + fmt.Sprintf("%s (%s:%s)\n", t.MarkerID.Apply(n.ID), t.Filepath.Apply(n.Filepath), t.LineNumber.Apply(n.Lines)))
	sb.WriteString(fmt.Sprintf("Hash: %s\n", t.Hash.Apply(n.Hash)))
	if n.Deleted {
		sb.WriteString(t.StatusWarn.Apply("Status: deleted from disk") + "\n")
		return
	}
	sb.WriteString(t.SectionHeader.Apply("--- content ---") + "\n")
	sb.WriteString(p.colorizeCodeBlock(n.Content))
	sb.WriteString("\n")
}

// --- Diff ---

func (p ColorPresenter) DiffClosure(r DiffClosureResult) string {
	t := p.Theme
	if len(r.Diffs) == 0 {
		return fmt.Sprintf("Closure %s: no diffable content.", t.Hash.Apply(r.Hash))
	}
	var sb strings.Builder
	sb.WriteString(t.SectionHeader.Apply(fmt.Sprintf("=== Closure %s ===", t.Hash.Apply(r.Hash))) + "\n\n")
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

func (p ColorPresenter) formatDiffSide(label string, side orchestrator.DiffSide, isSeed bool) string {
	t := p.Theme
	var sb strings.Builder

	var labelID string
	if label == "Spec" {
		labelID = t.SpecID.Apply(side.ID)
	} else {
		labelID = t.MarkerID.Apply(side.ID)
	}
	roleLabel := "[citer]"
	roleStyled := t.SectionHeader.Apply(roleLabel)
	if isSeed {
		roleLabel = "[SEED]"
		roleStyled = t.StatusWarn.Apply(roleLabel)
	}
	sb.WriteString(fmt.Sprintf("%s: %s %s", label, labelID, roleStyled))
	if side.Filepath != "" {
		loc := side.Filepath
		if side.Lines != "" {
			loc += ":" + side.Lines
		}
		sb.WriteString(fmt.Sprintf(" (%s)", t.Filepath.Apply(loc)))
	}
	sb.WriteString("\n")

	if side.Deleted {
		sb.WriteString(t.StatusError.Apply("Status: deleted from disk") + "\n")
	} else if !side.HasBaseline {
		sb.WriteString(t.StatusWarn.Apply(fmt.Sprintf("Status: no baseline snapshot (hash %s)", side.BaselineHash)) + "\n")
	} else if side.BaselineHash == side.CurrentHash && side.CurrentHash != "" {
		sb.WriteString(t.StatusOK.Apply("Status: in sync") + "\n")
	} else {
		sb.WriteString(t.StatusWarn.Apply(fmt.Sprintf("Baseline: %s   Current: %s", t.Hash.Apply(side.BaselineHash), t.Hash.Apply(side.CurrentHash))) + "\n")
	}

	if !side.HasBaseline {
		return strings.TrimRight(sb.String(), "\n")
	}
	if side.Baseline == side.Current {
		return strings.TrimRight(sb.String(), "\n")
	}

	sb.WriteString("\n" + t.DiffRemove.Apply("--- baseline") + "\n" + t.DiffAdd.Apply("+++ current") + "\n")
	patch := diff.UnifiedDiff(side.Baseline, side.Current)
	if patch != "" {
		sb.WriteString(p.colorizePatch(patch))
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (p ColorPresenter) DiffAll(r DiffAllResult) string {
	t := p.Theme
	if len(r.Closures) == 0 {
		return t.StatusOK.Apply("No drift detected.")
	}

	var sb strings.Builder
	for i, c := range r.Closures {
		if i > 0 {
			sb.WriteString("\n\n===\n\n")
		}
		sb.WriteString(t.SectionHeader.Apply(fmt.Sprintf("=== Closure %s ===", t.Hash.Apply(c.Hash))) + "\n\n")
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

// D! id=ccsum range-start
func (p ColorPresenter) ChangeSummary(r ChangeSummaryResult) string {
	t := p.Theme
	var sb strings.Builder
	if r.Preview {
		sb.WriteString(t.StatusWarn.Apply("Preview — no changes written") + "\n")
	}
	if r.Message != "" {
		sb.WriteString(r.Message + "\n")
	}
	sb.WriteString(fmt.Sprintf("  %s %s\n", t.SectionHeader.Apply("operation:"), r.Summary.Operation))
	for _, nc := range r.Summary.NodeChanges {
		old := shortHash(nc.OldHash)
		new := shortHash(nc.NewHash)
		switch nc.Kind {
		case "changed":
			sb.WriteString(fmt.Sprintf("  %s %s  %s → %s\n",
				t.StatusWarn.Apply(nc.Kind), t.SpecID.Apply(nc.ID), t.Hash.Apply(old), t.Hash.Apply(new)))
		case "added":
			sb.WriteString(fmt.Sprintf("  %s %s  → %s\n",
				t.StatusWarn.Apply(nc.Kind), t.SpecID.Apply(nc.ID), t.Hash.Apply(new)))
		case "removed":
			sb.WriteString(fmt.Sprintf("  %s %s  %s →\n",
				t.StatusError.Apply(nc.Kind), t.SpecID.Apply(nc.ID), t.Hash.Apply(old)))
		}
	}
	for _, ec := range r.Summary.EdgeChanges {
		sb.WriteString(fmt.Sprintf("  edge %s %s → %s\n",
			t.StatusWarn.Apply(ec.Kind), ec.From, ec.To))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// D! id=ccsum range-end

// --- Ok / Error / Text / Version ---

func (p ColorPresenter) Ok(r OkResult) string {
	return r.Message
}

func (p ColorPresenter) Error(r ErrorResult) string {
	t := p.Theme
	if r.Hint != "" {
		return t.StatusError.Apply(r.Message) + "\n" + r.Hint
	}
	return t.StatusError.Apply(r.Message)
}

func (p ColorPresenter) Text(r TextResult) string {
	return r.Text
}

func (p ColorPresenter) Version(r VersionResult) string {
	return "drift version " + r.Version
}

// D! id=ocol range-end
