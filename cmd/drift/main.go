package main

import (
	"fmt"
	"os"

	"drift/cli"
)

var version = "dev"

func main() {
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "version" || args[0] == "--version" || args[0] == "-v") {
		fmt.Println("drift version " + version)
		return
	}
	output, code := cli.Run(args, ".")
	if output != "" {
		fmt.Println(output)
	}
	os.Exit(code)
}
