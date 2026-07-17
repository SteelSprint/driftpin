package commands

import (
	"fmt"
	"strings"

	"drift/cli/output"
	"drift/core"
	"drift/orchestrator"
)

// DiffCommand implements `drift diff` in its three forms:
//   - drift diff <marker|spec>            (auto-expand to linked edges)
//   - drift diff <marker> <module.spec>   (specific edge)
//   - drift diff --all                    (all drifted edges)
type DiffCommand struct{}

// D! id=cdiff range-start
func (c DiffCommand) Run(ctx Context) (output.Result, int) {
	args := ctx.Args
	if len(args) < 2 {
		return output.ErrorResult{
			Command: "diff",
			Message: "usage:\n  drift diff <marker|spec>\n  drift diff <marker> <module.spec>\n  drift diff --all\n\nExample: drift diff cval\n         drift diff cval core.validate",
			Exit:    1,
		}, 1
	}
	if len(args) >= 2 && args[1] == "--all" {
		state, err := ctx.Orch.Todo()
		if err != nil {
			return output.ErrorResult{Command: "diff", Message: err.Error(), Exit: 1}, 1
		}
		edges := make([]orchestrator.DiffResult, 0, len(state.Todos))
		for _, todo := range state.Todos {
			result, err := ctx.Orch.Diff(todo.MarkerID, todo.SpecID)
			if err != nil {
				return output.ErrorResult{Command: "diff", Message: err.Error(), Exit: 1}, 1
			}
			edges = append(edges, result)
		}
		return output.DiffAllResult{State: state, Edges: edges}, 0
	}
	if len(args) >= 3 {
		result, err := ctx.Orch.Diff(args[1], args[2])
		if err != nil {
			return output.ErrorResult{Command: "diff", Message: err.Error(), Exit: 1}, 1
		}
		return output.DiffEdgeResult{Result: result}, 0
	}
	state, err := ctx.Orch.Todo()
	if err != nil {
		return output.ErrorResult{Command: "diff", Message: err.Error(), Exit: 1}, 1
	}
	edges, err := expandDiffEdges(ctx.Orch, state, args[1])
	if err != nil {
		return output.ErrorResult{Command: "diff", Message: err.Error(), Exit: 1}, 1
	}
	return output.DiffExpandedResult{ID: args[1], Edges: edges}, 0
}

// D! id=cdiff range-end
func (c DiffCommand) Meta() Meta {
	return Meta{
		Name:  "diff",
		Short: "Show what changed (unified diff vs baseline)",
		Usage: "Usage:\n  drift diff <marker|spec>          Show what changed for an entity and all linked counterparts\n  drift diff <marker> <module.spec>  Show what changed for a specific edge\n  drift diff --all                   Show diffs for ALL drifted edges at once\n\nDisplays unified diffs of spec and marker content against their baselines.\nIf the ID has a dot, it is treated as a spec ID; otherwise as a marker ID.\n\nExamples:\n  drift diff cval\n  drift diff core.validate\n  drift diff cval core.validate\n  drift diff --all",
		Flags: []string{"--all"},
	}
}

// expandDiffEdges resolves all linked edges for a single ID (marker or spec).
func expandDiffEdges(orch *orchestrator.Orchestrator, state core.EvaluatedState, id string) ([]orchestrator.DiffResult, error) {
	isSpec := strings.Contains(id, ".")
	var pairs []struct{ marker, spec string }
	if isSpec {
		for _, link := range state.Links {
			if link.SpecID == id {
				pairs = append(pairs, struct{ marker, spec string }{link.MarkerID, link.SpecID})
			}
		}
	} else {
		for _, link := range state.Links {
			if link.MarkerID == id {
				pairs = append(pairs, struct{ marker, spec string }{link.MarkerID, link.SpecID})
			}
		}
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("no linked edges found for %q", id)
	}
	edges := make([]orchestrator.DiffResult, 0, len(pairs))
	for _, p := range pairs {
		result, err := orch.Diff(p.marker, p.spec)
		if err != nil {
			return nil, err
		}
		edges = append(edges, result)
	}
	return edges, nil
}
