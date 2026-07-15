package cli

import (
	"fmt"
	"strconv"
	"strings"

	"driftpin/core"
	"driftpin/orchestrator"
	"driftpin/pinstore"
	"driftpin/scanner"
)

// #F cdisp
func Run(args []string, dir string) (string, int) {
	if len(args) == 0 {
		return "usage: drift <init|todo|reset <marker>:<spec>|link <marker>:<spec>>", 1
	}

	pin := pinstore.NewFilePinStore(dir)
	scanner := scanner.NewFileScanner(dir)
	orch := orchestrator.NewOrchestrator(pin, scanner)

	switch args[0] {
	case "init":
		if err := orch.Init(); err != nil {
			return err.Error(), 1
		}
		return "Initialized drift.pin", 0

	case "todo":
		state, err := orch.Todo()
		if err != nil {
			return err.Error(), 1
		}
		return formatTodo(state), 0

	// #F crfmt
	case "reset":
		if len(args) < 2 {
			return "usage: drift reset <marker>:<spec>", 1
		}
		parts := strings.SplitN(args[1], ":", 2)
		if len(parts) != 2 {
			return "invalid format, expected <marker>:<spec>", 1
		}
		_, err := orch.Reset(parts[0], parts[1])
		if err != nil {
			return err.Error(), 1
		}
		return "", 0

	// #F clfmt
	case "link":
		if len(args) < 2 {
			return "usage: drift link <marker>:<spec>", 1
		}
		parts := strings.SplitN(args[1], ":", 2)
		if len(parts) != 2 {
			return "invalid format, expected <marker>:<spec>", 1
		}
		err := orch.Link(parts[0], parts[1])
		if err != nil {
			return err.Error(), 1
		}
		return fmt.Sprintf("Linked marker %q to spec %q", parts[0], parts[1]), 0

	default:
		return fmt.Sprintf("unknown command: %s", args[0]), 1
	}
}

// #F cfmt
func formatTodo(state core.EvaluatedState) string {
	if len(state.Todos) == 0 {
		return "No changes detected."
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

		sb.WriteString(fmt.Sprintf("%d. [TODO] Edge between marker %q in %q and spec term %q in %q. %s Once you are satisfied, run `drift reset %s:%s` to mark this todo item as complete.\n",
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
