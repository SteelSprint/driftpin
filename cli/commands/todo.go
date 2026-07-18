package commands

import (
	"drift/cli/output"
	"drift/core"
)

// TodoCommand implements `drift todo`.
type TodoCommand struct{}

func (c TodoCommand) Run(ctx Context) (output.Result, int) {
	state, err := ctx.Orch.Todo()
	if err != nil {
		return output.ErrorResult{Command: "todo", Message: err.Error(), Exit: 2}, 2
	}
	code := 0
	if len(state.Todos) > 0 || hasUnlinkedMarkers(state) {
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

func (c TodoCommand) Meta() Meta {
	return Meta{
		Name:  "todo",
		Short: "Scan specs and markers, report drift",
		Usage: "Usage: drift todo\n\nScan specs and markers, report drift.\nExit codes: 0 = clean (all linked + in sync), 1 = drift or unlinked markers, 2 = error.\n\nNo arguments.",
		Flags: nil,
	}
}
