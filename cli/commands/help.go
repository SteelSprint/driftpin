package commands

import "drift/cli/output"

// D! id=chelpcmd range-start

// HelpCommand implements `drift help`. The help text is injected by the
// Registry from the embedded help.txt (or, in a future landing, generated from
// the Registry itself).
type HelpCommand struct {
	Text string // full help text, injected by Registry
}

func (c HelpCommand) Run(ctx Context) (output.Result, int) {
	return output.TextResult{Text: c.Text}, 0
}

// D! id=chelpcmd range-end

func (c HelpCommand) Meta() Meta {
	return Meta{
		Name:  "help",
		Short: "Show command reference with examples",
		Usage: "Usage: drift help\n\nShow the drift command reference with examples.",
	}
}
