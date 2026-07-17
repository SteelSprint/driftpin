package commands

import (
	"drift/cli/output"
	"drift/orchestrator"
)

// Command is the interface every subcommand implements. The dispatcher
// (cli.RunWithRender) looks up a Command by name in cli.Registry, constructs a
// Context, calls Run, and renders the returned Result via the active
// Presenter. Commands produce data (Result); they never format strings — that
// is the Presenter's job.
type Command interface {
	Run(ctx Context) (output.Result, int)
	Meta() Meta
}

// Context carries everything a command needs at dispatch time. Args are
// positional (global flags will be stripped here in Landing 4). Dir is the
// project root. Orch is the orchestrator wired to the project's state store,
// scanner, and baseline store.
type Context struct {
	Args []string
	Dir  string
	Orch *orchestrator.Orchestrator
}

// Meta describes a command's user-facing metadata. The dispatcher derives
// help text, flag validation, and the command table from Meta — this is the
// single source of truth that replaces the former help.txt,
// subcommandHelpTexts, and recognizedFlags maps.
type Meta struct {
	Name  string   // e.g. "todo"
	Short string   // one-line description for the command table in `drift help`
	Usage string   // multi-line usage text for `drift <cmd> --help`
	Flags []string // recognized long flags beyond --help (e.g. "--verbose", "--all")
}

// Version is the build version string, set by main.go via ldflags before any
// command dispatches. VersionCommand reads this at Run time.
var Version = "dev"
