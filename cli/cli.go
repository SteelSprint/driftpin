package cli

import (
	_ "embed"
	"fmt"
	"os"
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

// D! id=cdisp
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

	// D! id=crfmt
	case "reset":
		if len(args) < 3 {
			return "usage: drift reset <marker> <module.spec>\n\nExample: drift reset validate_input core.validate_input", 1
		}
		_, err := orch.Reset(args[1], args[2])
		if err != nil {
			return err.Error(), 1
		}
		return "", 0

	// D! id=clfmt
	case "link":
		if len(args) < 3 {
			return "usage: drift link <marker> <module.spec>\n\nExample: drift link validate_input core.validate_input", 1
		}
		err := orch.Link(args[1], args[2])
		if err != nil {
			return err.Error(), 1
		}
		return fmt.Sprintf("Linked marker %q to spec %q", args[1], args[2]), 0

	// D! id=cskill
	case "skill":
		return skillContent, 0

	default:
		return fmt.Sprintf("unknown command: %s\n\n%s", args[0], helpText()), 1
	}
}

// D! id=chelp
func helpText() string {
	return helpContent
}

// D! id=cinit
func writeInitFiles(dir string) error {
	mainPath := dir + "/main.pin.xml"
	if !fileExists(mainPath) {
		if err := writeFile(mainPath, initMainPinXML); err != nil {
			return err
		}
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// D! id=cfmt
func formatTodo(state core.EvaluatedState) string {
	if len(state.Todos) == 0 {
		nSpecs := len(state.Specs)
		nMarkers := len(state.Markers)
		nLinks := len(state.Links)
		if nSpecs == 0 && nMarkers == 0 {
			return "Nothing to check: no specs or markers registered.\nCreate spec files (*.pin.xml) and place " + markerSyntax + " markers in your code,\nthen run `drift link <marker> <module.spec>` to connect them."
		}
		return fmt.Sprintf("No drift: %d specs, %d markers, %d links in sync.", nSpecs, nMarkers, nLinks)
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
		if todo.MarkerChanged && todo.SpecChanged {
			driftDescription = "Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side."
		} else if todo.MarkerChanged {
			driftDescription = "The marker has changed but not the spec term. Please check whether the changed code still complies with the spec term and make any modifications necessary."
		} else {
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
