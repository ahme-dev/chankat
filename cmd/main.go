package main

import (
	"chansat/internal/cli"
	"chansat/internal/storage"
	"chansat/internal/tui"
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	db, err := storage.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := storage.Migrate(db); err != nil {
		return err
	}

	if len(os.Args) == 1 {
		return tui.Run()
	}

	return cli.Run(os.Args[1:], version)
}
