package commands

import (
	"errors"
	"fmt"

	"drift/cli/output"
	"drift/orchestrator"
)

// InitCommand implements `drift init`.
type InitCommand struct {
	InitTemplate string // starter main.drift.xml content, injected by Registry
}

func (c InitCommand) Run(ctx Context) (output.Result, int) {
	if err := ctx.Orch.Init(); err != nil {
		if errors.Is(err, orchestrator.ErrAlreadyInitialized) {
			return output.ErrorResult{
				Command: "init",
				Message: fmt.Sprintf("Project already initialized: %s/.drift/state.xml exists.\nTo reinitialize, delete .drift/ by hand (drift provides no command for this, by design).", ctx.Dir),
				Exit:    1,
			}, 1
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

func (c InitCommand) Meta() Meta {
	return Meta{
		Name:  "init",
		Short: "Initialize: create .drift/ + starter main.drift.xml",
		Usage: "Usage: drift init\n\nInitialize the .drift/ directory (state.xml + baselines/) and write a starter\nmain.drift.xml template if one does not already exist.\n\nNot idempotent: fails with exit code 1 if .drift/state.xml already exists.\nTo reinitialize, delete .drift/ by hand (drift provides no command for this).\n\nNo arguments.",
		Flags: nil,
	}
}
