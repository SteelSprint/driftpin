package main

import (
	"fmt"
	"os"

	"drift/cli"
	"drift/cli/commands"
	"drift/cli/output"
)

var version = "dev"

func main() {
	commands.Version = version

	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-v") {
		args[0] = "version"
	}

	presenter := output.SelectPresenter(args, os.Stdout, os.Environ(), ".")
	out, code := cli.RunWithRender(args, ".", presenter)
	if out != "" {
		fmt.Println(out)
	}
	os.Exit(code)
}
