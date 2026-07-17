package commands

import "drift/cli/output"

// ListCommand implements `drift list [--verbose]`.
type ListCommand struct{}

// D! id=clst range-start
func (c ListCommand) Run(ctx Context) (output.Result, int) {
	verbose := len(ctx.Args) >= 2 && ctx.Args[1] == "--verbose"
	state, err := ctx.Orch.Todo()
	if err != nil {
		return output.ErrorResult{Command: "list", Message: err.Error(), Exit: 1}, 1
	}
	result := output.BuildListResult(state, ctx.Dir, verbose)
	return result, 0
}

// D! id=clst range-end
func (c ListCommand) Meta() Meta {
	return Meta{
		Name:  "list",
		Short: "Show all specs, markers, links, and sync state",
		Usage: "Usage: drift list [--verbose]\n\nShow all specs, markers, links, and sync state.\n--verbose: include spec text and marker content preview.",
		Flags: []string{"--verbose"},
	}
}
