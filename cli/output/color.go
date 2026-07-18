package output

import (
	"fmt"
	"strconv"
	"strings"

	"drift/core"
	"drift/internal/diff"
	"drift/orchestrator"
)

// D! id=ocol range-start
// ColorPresenter formats Result data using a Theme. Every method renders from
// Result data independently from PlainPresenter, using theme element lookups
// (e.g. t.MarkerID.Apply(idString)) to wrap text in ANSI codes. The guardrail
// property (stripANSI(Color.X(r)) == Plain.X(r)) holds because Style.Apply
// only wraps text — it never changes content.
type ColorPresenter struct {
	Theme Theme
}

var _ Presenter = ColorPresenter{Theme: DefaultTheme}

// colorizeCode applies syntax highlighting to a single line of code using the
// theme's code elements (CodeComment, CodeString, CodeKeyword, CodeNumber).
// Language-agnostic — uses the poor-man's tokenizer (tokenize.go).
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

// colorizePatch applies theme colors to unified diff lines:
// + lines → DiffAdd (with syntax highlighting on content), - lines → DiffRemove,
// @@ headers → DiffHunk. The ---/+++ file headers are colored by formatDiffSide.
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

	if len(state.Todos) == 0 {
		sb.WriteString(t.StatusOK.Apply(fmt.Sprintf("No changes detected. %d specs, %d markers, %d edges in sync.", len(state.Specs), len(state.Markers), len(state.Edges))))
	} else {
		// Mirror Plain's kind-bucketed summary so the guardrail property holds.
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
			sb.WriteString(t.StatusWarn.Apply(strings.Join(parts, ", ") + ".") + "\n")
		}

		sb.WriteString("\n")

		for i, todo := range state.Todos {
			sb.WriteString(p.formatTodoColor(i+1, todo))
		}
	}

	if warning := unlinkedMarkerWarning(state); warning != "" {
		sb.WriteString("\n")
		sb.WriteString(t.StatusWarn.Apply(warning))
	}

	return strings.TrimRight(sb.String(), "\n")
}

// formatTodoColor mirrors PlainPresenter.formatTodo with theme styling.
func (p ColorPresenter) formatTodoColor(n int, todo core.Todo) string {
	t := p.Theme
	var sb strings.Builder
	switch todo.Kind {
	case core.TodoEdgeDrift:
		if todo.SourceSpecID != "" {
			sb.WriteString(fmt.Sprintf("%d. %s Spec %s in %s drifted because transitively-connected spec %s in %s changed. Review whether this spec still aligns with the new upstream. Once satisfied, run %s.\n",
				n,
				t.StatusWarn.Apply("[EDGE-DRIFT]"),
				t.SpecID.Apply(fmt.Sprintf("%q", todo.From)),
				t.Filepath.Apply(fmt.Sprintf("%q", todo.FromFilepath)),
				t.SpecID.Apply(fmt.Sprintf("%q", todo.To)),
				t.Filepath.Apply(fmt.Sprintf("%q", todo.ToFilepath)),
				t.Command.Apply(fmt.Sprintf("`drift reset %s %s`", todo.From, todo.To)),
			))
		} else {
			sb.WriteString(p.formatDirectEdgeTodoColor(n, todo))
		}
	case core.TodoCascade:
		markLoc := todo.FromFilepath + ":" + strconv.Itoa(todo.FromLineNumber)
		sb.WriteString(fmt.Sprintf("%d. %s Marker %s in %s drifted because transitively-connected spec %s in %s changed. Resolve the upstream edge drift to clear this; cascade drift is derived, not independently resettable. (Upstream reset: %s.)\n",
			n,
			t.StatusWarn.Apply("[CASCADE]"),
			t.MarkerID.Apply(fmt.Sprintf("%q", todo.From)),
			t.Filepath.Apply(fmt.Sprintf("%q", markLoc)),
			t.SpecID.Apply(fmt.Sprintf("%q", todo.SourceSpecID)),
			t.Filepath.Apply(fmt.Sprintf("%q", todo.SourceFilepath)),
			t.Command.Apply(fmt.Sprintf("`drift reset <spec> %s`", todo.SourceSpecID)),
		))
	case core.TodoEdgeAdded:
		sb.WriteString(fmt.Sprintf("%d. %s New edge declared: %s → %s. Confirm the new edge is intentional. Once satisfied, run %s.\n",
			n,
			t.StatusWarn.Apply("[EDGE-ADDED]"),
			t.SpecID.Apply(fmt.Sprintf("%q", todo.From)),
			t.SpecID.Apply(fmt.Sprintf("%q", todo.To)),
			t.Command.Apply(fmt.Sprintf("`drift reset %s %s`", todo.From, todo.To)),
		))
	case core.TodoEdgeRemoved:
		sb.WriteString(fmt.Sprintf("%d. %s Edge removed: %s no longer points to %s. Confirm the removal is intentional. Once satisfied, run %s.\n",
			n,
			t.StatusWarn.Apply("[EDGE-REMOVED]"),
			t.SpecID.Apply(fmt.Sprintf("%q", todo.From)),
			t.SpecID.Apply(fmt.Sprintf("%q", todo.To)),
			t.Command.Apply(fmt.Sprintf("`drift reset %s %s`", todo.From, todo.To)),
		))
	case core.TodoBrokenEdge:
		sb.WriteString(fmt.Sprintf("%d. %s Spec %s refs %s, but no node with that ID exists. Fix the target in the spec text, or restore the missing spec. Once fixed, run %s.\n",
			n,
			t.StatusError.Apply("[BROKEN-EDGE]"),
			t.SpecID.Apply(fmt.Sprintf("%q", todo.From)),
			t.SpecID.Apply(fmt.Sprintf("%q", todo.To)),
			t.Command.Apply(fmt.Sprintf("`drift reset %s %s`", todo.From, todo.To)),
		))
	default:
		sb.WriteString(fmt.Sprintf("%d. %s Unrecognized todo kind %v.\n", n, t.StatusError.Apply("[UNKNOWN]"), todo.Kind))
	}
	return sb.String()
}

func (p ColorPresenter) formatDirectEdgeTodoColor(n int, todo core.Todo) string {
	t := p.Theme
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

	fromStyled := t.MarkerID.Apply(fmt.Sprintf("%q", todo.From))
	if isSpecIDOutput(todo.From) {
		fromStyled = t.SpecID.Apply(fmt.Sprintf("%q", todo.From))
	}
	toStyled := t.SpecID.Apply(fmt.Sprintf("%q", todo.To))
	if !isSpecIDOutput(todo.To) {
		toStyled = t.MarkerID.Apply(fmt.Sprintf("%q", todo.To))
	}

	fromLabel, toLabel := "marker", "spec term"
	if isSpecIDOutput(todo.From) {
		fromLabel = "spec"
	}
	if !isSpecIDOutput(todo.To) {
		toLabel = "marker"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d. %s Edge between %s %s in %s and %s %s in %s. %s Once you are satisfied, run %s to mark this todo item as complete.\n",
		n,
		t.StatusWarn.Apply("[TODO]"),
		fromLabel,
		fromStyled,
		t.Filepath.Apply(fmt.Sprintf("%q", fromLocation)),
		toLabel,
		toStyled,
		t.Filepath.Apply(fmt.Sprintf("%q", toLocation)),
		driftDescription,
		t.Command.Apply(fmt.Sprintf("`drift reset %s %s`", todo.From, todo.To)),
	))
	hint := fmt.Sprintf("→ Run 'drift diff %s %s' to see what changed.", todo.From, todo.To)
	sb.WriteString(fmt.Sprintf("  %s\n", t.Hint.Apply(hint)))
	return sb.String()
}

// --- List ---

func (p ColorPresenter) List(r ListResult) string {
	t := p.Theme
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
		sb.WriteString(fmt.Sprintf("  %s %s%s\n", t.MarkerID.Apply(fmt.Sprintf("%-30s", spec.ID)), t.Filepath.Apply(spec.Filepath), linkFlag))
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
		sb.WriteString("\n" + t.SectionHeader.Apply(fmt.Sprintf("Edges (%d):", len(state.Edges))) + "\n")
		for _, e := range state.Edges {
			var status string
			if driftedEdges[e.From+"\x00"+e.To] || driftedEdges[e.To+"\x00"+e.From] {
				status = t.StatusError.Apply("[DRIFTED]")
			} else {
				status = t.StatusOK.Apply("[synced]")
			}
			fromStyled := t.MarkerID.Apply(fmt.Sprintf("%-30s", e.From))
			if isSpecID(e.From) {
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
	if r.IsSpec {
		if r.Spec == nil {
			return t.StatusError.Apply(fmt.Sprintf("spec %q not found", r.ID))
		}
		return p.showSpec(r)
	}
	if r.Marker == nil {
		return t.StatusError.Apply(fmt.Sprintf("marker %q not found", r.ID))
	}
	return p.showMarker(r)
}

func (p ColorPresenter) showSpec(r ShowResult) string {
	t := p.Theme
	var sb strings.Builder

	sb.WriteString(t.SectionHeader.Apply(fmt.Sprintf("=== Spec: %s ===", t.SpecID.Apply(r.ID))) + "\n")
	sb.WriteString(fmt.Sprintf("File: %s\n", t.Filepath.Apply(r.Spec.Filepath)))
	sb.WriteString(fmt.Sprintf("Hash: %s\n\n", t.Hash.Apply(r.Spec.Hash)))
	sb.WriteString(p.colorizeCodeBlock(r.Content))
	sb.WriteString("\n")

	if len(r.OutboundRefs) > 0 || len(r.InboundRefs) > 0 {
		sb.WriteString("\n")
		if len(r.OutboundRefs) > 0 {
			sb.WriteString(fmt.Sprintf("Outbound refs (%d): %s\n", len(r.OutboundRefs), t.SpecID.Apply(strings.Join(r.OutboundRefs, ", "))))
		}
		if len(r.InboundRefs) > 0 {
			sb.WriteString(fmt.Sprintf("Inbound refs (%d): %s\n", len(r.InboundRefs), t.SpecID.Apply(strings.Join(r.InboundRefs, ", "))))
		}
	}

	for _, m := range r.LinkedMarkers {
		sb.WriteString("\n" + t.SectionHeader.Apply(fmt.Sprintf("=== Marker: %s ===", t.MarkerID.Apply(m.Marker.ID))) + "\n")
		sb.WriteString(fmt.Sprintf("File: %s\n", t.Filepath.Apply(m.Marker.Filepath)))
		sb.WriteString(fmt.Sprintf("Lines: %s\n", t.LineNumber.Apply(fmt.Sprintf("%d-%d", m.Marker.LineNumber, m.Marker.EndLineNumber))))
		sb.WriteString(fmt.Sprintf("Hash: %s\n\n", t.Hash.Apply(m.Marker.Hash)))
		sb.WriteString(p.colorizeCodeBlock(m.Content))
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

func (p ColorPresenter) showMarker(r ShowResult) string {
	t := p.Theme
	var sb strings.Builder

	for _, s := range r.LinkedSpecs {
		sb.WriteString(t.SectionHeader.Apply(fmt.Sprintf("=== Spec: %s ===", t.SpecID.Apply(s.Spec.ID))) + "\n")
		sb.WriteString(fmt.Sprintf("File: %s\n", t.Filepath.Apply(s.Spec.Filepath)))
		sb.WriteString(fmt.Sprintf("Hash: %s\n\n", t.Hash.Apply(s.Spec.Hash)))
		sb.WriteString(p.colorizeCodeBlock(s.Content))
		sb.WriteString("\n\n")
	}

	sb.WriteString(t.SectionHeader.Apply(fmt.Sprintf("=== Marker: %s ===", t.MarkerID.Apply(r.ID))) + "\n")
	sb.WriteString(fmt.Sprintf("File: %s\n", t.Filepath.Apply(r.Marker.Filepath)))
	sb.WriteString(fmt.Sprintf("Lines: %s\n", t.LineNumber.Apply(fmt.Sprintf("%d-%d", r.Marker.LineNumber, r.Marker.EndLineNumber))))
	sb.WriteString(fmt.Sprintf("Hash: %s\n\n", t.Hash.Apply(r.Marker.Hash)))
	sb.WriteString(p.colorizeCodeBlock(r.Content))

	return strings.TrimRight(sb.String(), "\n")
}

// --- Diff ---

func (p ColorPresenter) DiffEdge(r DiffEdgeResult) string {
	var sb strings.Builder
	sb.WriteString(p.formatDiffSide("Spec", r.Result.Spec))
	sb.WriteString("\n---\n")
	sb.WriteString(p.formatDiffSide("Marker", r.Result.Marker))
	return strings.TrimRight(sb.String(), "\n")
}

func (p ColorPresenter) formatDiffSide(label string, side orchestrator.DiffSide) string {
	t := p.Theme
	var sb strings.Builder

	var labelID string
	if label == "Spec" {
		labelID = t.SpecID.Apply(side.ID)
	} else {
		labelID = t.MarkerID.Apply(side.ID)
	}
	sb.WriteString(fmt.Sprintf("%s: %s", label, labelID))
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

func (p ColorPresenter) DiffExpanded(r DiffExpandedResult) string {
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

func (p ColorPresenter) DiffAll(r DiffAllResult) string {
	t := p.Theme
	if len(r.Edges) == 0 {
		return t.StatusOK.Apply("No drift detected.")
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
