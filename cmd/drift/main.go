package main

import (
	"fmt"
	"os"

	"driftpin/cli"
)

func main() {
	output, code := cli.Run(os.Args[1:], ".")
	if output != "" {
		fmt.Println(output)
	}
	os.Exit(code)
}
