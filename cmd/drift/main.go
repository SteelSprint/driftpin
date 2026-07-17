package main

import (
	"fmt"
	"os"

	"drift/cli"
	"drift/cli/commands"
)

var version = "dev"

func main() {
	commands.Version = version

	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-v") {
		args[0] = "version"
	}

	output, code := cli.Run(args, ".")
	if output != "" {
		fmt.Println(output)
	}
	os.Exit(code)
}
