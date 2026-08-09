package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"chankat/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	reset := flag.Bool("reset", false, "replace all existing application data")
	nowValue := flag.String("now", "", "seed time in RFC3339 format")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flag.Arg(0))
	}

	now := time.Now()
	if *nowValue != "" {
		parsed, err := time.Parse(time.RFC3339, *nowValue)
		if err != nil {
			return fmt.Errorf("parse --now: %w", err)
		}
		now = parsed
	}

	stor, err := storage.Open()
	if err != nil {
		return err
	}
	defer stor.Close()
	if err := stor.Migrate(); err != nil {
		return err
	}
	if err := stor.SeedDevelopment(context.Background(), now, *reset); err != nil {
		return err
	}
	fmt.Println("seeded development data")
	return nil
}
