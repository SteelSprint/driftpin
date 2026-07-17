package commands

import (
	"fmt"
	"strings"

	"drift/cli/output"
)

// ResetCommand implements `drift reset` in its two forms:
//   - drift reset <marker> <module.spec>  (resolve a drifted edge)
//   - drift reset <id>                    (remove orphaned deleted entry)
type ResetCommand struct{}

// D! id=crfmt range-start
func (c ResetCommand) Run(ctx Context) (output.Result, int) {
	args := ctx.Args
	if len(args) < 2 {
		return output.ErrorResult{
			Command: "reset",
			Message: "usage:\n  drift reset <marker> <module.spec>\n  drift reset <id>\n\nExample: drift reset validate_input core.validate_input",
			Exit:    1,
		}, 1
	}
	if len(args) == 2 {
		err := ctx.Orch.ResetOrphan(args[1])
		if err != nil {
			return output.ErrorResult{Command: "reset", Message: err.Error(), Exit: 1}, 1
		}
		if strings.Contains(args[1], ".") {
			return output.OkResult{
				Command: "reset",
				Message: fmt.Sprintf("Removed deleted spec %q from state.xml", args[1]),
			}, 0
		}
		return output.OkResult{
			Command: "reset",
			Message: fmt.Sprintf("Removed deleted marker %q from state.xml", args[1]),
		}, 0
	}
	// D! id=cnobulk range-start
	_, err := ctx.Orch.Reset(args[1], args[2])
	if err != nil {
		return output.ErrorResult{Command: "reset", Message: err.Error(), Exit: 1}, 1
	}
	return output.OkResult{
		Command: "reset",
		Message: fmt.Sprintf("Resolved: %s → %s. Baseline updated.", args[1], args[2]),
	}, 0
	// D! id=cnobulk range-end
}

	// D! id=crfmt range-end
func (c ResetCommand) Meta() Meta {
	return Meta{
		Name:  "reset",
		Short: "Mark a drifted edge as resolved",
		Usage: "Usage:\n  drift reset <marker> <module.spec>  Resolve a drifted edge\n  drift reset <id>                Remove an orphaned (deleted, no links) spec/marker\n\nMark a drifted edge as resolved. Collapses baselines when all edges for a node are resolved.\nWhen a spec or marker has been deleted and has no links, use a single ID to remove it from state.xml.\n\nExamples:\n  drift reset validate_input core.validate_input\n  drift reset main.deleted_spec",
	}
}
