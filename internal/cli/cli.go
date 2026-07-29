package cli

import (
	"context"
	"fmt"
	"strconv"

	"chankat/internal/storage"
)

func Run(ctx context.Context, args []string, version string, stor *storage.Storage) error {
	switch {
	case len(args) == 0:
		return nil
	case args[0] == "":
		return fmt.Errorf("expected command, got nothing")
	case args[0] == "version":
		fmt.Printf("chankat %s\n", version)
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
			if len(args) != 5 {
				return fmt.Errorf("usage: rates create <name> <amount-minor> <currency>")
			}

			amountMinor, err := strconv.Atoi(args[3])
			if err != nil {
				return fmt.Errorf("invalid amount minor %q: %w", args[3], err)
			}

			rate := storage.Rate{
				Name:        args[2],
				AmountMinor: amountMinor,
				Currency:    args[4],
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
