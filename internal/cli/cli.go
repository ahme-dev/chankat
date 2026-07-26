package cli

import (
	"context"
	"fmt"
	"strconv"

	"chansat/internal/storage"
)

func Run(ctx context.Context, args []string, version string, stor *storage.Storage) error {
	switch {
	case len(args) == 0:
		return nil
	case args[0] == "":
		return fmt.Errorf("expected command, got nothing")
	case args[0] == "version":
		fmt.Printf("chansat %s\n", version)
		return nil
	case args[0] == "rates" && len(args) == 1:
		rates, err := stor.GetRates(ctx)
		if err != nil {
			return fmt.Errorf("get rates: %w", err)
		}
		for _, rate := range rates {
			fmt.Printf("%d: %s (%d minor)\n", rate.ID, rate.Name, rate.AmountMinor)
		}
		return nil
	case args[0] == "rates":
		switch args[1] {
		case "create":
			if len(args) != 4 {
				return fmt.Errorf("expected name and amount minor, got %d", len(args)-1)
			}

			amountMinor, err := strconv.Atoi(args[3])
			if err != nil {
				return fmt.Errorf("invalid amount minor %q: %w", args[3], err)
			}

			rate := storage.Rate{
				Name:        args[2],
				AmountMinor: amountMinor,
			}
			if err := stor.CreateRate(ctx, rate); err != nil {
				return fmt.Errorf("create rate: %w", err)
			}
			fmt.Println("rate created")
			return nil
		default:
			return fmt.Errorf("unknown subcommand: %s", args[1])
		}
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}
