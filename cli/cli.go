package cli

import (
	_ "embed"
	"fmt"
	"path/filepath"

	"drift/cli/commands"
	"drift/cli/output"
	"drift/orchestrator"
	"drift/scanner"
	"drift/statestore"
)

//go:embed skill.md
var skillContent string

//go:embed help.txt
var helpContent string

//go:embed init_main.drift.xml
var initMainDriftXML string

// Run is the legacy entry point that preserves the original
// (args, dir) -> (string, int) signature. It delegates to RunWithRender
// with PlainPresenter. ~50 existing test sites call Run directly; keeping
// this signature unchanged means those tests stay green untouched through
// the output-layer refactor.
func Run(args []string, dir string) (string, int) {
	return RunWithRender(args, dir, output.PlainPresenter{})
}

// RunWithRender dispatches a command via the Registry, builds a typed Result,
// and renders it via the supplied Presenter. The flow is:
//  1. Top-level help check (no args / help / --help / -h)
//  2. Per-subcommand help check (cmd --help)
//  3. Unknown-flag rejection
//  4. Registry lookup
//  5. Construct orchestrator + CommandContext
//  6. Call command.Run(ctx) → (Result, exitCode)
//  7. presenter.Render(result) → output string
//
// D! id=cdisp range-start
func RunWithRender(args []string, dir string, presenter output.Presenter) (string, int) {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		return presenter.Text(output.TextResult{Text: helpContent}), 0
	}

	if help, ok := subcommandHelp(args[0]); ok && len(args) >= 2 && (args[1] == "--help" || args[1] == "-h") {
		return presenter.Text(output.TextResult{Text: help}), 0
	}

	if msg, bad := rejectUnknownFlags(args); bad {
		return presenter.Error(output.ErrorResult{Message: msg, Exit: 1}), 1
	}

	cmd, ok := Registry[args[0]]
	if !ok {
		return presenter.Error(output.ErrorResult{
			Message: fmt.Sprintf("unknown command: %s\n\n%s", args[0], helpContent),
			Exit:    1,
		}), 1
	}

	stateStore := statestore.NewFileStateStore(dir)
	scn := scanner.NewFileScanner(dir)
	baselines := statestore.NewBaselineStore(filepath.Join(dir, ".drift", "baselines"))
	orch := orchestrator.NewOrchestrator(stateStore, scn, baselines)

	ctx := commands.Context{
		Args: args,
		Dir:  dir,
		Orch: orch,
	}

	result, code := cmd.Run(ctx)
	return output.Render(presenter, result), code
}

// D! id=cdisp range-end
