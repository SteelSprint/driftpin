package commands

import "drift/cli/output"

// TodoCommand implements `drift todo`.
type TodoCommand struct{}

func (c TodoCommand) Run(ctx Context) (output.Result, int) {
	state, err := ctx.Orch.Todo()
	if err != nil {
		return output.ErrorResult{Command: "todo", Message: err.Error(), Exit: 2}, 2
	}
	code := 0
	if len(state.Todos) > 0 {
		code = 1
	}
	return output.TodoResult{State: state}, code
}

func (c TodoCommand) Meta() Meta {
	return Meta{
		Name:  "todo",
		Short: "Scan specs and markers, report drift",
		Usage: "Usage: drift todo\n\nScan specs and markers, report drift.\nExit codes: 0 = clean, 1 = drift exists, 2 = error.\n\nNo arguments.",
		Flags: nil,
	}
}
