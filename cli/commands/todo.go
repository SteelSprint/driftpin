package commands

import (
	"strings"

	"drift/cli/output"
	"drift/core"
)

// TodoCommand implements `drift todo`.
type TodoCommand struct{}

func (c TodoCommand) Run(ctx Context) (output.Result, int) {
	state, err := ctx.Orch.Todo(ctx.Sess)
	if err != nil {
		return output.ErrorResult{Command: "todo", Message: err.Error(), Exit: 2}, 2
	}
	code := 0
	if len(state.Closures) > 0 || hasUnlinkedMarkers(state) {
		code = 1
	}
	return output.TodoResult{State: state}, code
}

func hasUnlinkedMarkers(state core.EvaluatedState) bool {
	linkedMarkers := make(map[string]bool)
	for _, e := range state.Edges {
		// Link-style edge: From is marker (no dot).
		if isSpecIDLocal(e.From) {
			continue
		}
		linkedMarkers[e.From] = true
	}
	for _, m := range state.Markers {
		if !m.Deleted && !linkedMarkers[m.ID] {
			return true
		}
	}
	return false
}

func isSpecIDLocal(id string) bool {
	first := strings.Index(id, ".")
	if first < 0 {
		return false
	}
	return strings.Index(id[first+1:], ".") < 0
}

func (c TodoCommand) Meta() Meta {
	return Meta{
		Name:  "todo",
		Short: "Scan specs and markers, report drift closures",
		Usage: "Usage: drift todo\n\nScan specs and markers, report drift closures.\nExit codes: 0 = clean (all linked + in sync), 1 = drift or unlinked markers, 2 = error.\n\nNo arguments.",
		Flags: nil,
	}
}
