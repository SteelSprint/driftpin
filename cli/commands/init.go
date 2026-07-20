package commands

import (
	"errors"
	"fmt"

	"drift/cli/output"
	"drift/orchestrator"
)

// D! id=cinitcmd range-start

// InitCommand implements `drift init`.
type InitCommand struct {
	InitTemplate string // starter main.drift.xml content, injected by Registry
}

func (c InitCommand) Run(ctx Context) (output.Result, int) {
	if err := ctx.Orch.Init(ctx.Sess); err != nil {
		if errors.Is(err, orchestrator.ErrAlreadyInitialized) {
			return output.OkResult{
				Command: "init",
				Message: "Already initialized. Run `drift todo` to check state.",
			}, 0
		}
		return output.ErrorResult{Command: "init", Message: err.Error(), Exit: 1}, 1
	}
	if err := writeInitFile(ctx.Dir, c.InitTemplate); err != nil {
		return output.OkResult{
			Command: "init",
			Message: fmt.Sprintf("Initialized .drift/ but failed to write template: %s", err.Error()),
		}, 0
	}
	return output.OkResult{
		Command: "init",
		Message: "Initialized .drift/ and main.drift.xml\nEdit main.drift.xml to add your specs, then place " + markerSyntax + " markers in your code.\nRun `drift skill` for a comprehensive guide.",
	}, 0
}

// D! id=cinitcmd range-end

func (c InitCommand) Meta() Meta {
	return Meta{
		Name:  "init",
		Short: "Initialize: create .drift/ + starter main.drift.xml",
		Usage: "Usage: drift init\n\nInitialize the .drift/ directory (state.xml) and write a starter\nmain.drift.xml template if one does not already exist. baselines.bin is\ncreated lazily on the first baseline write (e.g. drift link).\n\nIdempotent: if .drift/state.xml already exists, prints \"Already initialized\"\nwith exit code 0 and does not modify any files. To forcibly reinitialize,\ndelete .drift/ by hand (drift provides no command for this, by design).\n\nNo arguments.",
		Flags: nil,
	}
}
