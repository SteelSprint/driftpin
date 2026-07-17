package commands

import "drift/cli/output"

// ShowCommand implements `drift show <marker|spec>`.
type ShowCommand struct{}

// D! id=cshow range-start
func (c ShowCommand) Run(ctx Context) (output.Result, int) {
	if len(ctx.Args) < 2 {
		return output.ErrorResult{
			Command: "show",
			Message: "usage: drift show <marker|spec>\n\nExample: drift show cval\n         drift show core.validate",
			Exit:    1,
		}, 1
	}
	state, err := ctx.Orch.Todo()
	if err != nil {
		return output.ErrorResult{Command: "show", Message: err.Error(), Exit: 1}, 1
	}
	result, err := output.BuildShowResult(state, ctx.Dir, ctx.Args[1])
	if err != nil {
		return output.ErrorResult{Command: "show", Message: err.Error(), Exit: 1}, 1
	}
	code := 0
	if !output.EntityExists(state, ctx.Args[1]) {
		code = 1
	}
	return result, code
}

// D! id=cshow range-end
func (c ShowCommand) Meta() Meta {
	return Meta{
		Name:  "show",
		Short: "Show current content of a spec or marker",
		Usage: "Usage: drift show <marker|spec>\n\nShow current content of a spec or marker with filepath and line ranges.\nIf the ID has a dot, it is treated as a spec ID; otherwise as a marker ID.\nLinked specs/markers are also displayed.\n\nExamples:\n  drift show cval\n  drift show core.validate",
	}
}
