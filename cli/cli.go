package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"drift/core"
	"drift/internal/diff"
	"drift/orchestrator"
	"drift/statestore"
	"drift/scanner"
)

//go:embed skill.md
var skillContent string

//go:embed help.txt
var helpContent string

//go:embed init_main.drift.xml
var initMainDriftXML string

var markerSyntax = "D" + "! id=<markerid>"

// D! id=cdisp range-start
func Run(args []string, dir string) (string, int) {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		return helpText(), 0
	}

	if help, ok := subcommandHelp(args[0]); ok && len(args) >= 2 && (args[1] == "--help" || args[1] == "-h") {
		return help, 0
	}

	if msg, bad := rejectUnknownFlags(args); bad {
		return msg, 1
	}

	stateStore := statestore.NewFileStateStore(dir)
	scanner := scanner.NewFileScanner(dir)
	baselines := statestore.NewBaselineStore(filepath.Join(dir, ".drift", "baselines"))
	orch := orchestrator.NewOrchestrator(stateStore, scanner, baselines)

	switch args[0] {
	case "init":
		if err := orch.Init(); err != nil {
			return err.Error(), 1
		}
		if err := writeInitFiles(dir); err != nil {
			return fmt.Sprintf("Initialized .drift/ but failed to write template: %s", err.Error()), 0
		}
		return "Initialized .drift/ and main.drift.xml\nEdit main.drift.xml to add your specs, then place " + markerSyntax + " markers in your code.\nRun `drift skill` for a comprehensive guide.", 0

	case "todo":
		state, err := orch.Todo()
		if err != nil {
			return err.Error(), 2
		}
		if len(state.Todos) > 0 {
			return formatTodo(state), 1
		}
		return formatTodo(state), 0

	// D! id=crfmt range-start
	case "reset":
		if len(args) < 2 {
			return "usage:\n  drift reset <marker> <module.spec>\n  drift reset <id>\n\nExample: drift reset validate_input core.validate_input", 1
		}
		if len(args) == 2 {
			err := orch.ResetOrphan(args[1])
			if err != nil {
				return err.Error(), 1
			}
			if strings.Contains(args[1], ".") {
				return fmt.Sprintf("Removed deleted spec %q from state.xml", args[1]), 0
			}
			return fmt.Sprintf("Removed deleted marker %q from state.xml", args[1]), 0
		}
		_, err := orch.Reset(args[1], args[2])
		if err != nil {
			return err.Error(), 1
		}
		return fmt.Sprintf("Resolved: %s → %s. Baseline updated.", args[1], args[2]), 0

	// D! id=crfmt range-end
	// D! id=clfmt range-start
	case "link":
		if len(args) < 3 {
			return "usage: drift link <marker> <module.spec>\n\nExample: drift link validate_input core.validate_input", 1
		}
		err := orch.Link(args[1], args[2])
		if err != nil {
			return err.Error(), 1
		}
		return fmt.Sprintf("Linked marker %q to spec %q", args[1], args[2]), 0

	// D! id=clfmt range-end
	// D! id=cunlnk range-start
	case "unlink":
		if len(args) < 3 {
			return "usage: drift unlink <marker> <module.spec>\n\nExample: drift unlink validate_input core.validate_input", 1
		}
		err := orch.Unlink(args[1], args[2])
		if err != nil {
			return err.Error(), 1
		}
		return fmt.Sprintf("Unlinked marker %q from spec %q", args[1], args[2]), 0

	// D! id=cunlnk range-end
	// D! id=clst range-start
	case "list":
		verbose := len(args) >= 2 && args[1] == "--verbose"
		state, err := orch.Todo()
		if err != nil {
			return err.Error(), 1
		}
		return formatList(state, dir, verbose), 0

	// D! id=clst range-end
	// D! id=cskill range-start
	case "skill":
		return skillContent, 0
	// D! id=cskill range-end

	// D! id=cshow range-start
	case "show":
		if len(args) < 2 {
			return "usage: drift show <marker|spec>\n\nExample: drift show cval\n         drift show core.validate", 1
		}
		state, err := orch.Todo()
		if err != nil {
			return err.Error(), 1
		}
		return formatShow(state, dir, args[1])

	// D! id=cshow range-end
	// D! id=cdiff range-start
	case "diff":
		if len(args) < 2 {
			return "usage:\n  drift diff <marker|spec>\n  drift diff <marker> <module.spec>\n\nExample: drift diff cval\n         drift diff cval core.validate", 1
		}
		if len(args) >= 3 {
			result, err := orch.Diff(args[1], args[2])
			if err != nil {
				return err.Error(), 1
			}
			return formatDiffEdge(result)
		}
		state, err := orch.Todo()
		if err != nil {
			return err.Error(), 1
		}
		return formatDiffExpanded(orch, state, args[1])

	// D! id=cdiff range-end
	default:
		return fmt.Sprintf("unknown command: %s\n\n%s", args[0], helpText()), 1
	}
}

// D! id=cdisp range-end

// D! id=chelp range-start
func helpText() string {
	return helpContent
}

// subcommandHelp returns the usage text for a known subcommand and ok=true,
// or ok=false if the name is not a recognized subcommand. Centralized so the
// help_flag uniform check at the top of Run works for every subcommand
// (including those that take no arguments: init, todo, skill).
func subcommandHelp(name string) (string, bool) {
	help, ok := subcommandHelpTexts[name]
	if !ok {
		return "", false
	}
	return help, true
}

var subcommandHelpTexts = map[string]string{
	"init": "Usage: drift init\n\nInitialize the .drift/ directory (state.xml + baselines/) and write a starter\nmain.drift.xml template if one does not already exist.\n\nNo arguments.",
	"todo": "Usage: drift todo\n\nScan specs and markers, report drift.\nExit codes: 0 = clean, 1 = drift exists, 2 = error.\n\nNo arguments.",
	"list": "Usage: drift list [--verbose]\n\nShow all specs, markers, links, and sync state.\n--verbose: include spec text and marker content preview.",
	"show": "Usage: drift show <marker|spec>\n\nShow current content of a spec or marker with filepath and line ranges.\nIf the ID has a dot, it is treated as a spec ID; otherwise as a marker ID.\nLinked specs/markers are also displayed.\n\nExamples:\n  drift show cval\n  drift show core.validate",
	"diff": "Usage:\n  drift diff <marker|spec>          Show what changed for an entity and all linked counterparts\n  drift diff <marker> <module.spec>  Show what changed for a specific edge\n\nDisplays unified diffs of spec and marker content against their baselines.\nIf the ID has a dot, it is treated as a spec ID; otherwise as a marker ID.\n\nExamples:\n  drift diff cval\n  drift diff core.validate\n  drift diff cval core.validate",
	"link": "Usage: drift link <marker> <module.spec>\n\nConnect a marker to a spec. Both must exist on disk.\n\nExample: drift link validate_input core.validate_input",
	"unlink": "Usage: drift unlink <marker> <module.spec>\n\nRemove a link between a marker and a spec. Also clears any resolution state for that edge.\n\nExample: drift unlink validate_input core.validate_input",
	"reset": "Usage:\n  drift reset <marker> <module.spec>  Resolve a drifted edge\n  drift reset <id>                Remove an orphaned (deleted, no links) spec/marker\n\nMark a drifted edge as resolved. Collapses baselines when all edges for a node are resolved.\nWhen a spec or marker has been deleted and has no links, use a single ID to remove it from state.xml.\n\nExamples:\n  drift reset validate_input core.validate_input\n  drift reset main.deleted_spec",
	"skill": "Usage: drift skill\n\nPrint the comprehensive drift guide for LLM agents: workflow, spec file format,\nmarker syntax and range hashing model, CLI command table, drift detection model,\n.drift/ directory layout, and edge cases.\n\nNo arguments.",
}

// D! id=chelp range-end

// D! id=cflag range-start
// recognizedFlags lists the recognized long/short flags for each subcommand.
// --help/-h are not listed here because they are intercepted earlier (in Run)
// before this check runs. Any arg starting with -- or - that is not in this
// allowlist for the current subcommand produces "unknown flag: <arg>".
var recognizedFlags = map[string]map[string]bool{
	"init":   {},
	"todo":   {},
	"skill":  {},
	"list":   {"--verbose": true},
	"show":   {},
	"diff":   {},
	"link":   {},
	"unlink": {},
	"reset":  {},
}

// rejectUnknownFlags scans args[1:] for any token starting with - or -- that
// is not a recognized flag for the subcommand args[0]. Returns the error
// message and true when an unknown flag is found, or "" and false otherwise.
// Top-level args[0] is the subcommand name itself; help/--help/-h at args[0]
// are handled earlier in Run and never reach this function.
func rejectUnknownFlags(args []string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	cmd := args[0]
	allowed, ok := recognizedFlags[cmd]
	if !ok {
		return "", false
	}
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			continue
		}
		if a == "--help" || a == "-h" {
			continue
		}
		if allowed[a] {
			continue
		}
		return fmt.Sprintf("unknown flag: %s", a), true
	}
	return "", false
}

// D! id=cflag range-end

// D! id=cinit range-start
func writeInitFiles(dir string) error {
	mainPath := dir + "/main.drift.xml"
	if !fileExists(mainPath) {
		if err := writeFile(mainPath, initMainDriftXML); err != nil {
			return err
		}
	}
	return nil
}

// D! id=cinit range-end

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// D! id=cfmt range-start
func formatTodo(state core.EvaluatedState) string {
	if len(state.Specs) == 0 && len(state.Markers) == 0 {
		return "Nothing to check: no specs or markers registered.\nCreate spec files (*.drift.xml) and place " + markerSyntax + " markers in your code,\nthen run `drift link <marker> <module.spec>` to connect them."
	}

	var sb strings.Builder

	if len(state.Todos) == 0 {
		sb.WriteString(fmt.Sprintf("No changes detected. %d specs, %d markers, %d links in sync.", len(state.Specs), len(state.Markers), len(state.Links)))
	} else {
		changedMarkers := make(map[string]bool)
		changedSpecs := make(map[string]bool)
		for _, todo := range state.Todos {
			if todo.MarkerChanged {
				changedMarkers[todo.MarkerID] = true
			}
			if todo.SpecChanged {
				changedSpecs[todo.SpecID] = true
			}
		}

		if n := len(changedMarkers); n > 0 {
			if n == 1 {
				sb.WriteString("1 marker has unchecked changes.\n")
			} else {
				sb.WriteString(fmt.Sprintf("%d markers have unchecked changes.\n", n))
			}
		}
		if n := len(changedSpecs); n > 0 {
			if n == 1 {
				sb.WriteString("1 spec item has unchecked changes.\n")
			} else {
				sb.WriteString(fmt.Sprintf("%d spec items have unchecked changes.\n", n))
			}
		}

		sb.WriteString("\n")

		for i, todo := range state.Todos {
			var driftDescription string
			switch {
			case todo.SpecDeleted:
				driftDescription = "The spec term has been deleted from disk. If this was intentional, run the reset command below to acknowledge the removal."
			case todo.MarkerDeleted:
				driftDescription = "The marker has been deleted from disk. If this was intentional, run the reset command below to acknowledge the removal."
			case todo.MarkerChanged && todo.SpecChanged:
				driftDescription = "Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side."
			case todo.MarkerChanged:
				driftDescription = "The marker has changed but not the spec term. Please check whether the changed code still complies with the spec term and make any modifications necessary."
			default:
				driftDescription = "The spec term has changed but not the marker. Please check whether the new version of the spec term is still reflected in the code and make any modifications necessary."
			}

			markerLocation := todo.MarkerFilepath + ":" + strconv.Itoa(todo.MarkerLineNumber)
			specLocation := todo.SpecFilepath + ":" + strconv.Itoa(todo.SpecLineNumber)

			sb.WriteString(fmt.Sprintf("%d. [TODO] Edge between marker %q in %q and spec term %q in %q. %s Once you are satisfied, run `drift reset %s %s` to mark this todo item as complete.\n",
				i+1,
				todo.MarkerID,
				markerLocation,
				todo.SpecID,
				specLocation,
				driftDescription,
				todo.MarkerID,
				todo.SpecID,
			))
			sb.WriteString(fmt.Sprintf("  → Run 'drift diff %s %s' to see what changed.\n", todo.MarkerID, todo.SpecID))
		}
	}

	if warning := unlinkedMarkerWarning(state); warning != "" {
		sb.WriteString("\n")
		sb.WriteString(warning)
	}

	return strings.TrimRight(sb.String(), "\n")
}

// unlinkedMarkerWarning returns the one-line warning summary for non-deleted
// markers that have no links, or "" when there are none. Deleted markers
// (no longer on disk) are not counted — they are reported as drift todos, not
// as unlinked markers. Suppressed entirely when there are no specs and no
// markers (the "Nothing to check" case is handled by the caller).
func unlinkedMarkerWarning(state core.EvaluatedState) string {
	linkedMarkers := make(map[string]bool, len(state.Links))
	for _, link := range state.Links {
		linkedMarkers[link.MarkerID] = true
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
func formatList(state core.EvaluatedState, dir string, verbose bool) string {
	if len(state.Specs) == 0 && len(state.Markers) == 0 {
		return "No specs or markers registered.\nRun `drift init` to get started, then create spec files (*.drift.xml) and place " + markerSyntax + " markers in your code."
	}

	driftedEdges := make(map[string]bool)
	for _, todo := range state.Todos {
		driftedEdges[todo.MarkerID+"\x00"+todo.SpecID] = true
	}

	linkedSpecs := make(map[string]bool)
	linkedMarkers := make(map[string]bool)
	for _, link := range state.Links {
		linkedSpecs[link.SpecID] = true
		linkedMarkers[link.MarkerID] = true
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
		if verbose && !spec.Deleted {
			content, err := readSpecContent(dir, spec.Filepath, spec.ID)
			if err == nil && len(content) > 0 {
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
		if verbose && !marker.Deleted {
			content, err := readMarkerContent(dir, marker.Filepath, marker.LineNumber, marker.EndLineNumber)
			if err == nil && len(content) > 0 {
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

	if len(state.Links) > 0 {
		sb.WriteString(fmt.Sprintf("\nLinks (%d):\n", len(state.Links)))
		for _, link := range state.Links {
			status := "[synced]"
			if driftedEdges[link.MarkerID+"\x00"+link.SpecID] {
				status = "[DRIFTED]"
			}
			sb.WriteString(fmt.Sprintf("  %-15s → %-30s %s\n", link.MarkerID, link.SpecID, status))
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// D! id=ofmtl range-end

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

func formatShow(state core.EvaluatedState, dir, id string) (string, int) {
	isSpec := strings.Contains(id, ".")

	if isSpec {
		return formatShowSpec(state, dir, id)
	}
	return formatShowMarker(state, dir, id)
}

func formatShowSpec(state core.EvaluatedState, dir, specID string) (string, int) {
	var spec *core.Spec
	for i := range state.Specs {
		if state.Specs[i].ID == specID {
			spec = &state.Specs[i]
			break
		}
	}
	if spec == nil {
		return fmt.Sprintf("spec %q not found", specID), 1
	}

	var sb strings.Builder

	content, err := readSpecContent(dir, spec.Filepath, spec.ID)
	if err != nil {
		return fmt.Sprintf("error reading spec content: %s", err), 1
	}

	sb.WriteString(fmt.Sprintf("=== Spec: %s ===\n", spec.ID))
	sb.WriteString(fmt.Sprintf("File: %s\n", spec.Filepath))
	sb.WriteString(fmt.Sprintf("Hash: %s\n\n", spec.Hash))
	sb.WriteString(content)
	sb.WriteString("\n")

	for _, link := range state.Links {
		if link.SpecID != specID {
			continue
		}
		for i := range state.Markers {
			if state.Markers[i].ID == link.MarkerID {
				m := &state.Markers[i]
				markerContent, err := readMarkerContent(dir, m.Filepath, m.LineNumber, m.EndLineNumber)
				if err != nil {
					continue
				}
				sb.WriteString(fmt.Sprintf("\n=== Marker: %s ===\n", m.ID))
				sb.WriteString(fmt.Sprintf("File: %s\n", m.Filepath))
				sb.WriteString(fmt.Sprintf("Lines: %d-%d\n", m.LineNumber, m.EndLineNumber))
				sb.WriteString(fmt.Sprintf("Hash: %s\n\n", m.Hash))
				sb.WriteString(markerContent)
				sb.WriteString("\n")
				break
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n"), 0
}

func formatShowMarker(state core.EvaluatedState, dir, markerID string) (string, int) {
	var marker *core.Marker
	for i := range state.Markers {
		if state.Markers[i].ID == markerID {
			marker = &state.Markers[i]
			break
		}
	}
	if marker == nil {
		return fmt.Sprintf("marker %q not found", markerID), 1
	}

	var sb strings.Builder

	for _, link := range state.Links {
		if link.MarkerID != markerID {
			continue
		}
		for i := range state.Specs {
			if state.Specs[i].ID == link.SpecID {
				s := &state.Specs[i]
				content, err := readSpecContent(dir, s.Filepath, s.ID)
				if err != nil {
					continue
				}
				sb.WriteString(fmt.Sprintf("=== Spec: %s ===\n", s.ID))
				sb.WriteString(fmt.Sprintf("File: %s\n", s.Filepath))
				sb.WriteString(fmt.Sprintf("Hash: %s\n\n", s.Hash))
				sb.WriteString(content)
				sb.WriteString("\n\n")
				break
			}
		}
	}

	markerContent, err := readMarkerContent(dir, marker.Filepath, marker.LineNumber, marker.EndLineNumber)
	if err != nil {
		return fmt.Sprintf("error reading marker content: %s", err), 1
	}

	sb.WriteString(fmt.Sprintf("=== Marker: %s ===\n", marker.ID))
	sb.WriteString(fmt.Sprintf("File: %s\n", marker.Filepath))
	sb.WriteString(fmt.Sprintf("Lines: %d-%d\n", marker.LineNumber, marker.EndLineNumber))
	sb.WriteString(fmt.Sprintf("Hash: %s\n\n", marker.Hash))
	sb.WriteString(markerContent)

	return strings.TrimRight(sb.String(), "\n"), 0
}

func readSpecContent(dir, filepath, specID string) (string, error) {
	return scanner.ReadSpecContent(resolvePath(dir, filepath), specID)
}

func readMarkerContent(dir, filepath string, startLine, endLine int) (string, error) {
	return scanner.ReadMarkerContent(resolvePath(dir, filepath), startLine, endLine)
}

func resolvePath(dir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}

// D! id=cdifffmt range-start
func formatDiffEdge(result orchestrator.DiffResult) (string, int) {
	var sb strings.Builder
	sb.WriteString(formatDiffSide("Spec", result.Spec))
	sb.WriteString("\n---\n")
	sb.WriteString(formatDiffSide("Marker", result.Marker))
	return strings.TrimRight(sb.String(), "\n"), 0
}

func formatDiffSide(label string, side orchestrator.DiffSide) string {
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

func formatDiffExpanded(orch *orchestrator.Orchestrator, state core.EvaluatedState, id string) (string, int) {
	isSpec := strings.Contains(id, ".")

	var edges []struct{ marker, spec string }
	if isSpec {
		for _, link := range state.Links {
			if link.SpecID == id {
				edges = append(edges, struct{ marker, spec string }{link.MarkerID, link.SpecID})
			}
		}
	} else {
		for _, link := range state.Links {
			if link.MarkerID == id {
				edges = append(edges, struct{ marker, spec string }{link.MarkerID, link.SpecID})
			}
		}
	}

	if len(edges) == 0 {
		return fmt.Sprintf("no linked edges found for %q", id), 1
	}

	var sb strings.Builder
	for i, edge := range edges {
		if i > 0 {
			sb.WriteString("\n\n===\n\n")
		}
		result, err := orch.Diff(edge.marker, edge.spec)
		if err != nil {
			return err.Error(), 1
		}
		out, code := formatDiffEdge(result)
		if code != 0 {
			return out, code
		}
		sb.WriteString(out)
	}
	return strings.TrimRight(sb.String(), "\n"), 0
}

// D! id=cdifffmt range-end
