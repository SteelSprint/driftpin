package commands

import "drift/cli/output"

// D! id=cvers range-start

// VersionCommand implements `drift version`. The version string is read from
// the package-level Version var, which main.go sets from ldflags before any
// command dispatches.
type VersionCommand struct{}

func (c VersionCommand) Run(ctx Context) (output.Result, int) {
	return output.VersionResult{Version: Version}, 0
}

// D! id=cvers range-end

func (c VersionCommand) Meta() Meta {
	return Meta{
		Name:  "version",
		Short: "Show version",
		Usage: "Usage: drift version\n\nShow the drift version string.",
	}
}
