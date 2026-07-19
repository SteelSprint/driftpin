package cli

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"

	"drift/cli/commands"
	"drift/cli/output"
	"drift/orchestrator"
	"drift/scanner"
	"drift/statestore"
)

//go:embed skill.md
var SkillContent string

//go:embed help.txt
var HelpContent string

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

// RunAuto selects the Presenter based on global flags in args (--json),
// then delegates to RunWithRender. Used by main.go.
func RunAuto(args []string, dir string) (string, int) {
	presenter := output.Presenter(output.PlainPresenter{})
	for _, a := range args {
		if a == "--json" {
			presenter = output.JSONPresenter{}
			break
		}
	}
	return RunWithRender(args, dir, presenter)
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
	args = stripGlobalFlags(args)

	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		return presenter.Text(output.TextResult{Text: HelpContent}), 0
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
			Message: fmt.Sprintf("unknown command: %s\n\n%s", args[0], HelpContent),
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

// stripGlobalFlags removes recognized global flags (--json, --no-color,
// --color=...) from args. These flags are handled before dispatch and must
// not appear in any command's recognized flag list or trigger
// unknown_flag_rejection.
func stripGlobalFlags(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--json" || a == "--no-color" {
			continue
		}
		if strings.HasPrefix(a, "--color=") {
			continue
		}
		out = append(out, a)
	}
	return out
}
