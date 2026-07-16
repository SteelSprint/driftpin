package cli

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"driftpin/core"
	"driftpin/orchestrator"
	"driftpin/pinstore"
	"driftpin/scanner"
)

//go:embed skill.md
var skillContent string

//go:embed help.txt
var helpContent string

//go:embed init_main.pin.xml
var initMainPinXML string

var markerSyntax = "D" + "! id=<markerid>"

// D! id=cdisp range-start
func Run(args []string, dir string) (string, int) {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		return helpText(), 0
	}

	pin := pinstore.NewFilePinStore(dir)
	scanner := scanner.NewFileScanner(dir)
	orch := orchestrator.NewOrchestrator(pin, scanner)

	switch args[0] {
	case "init":
		if err := orch.Init(); err != nil {
			return err.Error(), 1
		}
		if err := writeInitFiles(dir); err != nil {
			return fmt.Sprintf("Initialized drift.pin but failed to write template: %s", err.Error()), 0
		}
		return "Initialized drift.pin and main.pin.xml\nEdit main.pin.xml to add your specs, then place " + markerSyntax + " markers in your code.\nRun `drift skill` for a comprehensive guide.", 0

	case "todo":
		state, err := orch.Todo()
		if err != nil {
			return err.Error(), 1
		}
		return formatTodo(state), 0

	// D! id=crfmt range-start
	case "reset":
		if len(args) >= 2 && (args[1] == "--help" || args[1] == "-h") {
			return "Usage:\n  drift reset <marker> <module.spec>  Resolve a drifted edge\n  drift reset <id>                Remove an orphaned (deleted, no links) spec/marker\n\nMark a drifted edge as resolved. Collapses baselines when all edges for a node are resolved.\nWhen a spec or marker has been deleted and has no links, use a single ID to remove it from drift.pin.\n\nExamples:\n  drift reset validate_input core.validate_input\n  drift reset main.deleted_spec", 0
		}
		if len(args) < 2 {
			return "usage:\n  drift reset <marker> <module.spec>\n  drift reset <id>\n\nExample: drift reset validate_input core.validate_input", 1
		}
		if len(args) == 2 {
			err := orch.ResetOrphan(args[1])
			if err != nil {
				return err.Error(), 1
			}
			if strings.Contains(args[1], ".") {
				return fmt.Sprintf("Removed deleted spec %q from drift.pin", args[1]), 0
			}
			return fmt.Sprintf("Removed deleted marker %q from drift.pin", args[1]), 0
		}
		_, err := orch.Reset(args[1], args[2])
		if err != nil {
			return err.Error(), 1
		}
		return "", 0

	// D! id=crfmt range-end
	// D! id=clfmt range-start
	case "link":
		if len(args) >= 2 && (args[1] == "--help" || args[1] == "-h") {
			return "Usage: drift link <marker> <module.spec>\n\nConnect a marker to a spec. Both must exist on disk.\n\nExample: drift link validate_input core.validate_input", 0
		}
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
		if len(args) >= 2 && (args[1] == "--help" || args[1] == "-h") {
			return "Usage: drift unlink <marker> <module.spec>\n\nRemove a link between a marker and a spec. Also clears any resolution state for that edge.\n\nExample: drift unlink validate_input core.validate_input", 0
		}
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
		if len(args) >= 2 && (args[1] == "--help" || args[1] == "-h") {
			return "Usage: drift list [--verbose]\n\nShow all specs, markers, links, and sync state.\n--verbose: include spec text and marker content preview.", 0
		}
		verbose := len(args) >= 2 && args[1] == "--verbose"
		state, err := orch.Todo()
		if err != nil {
			return err.Error(), 1
		}
		return formatList(state, verbose), 0

	// D! id=clst range-end
	// D! id=cskill range-start
	case "skill":
		return skillContent, 0

	// D! id=cshow range-start
	case "show":
		if len(args) >= 2 && (args[1] == "--help" || args[1] == "-h") {
			return "Usage: drift show <marker|spec>\n\nShow current content of a spec or marker with filepath and line ranges.\nIf the ID has a dot, it is treated as a spec ID; otherwise as a marker ID.\nLinked specs/markers are also displayed.\n\nExamples:\n  drift show cval\n  drift show core.validate", 0
		}
		if len(args) < 2 {
			return "usage: drift show <marker|spec>\n\nExample: drift show cval\n         drift show core.validate", 1
		}
		state, err := orch.Todo()
		if err != nil {
			return err.Error(), 1
		}
		return formatShow(state, args[1])

	// D! id=cshow range-end
	default:
		return fmt.Sprintf("unknown command: %s\n\n%s", args[0], helpText()), 1
	}
}

// D! id=cdisp range-end

// D! id=chelp range-start
func helpText() string {
	return helpContent
}

// D! id=chelp range-end

// D! id=cinit range-start
func writeInitFiles(dir string) error {
	mainPath := dir + "/main.pin.xml"
	if !fileExists(mainPath) {
		// D! id=cskill range-end
		if err := writeFile(mainPath, initMainPinXML); err != nil {
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
	if len(state.Todos) == 0 {
		nSpecs := len(state.Specs)
		nMarkers := len(state.Markers)
		nLinks := len(state.Links)
		if nSpecs == 0 && nMarkers == 0 {
			return "Nothing to check: no specs or markers registered.\nCreate spec files (*.pin.xml) and place " + markerSyntax + " markers in your code,\nthen run `drift link <marker> <module.spec>` to connect them."
		}
		return fmt.Sprintf("No changes detected. %d specs, %d markers, %d links in sync.", nSpecs, nMarkers, nLinks)
	}

	var sb strings.Builder

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
	}

	return strings.TrimRight(sb.String(), "\n")
}

// D! id=cfmt range-end

// D! id=ofmtl range-start
func formatList(state core.EvaluatedState, verbose bool) string {
	if len(state.Specs) == 0 && len(state.Markers) == 0 {
		return "No specs or markers registered.\nRun `drift init` to get started, then create spec files (*.pin.xml) and place " + markerSyntax + " markers in your code."
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
			content, err := readSpecContent(spec.Filepath, spec.ID)
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
			content, err := readMarkerContent(marker.Filepath, marker.LineNumber, marker.EndLineNumber)
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

var showMarkerPattern = regexp.MustCompile(`D!\s+id=\S+`)

func formatShow(state core.EvaluatedState, id string) (string, int) {
	isSpec := strings.Contains(id, ".")

	if isSpec {
		return formatShowSpec(state, id)
	}
	return formatShowMarker(state, id)
}

func formatShowSpec(state core.EvaluatedState, specID string) (string, int) {
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

	content, err := readSpecContent(spec.Filepath, spec.ID)
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
				markerContent, err := readMarkerContent(m.Filepath, m.LineNumber, m.EndLineNumber)
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

func formatShowMarker(state core.EvaluatedState, markerID string) (string, int) {
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
				content, err := readSpecContent(s.Filepath, s.ID)
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

	markerContent, err := readMarkerContent(marker.Filepath, marker.LineNumber, marker.EndLineNumber)
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

func readSpecContent(filepath, specID string) (string, error) {
	parts := strings.SplitN(specID, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid spec ID %q", specID)
	}
	localID := parts[1]

	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}

	type specElem struct {
		ID      string `xml:"id,attr"`
		Content string `xml:",innerxml"`
	}
	type pinFile struct {
		XMLName xml.Name
		Specs   []specElem `xml:"spec"`
	}

	var file pinFile
	if err := xml.Unmarshal(data, &file); err != nil {
		return "", err
	}

	for _, s := range file.Specs {
		if s.ID == localID {
			return strings.TrimSpace(s.Content), nil
		}
	}
	return "", fmt.Errorf("spec %q not found in %s", specID, filepath)
}

func readMarkerContent(filepath string, startLine, endLine int) (string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	markerLines := make(map[int]bool)
	for i, line := range lines {
		if showMarkerPattern.MatchString(line) {
			markerLines[i] = true
		}
	}

	var contentLines []string
	for i := startLine; i < endLine-1; i++ {
		if i >= len(lines) {
			break
		}
		line := lines[i]
		if markerLines[i] {
			idx := strings.Index(line, "D!")
			if idx >= 0 {
				line = line[:idx]
			}
		}
		contentLines = append(contentLines, line)
	}

	content := strings.Join(contentLines, "\n")
	if len(contentLines) > 0 {
		content += "\n"
	}
	return content, nil
}
