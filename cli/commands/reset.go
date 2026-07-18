package commands

import (
	"fmt"
	"strings"

	"drift/cli/output"
)

// ResetCommand implements `drift reset` in its three forms:
//   - drift reset <marker> <module.spec>           (resolve a drifted edge)
//   - drift reset <module.reviewed> <module.source> (resolve a drifted spec-spec edge)
//   - drift reset <id>                              (remove orphaned deleted entry)
type ResetCommand struct{}

// D! id=crfmt range-start
func (c ResetCommand) Run(ctx Context) (output.Result, int) {
	args := ctx.Args
	if len(args) < 2 {
		return output.ErrorResult{
			Command: "reset",
			Message: "usage:\n  drift reset <marker> <module.spec>            Resolve a drifted link edge\n  drift reset <module.reviewed> <module.source> Resolve a drifted spec-spec edge\n  drift reset <id>                                Remove orphaned (deleted) spec/marker\n\nExample: drift reset validate_input core.validate_input\n         drift reset output.color_mode output_impl.color_presenter",
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
	// Two args: dispatch on dots. Both specs (dots) → spec-spec edge reset.
	// Marker + spec → link-edge reset. Spec + marker → error.
	firstHasDot := strings.Contains(args[1], ".")
	secondHasDot := strings.Contains(args[2], ".")
	if firstHasDot && !secondHasDot {
		return output.ErrorResult{
			Command: "reset",
			Message: "first arg is a spec (has dot) but second arg is a marker (no dot); did you mean `drift reset <marker> <spec>`?",
			Exit:    1,
		}, 1
	}
	// D! id=cnobulk range-start
	_, err := ctx.Orch.Reset(args[1], args[2])
	if err != nil {
		return output.ErrorResult{Command: "reset", Message: err.Error(), Exit: 1}, 1
	}
	if firstHasDot && secondHasDot {
		return output.OkResult{
			Command: "reset",
			Message: fmt.Sprintf("Resolved edge: %s reviewed against %s.", args[1], args[2]),
		}, 0
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
		Usage: "Usage:\n  drift reset <marker> <module.spec>                  Resolve a drifted link edge\n  drift reset <module.reviewed> <module.source>       Resolve a drifted spec-spec edge\n  drift reset <id>                                     Remove an orphaned (deleted, no edges) spec/marker\n\nMark a drifted edge as resolved. Collapses baselines when all edges for a node are resolved.\nWhen a spec or marker has been deleted and has no edges, use a single ID to remove it from state.xml.\n\nForms are distinguished by dots: marker shortcodes have no dots; spec IDs have exactly one dot.\nA spec-spec reset has dots in BOTH args.\n\nExamples:\n  drift reset validate_input core.validate_input\n  drift reset output.color_mode output_impl.color_presenter\n  drift reset main.deleted_spec",
	}
}
