package commands

import (
	"fmt"

	"drift/cli/output"
)

// UnlinkCommand implements `drift unlink <marker> <module.spec>`.
type UnlinkCommand struct{}

// D! id=cunlnk range-start
func (c UnlinkCommand) Run(ctx Context) (output.Result, int) {
	if len(ctx.Args) < 3 {
		return output.ErrorResult{
			Command: "unlink",
			Message: "usage: drift unlink <marker> <module.spec>\n\nExample: drift unlink validate_input core.validate_input",
			Exit:    1,
		}, 1
	}
	err := ctx.Orch.Unlink(ctx.Args[1], ctx.Args[2])
	if err != nil {
		return output.ErrorResult{Command: "unlink", Message: err.Error(), Exit: 1}, 1
	}
	return output.OkResult{
		Command: "unlink",
		Message: fmt.Sprintf("Unlinked marker %q from spec %q", ctx.Args[1], ctx.Args[2]),
	}, 0
}

// D! id=cunlnk range-end
func (c UnlinkCommand) Meta() Meta {
	return Meta{
		Name:  "unlink",
		Short: "Remove a link between a marker and a spec",
		Usage: "Usage: drift unlink <marker> <module.spec>\n\nRemove a link between a marker and a spec. Also clears any resolution state for that edge.\n\nExample: drift unlink validate_input core.validate_input",
	}
}
