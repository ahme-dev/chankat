package main

import (
	"chansat/internal/cli"
	"chansat/internal/tui"
	"fmt"
	"os"
)

func main() {
	var err error

	if len(os.Args) == 1 {
		err = tui.Run()
	} else {
		err = cli.Run(os.Args[1:])
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(0)
}
