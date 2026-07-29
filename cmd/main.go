package main

import (
	"chankat/internal/cli"
	"chankat/internal/storage"
	"chankat/internal/tui"
	"context"
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
	ctx := context.Background()

	stor, err := storage.Open()
	if err != nil {
		return err
	}
	defer stor.Close()
	if err := stor.Migrate(); err != nil {
		return err
	}

	if len(os.Args) == 1 {
		return tui.Run(ctx, stor)
	}

	return cli.Run(ctx, os.Args[1:], version, stor)
}
