package cli

import (
	"fmt"
	"strings"

	"drift/cli/commands"
)

// Registry is the single source of truth for command metadata. Every command
// registers here with its name, implementation, and Meta. Help text, usage
// strings, and flag validation are all derived from this map — replacing the
// former help.txt + subcommandHelpTexts + recognizedFlags trio.
//
// The Registry is constructed at package load time. Commands that need
// embedded content (help text, skill guide, init template) receive it via
// struct fields set here from the package-level embeds in cli.go.
var Registry = map[string]commands.Command{
	"init":    commands.InitCommand{InitTemplate: initMainDriftXML},
	"todo":    commands.TodoCommand{},
	"list":    commands.ListCommand{},
	"show":    commands.ShowCommand{},
	"diff":    commands.DiffCommand{},
	"link":    commands.LinkCommand{},
	"unlink":  commands.UnlinkCommand{},
	"reset":   commands.ResetCommand{},
	"help":    commands.HelpCommand{Text: helpContent},
	"skill":   commands.SkillCommand{Text: skillContent},
	"version": commands.VersionCommand{},
}

// D! id=chelp range-start
// subcommandHelp returns the usage text for a known subcommand. Derived from
// Registry — no separate map to maintain.
func subcommandHelp(name string) (string, bool) {
	cmd, ok := Registry[name]
	if !ok {
		return "", false
	}
	return cmd.Meta().Usage, true
}

// D! id=chelp range-end

// D! id=cflag range-start
// recognizedFlagsFor returns the set of recognized long flags for a subcommand
// (beyond the universal --help/-h). Derived from Registry — no separate map
// to maintain.
func recognizedFlagsFor(cmd string) map[string]bool {
	c, ok := Registry[cmd]
	if !ok {
		return nil
	}
	flags := map[string]bool{}
	for _, f := range c.Meta().Flags {
		flags[f] = true
	}
	return flags
}

// rejectUnknownFlags scans args[1:] for any token starting with - or -- that
// is not a recognized flag for the subcommand args[0]. Returns the error
// message and true when an unknown flag is found, or "" and false otherwise.
func rejectUnknownFlags(args []string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	cmd := args[0]
	allowed := recognizedFlagsFor(cmd)
	if allowed == nil {
		return "", false
	}
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			continue
		}
		if a == "--help" || a == "-h" {
			continue
		}
		if allowed[a] {
			continue
		}
		return fmt.Sprintf("unknown flag: %s", a), true
	}
	return "", false
}

// D! id=cflag range-end
