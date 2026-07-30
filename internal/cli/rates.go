package cli

import (
	"fmt"

	"chankat/internal/storage"
)

func (r runner) runRates(args []string) error {
	if len(args) == 0 {
		return r.listRates(nil)
	}
	switch args[0] {
	case "list":
		return r.listRates(args[1:])
	case "get":
		return r.getRate(args[1:])
	case "create":
		return r.createRate(args[1:])
	case "update":
		return r.updateRate(args[1:])
	case "delete":
		return r.deleteRate(args[1:])
	case "help", "-h", "--help":
		fmt.Fprintln(r.out, "Usage: chankat rates list|get|create|update|delete")
		return nil
	default:
		return fmt.Errorf("unknown rates command %q", args[0])
	}
}

func (r runner) loadRates() ([]storage.RateSummary, error) {
	rates, err := r.stor.GetRates(r.ctx)
	if err != nil {
		return nil, err
	}
	projects, err := r.stor.GetProjects(r.ctx)
	if err != nil {
		return nil, err
	}
	return storage.SummarizeRates(rates, projects), nil
}

func (r runner) listRates(args []string) error {
	if err := requireNoArgs(args, "chankat rates list"); err != nil {
		return err
	}
	items, err := r.loadRates()
	if err != nil {
		return fmt.Errorf("list rates: %w", err)
	}
	output := rateOutputs(items)
	if r.json {
		return r.writeJSON(output)
	}
	rows := make([]string, len(output))
	for i, item := range output {
		rows[i] = fmt.Sprintf("%d\t%s\t%d\t%s\t%d", item.ID, item.Name,
			item.AmountMinor, item.Currency, item.ProjectCount)
	}
	return r.table("ID\tNAME\tAMOUNT_MINOR\tCURRENCY\tPROJECTS", rows)
}

func (r runner) getRate(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: chankat rates get ID")
	}
	id, err := parseID(args[0], "rate")
	if err != nil {
		return err
	}
	items, err := r.loadRates()
	if err != nil {
		return fmt.Errorf("get rate: %w", err)
	}
	for _, item := range items {
		if item.ID == id {
			output := rateOutputs([]storage.RateSummary{item})
			if r.json {
				return r.writeJSON(output[0])
			}
			return r.table("ID\tNAME\tAMOUNT_MINOR\tCURRENCY\tPROJECTS",
				[]string{fmt.Sprintf("%d\t%s\t%d\t%s\t%d", item.ID, item.Name,
					item.AmountMinor, item.Currency, item.ProjectCount)})
		}
	}
	return fmt.Errorf("rate %d not found", id)
}

func (r runner) createRate(args []string) error {
	flags := r.flags("rates create",
		"Usage: chankat rates create --name NAME --amount-minor N --currency CODE")
	name := flags.String("name", "", "rate name")
	amount := flags.Int("amount-minor", 0, "hourly amount in minor units")
	currency := flags.String("currency", "", "three-letter currency code")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if err := required(flags, "name", "amount-minor", "currency"); err != nil {
		return err
	}
	id, err := r.stor.CreateRateID(r.ctx, storage.Rate{
		Name: *name, AmountMinor: *amount, Currency: *currency,
	})
	if err != nil {
		return err
	}
	return r.status("created", "rate", id)
}

func (r runner) updateRate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: chankat rates update ID [options]")
	}
	id, err := parseID(args[0], "rate")
	if err != nil {
		return err
	}
	rate, err := r.stor.GetRate(r.ctx, id)
	if err != nil {
		return err
	}
	flags := r.flags("rates update",
		"Usage: chankat rates update ID [--name NAME] [--amount-minor N] [--currency CODE]")
	name := flags.String("name", rate.Name, "rate name")
	amount := flags.Int("amount-minor", rate.AmountMinor, "hourly amount in minor units")
	currency := flags.String("currency", rate.Currency, "three-letter currency code")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if flags.NFlag() == 0 {
		return fmt.Errorf("expected at least one update option")
	}
	rate.Name, rate.AmountMinor, rate.Currency = *name, *amount, *currency
	if err := r.stor.UpdateRate(r.ctx, rate); err != nil {
		return err
	}
	return r.status("updated", "rate", id)
}

func (r runner) deleteRate(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: chankat rates delete ID")
	}
	id, err := parseID(args[0], "rate")
	if err != nil {
		return err
	}
	if err := r.stor.DeleteRate(r.ctx, id); err != nil {
		return err
	}
	return r.status("deleted", "rate", id)
}
